# Architecture Boundary Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the highest-risk module-boundary problems in the fallback/free-provider stack without changing public API behavior.

**Architecture:** Fix the shared mutable config boundary first, then split overloaded modules into pure planning/projection helpers plus thin HTTP/runtime adapters. Keep FreeLLMAPI provider support behavior stable while making provider identity explicit where runtime quirks are applied.

**Tech Stack:** Go backend with Gin, GORM, existing `fallback` package tests; React frontend with Semantic UI and Jest; existing PowerShell smoke script.

## Global Constraints

- Work on branch `cleanup/structure-boundaries`; do not touch `main`.
- Do not remove or rename existing API endpoints.
- Do not expose raw `free_providers.*.keys` values in API responses, backups, logs, or tests.
- Preserve existing provider IDs, auto deployment ID format `free:{provider}-{suffix}`, and auto channel name prefix `[CCT Auto] `.
- Preserve existing fallback config JSON compatibility, including legacy `routing_mode`, `fallback_order`, and `fixed_deployment` handling.
- Add or update tests before production code when behavior changes.
- For pure refactors, run existing characterization tests before and after the split.
- Keep generated frontend build assets in sync when frontend source changes.

---

## File Structure

- `fallback/config.go`: Owns config schema, normalization, validation, cloning, and safe live config access.
- `fallback/free_provider_sync.go`: Owns free-provider runtime reconciliation. After cleanup, pure desired-resource planning should be separated from DB apply logic.
- `fallback/free_provider_registry.go`: Owns provider metadata and auto ID helpers. Export only stable helper functions needed by other packages.
- `router/fallback_gateway.go`: Should become thin HTTP handlers only.
- `router/fallback_gateway_types.go`: New file for gateway DTOs.
- `router/fallback_gateway_projection.go`: New file for DTO projection and input merge helpers.
- `router/fallback_gateway_service.go`: New file for save/backup/reload orchestration shared by gateway and manual config handlers.
- `common/ctxkey/fallback.go`: Owns fallback context keys.
- `relay/adaptor/openai/adaptor.go`: Applies OpenAI-compatible request quirks using explicit context metadata first, with channel-name fallback for compatibility.
- `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`: Should become a light container.
- `web/default/src/components/fallback-gateway/FreeProviderRow.js`: New row component for provider UI.
- `web/default/src/components/fallback-gateway/freeProviderDisplay.js`: New display metadata and formatting helpers.
- `.superpowers/sdd/progress.md`: Durable task progress ledger.

## Task 1: Safe Config Snapshot And Runtime Free Pool Sync

**Files:**
- Modify: `fallback/config.go`
- Modify: `router/fallback.go`
- Modify: `fallback/free_provider_scheduler.go`
- Modify: `fallback/health.go`
- Modify: `fallback/alert.go`
- Test: `fallback/config_clone_test.go`
- Test: `fallback/free_pool_test.go`

**Interfaces:**
- Produces: `func GetConfig() *Config` returns a deep clone instead of the live pointer.
- Produces: `func SyncFreePoolRuntime() error` clones the live config, runs `SyncFreePool` on the clone, validates it, and swaps it under the config lock.
- Consumes: existing `SyncFreePool(cfg *Config) error`, `validateConfigData(cfg *Config) error`, and `cloneConfig(src *Config) *Config`.

- [ ] **Step 1: Add failing clone-boundary test**

Add this test to `fallback/config_clone_test.go`:

```go
func TestGetConfigReturnsDeepClone(t *testing.T) {
	rpm := 10
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {Enabled: true, Strategy: StrategyFreeFirst, Pools: []string{"free"}},
		},
		Deployments: map[string]DeploymentConfig{
			"free:groq-001122ff": {Enabled: true, ChannelID: 1, RealModel: "original", Pool: "free", Weight: 100, SoftLimitRatio: 0.95, HardLimitRatio: 1.0},
		},
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {
				Enabled:        true,
				Keys:           []string{"original-key"},
				Models:         []string{"original-model"},
				LimitsOverride: &FreeProviderLimits{RPMLimit: &rpm},
			},
		},
	})

	got := GetConfig()
	got.VirtualModels["cct/free"] = VirtualModelConfig{Enabled: false}
	got.Deployments["free:groq-001122ff"] = DeploymentConfig{RealModel: "mutated"}
	fp := got.FreeProviders["groq"]
	fp.Keys[0] = "mutated-key"
	fp.Models[0] = "mutated-model"
	*fp.LimitsOverride.RPMLimit = 99
	got.FreeProviders["groq"] = fp

	live := GetConfig()
	if !live.VirtualModels["cct/free"].Enabled {
		t.Fatalf("GetConfig leaked virtual model mutation")
	}
	if live.Deployments["free:groq-001122ff"].RealModel != "original" {
		t.Fatalf("GetConfig leaked deployment mutation: %s", live.Deployments["free:groq-001122ff"].RealModel)
	}
	if live.FreeProviders["groq"].Keys[0] != "original-key" {
		t.Fatalf("GetConfig leaked free provider key mutation: %v", live.FreeProviders["groq"].Keys)
	}
	if live.FreeProviders["groq"].Models[0] != "original-model" {
		t.Fatalf("GetConfig leaked free provider model mutation: %v", live.FreeProviders["groq"].Models)
	}
	if *live.FreeProviders["groq"].LimitsOverride.RPMLimit != 10 {
		t.Fatalf("GetConfig leaked limit mutation: %d", *live.FreeProviders["groq"].LimitsOverride.RPMLimit)
	}
}
```

- [ ] **Step 2: Run clone-boundary test and confirm RED**

Run: `go test ./fallback -run TestGetConfigReturnsDeepClone -count=1`

Expected before implementation: FAIL because mutating `GetConfig()` result mutates live config.

- [ ] **Step 3: Implement safe `GetConfig` and runtime sync**

Change `GetConfig` in `fallback/config.go` to return `cloneConfig(config)`.

Add this function in `fallback/config.go` near `ReloadConfig`:

```go
func SyncFreePoolRuntime() error {
	configLock.RLock()
	newCfg := cloneConfig(config)
	configLock.RUnlock()
	if newCfg == nil || !newCfg.Enabled {
		return fmt.Errorf("fallback config is not enabled")
	}

	if err := SyncFreePool(newCfg); err != nil {
		return err
	}
	if err := validateConfigData(newCfg); err != nil {
		return err
	}

	configLock.Lock()
	config = newCfg
	configLock.Unlock()
	return nil
}
```

- [ ] **Step 4: Update runtime sync endpoint**

In `router/fallback.go`, replace the direct `GetConfig()` plus `SyncFreePool(cfg)` mutation path with `fallback.SyncFreePoolRuntime()`. Preserve the JSON response shape:

```go
if err := fallback.SyncFreePoolRuntime(); err != nil {
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
	return
}
c.JSON(http.StatusOK, gin.H{"success": true, "message": "free pool synced successfully"})
```

- [ ] **Step 5: Run focused tests and confirm GREEN**

Run:

```powershell
go test ./fallback -run "TestGetConfigReturnsDeepClone|TestCloneConfigMutationIsolation|TestSyncFreePoolPreservesDeploymentRealModelOverride|TestMultiKey" -count=1
go test ./router -run "TestGatewayUpdateConfig|TestBackupFallbackEditorConfigSanitizesFreeProviderKeys" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add fallback/config.go fallback/config_clone_test.go router/fallback.go fallback/free_provider_scheduler.go fallback/health.go fallback/alert.go fallback/free_pool_test.go
git commit -m "fix: isolate fallback config snapshots"
```

## Task 2: Split Free Provider Sync Planning From DB Apply

**Files:**
- Modify: `fallback/free_provider_sync.go`
- Test: `fallback/free_pool_test.go`

**Interfaces:**
- Produces: `type desiredFreeProviderResource struct`.
- Produces: `func buildDesiredFreeProviderResources(cfg *Config) ([]desiredFreeProviderResource, map[string]DeploymentConfig)`.
- Consumes: existing `BuiltinFreeProviders`, `SafeKeyHash`, `channelName`, `deploymentID`, and `ResolveFreeProviderLimits`.

- [ ] **Step 1: Add failing planner tests**

Add tests to `fallback/free_pool_test.go`:

```go
func TestBuildDesiredFreeProviderResourcesKeylessProvider(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		FreeProviders: map[string]FreeProviderConfig{
			"pollinations": {Enabled: true},
		},
	}

	resources, deployments := buildDesiredFreeProviderResources(cfg)

	if len(resources) != 1 {
		t.Fatalf("resources length = %d, want 1", len(resources))
	}
	if resources[0].provider != "pollinations" {
		t.Fatalf("provider = %q, want pollinations", resources[0].provider)
	}
	if resources[0].ch.Key != "" {
		t.Fatalf("keyless channel key = %q, want empty", resources[0].ch.Key)
	}
	depID := deploymentID("pollinations", SafeKeyHash(""))
	if _, ok := deployments[depID]; !ok {
		t.Fatalf("missing deployment %s", depID)
	}
}

func TestBuildDesiredFreeProviderResourcesSkipsMissingKey(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {Enabled: true},
		},
	}

	resources, deployments := buildDesiredFreeProviderResources(cfg)

	if len(resources) != 0 {
		t.Fatalf("resources length = %d, want 0", len(resources))
	}
	if len(deployments) != 0 {
		t.Fatalf("deployments length = %d, want 0", len(deployments))
	}
}
```

- [ ] **Step 2: Run planner tests and confirm RED**

Run: `go test ./fallback -run "TestBuildDesiredFreeProviderResources" -count=1`

Expected before implementation: FAIL because `buildDesiredFreeProviderResources` is undefined.

- [ ] **Step 3: Extract pure planner**

Move the desired-channel/deployment construction from `SyncFreePool` into `buildDesiredFreeProviderResources`. The helper must not touch `model.DB`, must not read or mutate global config, and must keep `CreatedTime` out of planner output so tests stay deterministic. Use `ResolveFreeProviderLimits(meta, fp)` instead of duplicating default/override limit resolution.

- [ ] **Step 4: Rewire `SyncFreePool`**

Make `SyncFreePool` call `buildDesiredFreeProviderResources(cfg)` before the DB reconciliation block. Keep existing channel update/create/disable behavior unchanged.

Keep these behaviors in the apply layer, not the planner:

```go
helper.GetTimestamp()
preserveDeploymentRealModelOverride(...)
deploymentUsesAutoChannel(...)
model.DB.Where(...)
model.DB.Model(...)
```

- [ ] **Step 5: Rewire expected-resource computation**

Change `computeExpectedAutoResources` to call `buildDesiredFreeProviderResources(cfg)` and derive expected channel/deployment sets from its output. This avoids maintaining two copies of provider/key iteration logic.

- [ ] **Step 6: Run focused tests and confirm GREEN**

Run:

```powershell
go test ./fallback -run "TestBuildDesiredFreeProviderResources|TestComputeExpectedAutoResources|TestMultiKey|TestSyncFreePool|TestSyncAllProviderModels|TestGetConfigReturnsDeepClone|TestCloneConfigMutationIsolation" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add fallback/free_provider_sync.go fallback/free_pool_test.go
git commit -m "refactor: split free provider sync planning"
```

## Task 3: Split Gateway Config Handler Boundaries

**Files:**
- Create: `router/fallback_gateway_types.go`
- Create: `router/fallback_gateway_projection.go`
- Create: `router/fallback_gateway_service.go`
- Modify: `router/fallback_gateway.go`
- Test: `router/fallback_gateway_test.go`

**Interfaces:**
- Produces: DTO structs moved unchanged from `fallback_gateway.go`.
- Produces: projection helpers moved unchanged from `fallback_gateway.go`.
- Produces: `func saveGatewayConfigPayload(merged fallback.Config) (backupPath string, err error)`.
- Consumes: existing `backupFallbackEditorConfig`, `fallbackEditorConfigPath`, and `fallback.ReloadConfig`.

- [ ] **Step 1: Run characterization tests before refactor**

Run:

```powershell
go test ./router -run "TestGatewayGetConfig|TestGatewayUpdateConfig|TestUpdateManualConfigDeepCopy|TestBuildGatewayV2Config|TestBackupFallbackEditorConfigSanitizesFreeProviderKeys" -count=1
```

Expected: PASS before moving code.

- [ ] **Step 2: Move DTO types**

Move `gatewayV2Config`, `gatewayV2VirtualModel`, `gatewayV2Deployment`, `gatewayV2FreeProvider`, `gatewayV2LimitsOverride`, `gatewayV2ConfigInput`, and `gatewayV2FreeProviderInput` from `router/fallback_gateway.go` into `router/fallback_gateway_types.go` without changing field names or JSON tags.

- [ ] **Step 3: Move projection and merge helpers**

Move these helpers into `router/fallback_gateway_projection.go` without behavior changes:

```go
containsLegacyFields
toFreeProviderLimits
mergeGatewayFreeProviderInput
cloneInt
buildGatewayV2FreeProviders
cloneGatewayFreeProviderQuirks
buildGatewayV2Config
isManualDeployment
buildManualConfig
```

- [ ] **Step 4: Extract save service**

Create `router/fallback_gateway_service.go` with:

```go
func saveGatewayConfigPayload(merged fallback.Config) (string, error) {
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	backupPath, err := backupFallbackEditorConfig(fallbackEditorConfigPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(fallbackEditorConfigPath, data, 0644); err != nil {
		return "", err
	}
	if err := fallback.ReloadConfig(fallbackEditorConfigPath); err != nil {
		return "", err
	}
	return backupPath, nil
}
```

Update both `updateManualConfig` and `updateGatewayConfig` to call it.

- [ ] **Step 5: Run characterization tests after refactor**

Run:

```powershell
go test ./router -run "TestGatewayGetConfig|TestGatewayUpdateConfig|TestUpdateManualConfigDeepCopy|TestBuildGatewayV2Config|TestBackupFallbackEditorConfigSanitizesFreeProviderKeys" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add router/fallback_gateway.go router/fallback_gateway_types.go router/fallback_gateway_projection.go router/fallback_gateway_service.go router/fallback_gateway_test.go
git commit -m "refactor: split gateway config boundaries"
```

## Task 4: Make Free Provider Runtime Identity Explicit

**Files:**
- Modify: `common/ctxkey/fallback.go`
- Modify: `fallback/free_provider_registry.go`
- Modify: `controller/relay.go`
- Modify: `middleware/distributor.go`
- Modify: `relay/adaptor/openai/adaptor.go`
- Test: `relay/adaptor/openai/adaptor_test.go`
- Test: `fallback/free_pool_test.go`

**Interfaces:**
- Produces: `ctxkey.FallbackFreeProviderName`.
- Produces: `func FreeProviderNameFromDeploymentID(id string) (string, bool)`.
- Consumes: existing `freeproviderquirks.ForProvider` and channel-name fallback parsing.

- [ ] **Step 1: Add failing helper and adaptor tests**

Add to `fallback/free_pool_test.go`:

```go
func TestFreeProviderNameFromDeploymentID(t *testing.T) {
	provider, ok := FreeProviderNameFromDeploymentID("free:groq-001122ff")
	if !ok || provider != "groq" {
		t.Fatalf("FreeProviderNameFromDeploymentID = %q, %v; want groq, true", provider, ok)
	}
	if _, ok := FreeProviderNameFromDeploymentID("free:unknown-001122ff"); ok {
		t.Fatalf("unknown provider should not be accepted")
	}
	if _, ok := FreeProviderNameFromDeploymentID("manual-free"); ok {
		t.Fatalf("manual deployment should not be accepted")
	}
}
```

Add to `relay/adaptor/openai/adaptor_test.go`:

```go
func TestApplyFreeProviderRequestQuirksUsesExplicitProviderContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ctxkey.ChannelName, "manual-channel-name")
	c.Set(ctxkey.FallbackFreeProviderName, "aihorde")
	maxTokens := 4096
	req := &model.GeneralOpenAIRequest{
		Stream:              true,
		MaxTokens:           4096,
		MaxCompletionTokens: &maxTokens,
		Stop:                []string{"stop"},
	}

	ApplyFreeProviderRequestQuirks(c, req)

	if req.Stream {
		t.Fatalf("stream should be disabled for aihorde")
	}
	if req.MaxTokens != 1024 {
		t.Fatalf("MaxTokens = %d, want 1024", req.MaxTokens)
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 1024 {
		t.Fatalf("MaxCompletionTokens = %v, want 1024", req.MaxCompletionTokens)
	}
	if req.Stop != nil {
		t.Fatalf("Stop = %#v, want nil", req.Stop)
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run:

```powershell
go test ./fallback -run TestFreeProviderNameFromDeploymentID -count=1
go test ./relay/adaptor/openai -run TestApplyFreeProviderRequestQuirksUsesExplicitProviderContext -count=1
```

Expected before implementation: FAIL because the helper/context key does not exist.

- [ ] **Step 3: Export provider-name helper**

In `fallback/free_provider_registry.go`, implement:

```go
func FreeProviderNameFromDeploymentID(id string) (string, bool) {
	for providerName := range knownFreeProviders {
		prefix := "free:" + providerName + "-"
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(id, prefix)
		if !isAutoDeploymentSuffix(suffix) {
			return "", false
		}
		return providerName, true
	}
	return "", false
}
```

Change `providerNameForAutoDeploymentID` to call this helper.

- [ ] **Step 4: Add explicit context key**

In `common/ctxkey/fallback.go`, add:

```go
FallbackFreeProviderName = "fallback_free_provider_name"
```

- [ ] **Step 5: Set context during fallback selection**

In both `controller/relay.go` and `middleware/distributor.go`, after setting fallback deployment ID and real model, call `fallback.FreeProviderNameFromDeploymentID(dep.ID)`. If it returns true, set `ctxkey.FallbackFreeProviderName`.

- [ ] **Step 6: Use explicit context in OpenAI adaptor**

In `relay/adaptor/openai/adaptor.go`, update `freeProviderQuirksFromContext` to check `ctxkey.FallbackFreeProviderName` first:

```go
if provider := strings.TrimSpace(c.GetString(ctxkey.FallbackFreeProviderName)); provider != "" {
	if quirks, ok := freeproviderquirks.ForProvider(provider); ok {
		return quirks
	}
}
```

Keep the existing channel-name parsing fallback for compatibility.

- [ ] **Step 7: Run focused tests and confirm GREEN**

Run:

```powershell
go test ./fallback -run "TestFreeProviderNameFromDeploymentID|TestIsAutoDeploymentID" -count=1
go test ./relay/adaptor/openai -run "TestApplyFreeProviderRequestQuirks" -count=1
go test ./controller ./middleware -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add common/ctxkey/fallback.go fallback/free_provider_registry.go fallback/free_pool_test.go controller/relay.go middleware/distributor.go relay/adaptor/openai/adaptor.go relay/adaptor/openai/adaptor_test.go
git commit -m "refactor: pass free provider identity explicitly"
```

## Task 5: Split Free Provider Frontend Editor

**Files:**
- Create: `web/default/src/components/fallback-gateway/freeProviderDisplay.js`
- Create: `web/default/src/components/fallback-gateway/FreeProviderRow.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
- Test: `web/default/src/components/fallback-gateway/freeProviderDisplay.test.js`
- Modify generated build assets under `web/build/default`

**Interfaces:**
- Produces: `PROVIDER_DISPLAY`, `LIMIT_FIELDS`, `formatNumber`, `formatLimit`, `validateLimits`.
- Produces: `FreeProviderRow` component.
- Consumes: existing `buildFreeProviderRows`.

- [ ] **Step 1: Add failing display-helper tests**

Create `web/default/src/components/fallback-gateway/freeProviderDisplay.test.js`:

```js
import { formatLimit, validateLimits } from './freeProviderDisplay';

describe('freeProviderDisplay', () => {
  test('formatLimit treats zero as unlimited', () => {
    expect(formatLimit(0)).toBe('unlimited');
  });

  test('formatLimit formats missing values as dash', () => {
    expect(formatLimit(undefined)).toBe('-');
    expect(formatLimit('not-a-number')).toBe('-');
  });

  test('validateLimits rejects negative values', () => {
    expect(validateLimits({ rpm_limit: -1 })).toBe(false);
    expect(validateLimits({ rpm_limit: 0, rpd_limit: '10' })).toBe(true);
  });
});
```

- [ ] **Step 2: Run helper tests and confirm RED**

Run from `web/default`:

```powershell
npm test -- --watchAll=false freeProviderDisplay.test.js
```

Expected before implementation: FAIL because `freeProviderDisplay` does not exist.

- [ ] **Step 3: Extract display helpers**

Create `freeProviderDisplay.js` with `PROVIDER_DISPLAY`, `LIMIT_FIELDS`, `formatNumber`, `formatLimit`, and `validateLimits` moved from `FreeProvidersEditor.js`.

- [ ] **Step 4: Extract row component**

Create `FreeProviderRow.js` containing the table row rendering currently inside `providerRows.map`. Pass these props:

```js
provider
providerConfig
onUpdateProvider
onUpdateLimit
onUpdateKeys
```

- [ ] **Step 5: Slim down editor**

Keep `FreeProvidersEditor.js` responsible only for building rows, holding update callbacks, rendering table headers/body, and the key safety message.

- [ ] **Step 6: Run frontend tests and build**

Run from `web/default`:

```powershell
npm test -- --watchAll=false
npm run build
```

Expected: tests PASS. Build PASS with only pre-existing warnings.

- [ ] **Step 7: Commit**

```powershell
git add web/default/src/components/fallback-gateway/FreeProvidersEditor.js web/default/src/components/fallback-gateway/FreeProviderRow.js web/default/src/components/fallback-gateway/freeProviderDisplay.js web/default/src/components/fallback-gateway/freeProviderDisplay.test.js web/build/default
git commit -m "refactor: split free provider editor UI"
```

## Task 6: Final Verification And Review

**Files:**
- Modify: `.superpowers/sdd/progress.md`

**Interfaces:**
- Consumes: all previous task commits.
- Produces: final verification evidence and merge-ready branch.

- [ ] **Step 1: Run backend verification**

Run:

```powershell
go test ./... -count=1
go build -o one-api.exe
```

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run from `web/default`:

```powershell
npm test -- --watchAll=false
npm run build
```

Expected: tests PASS. Build PASS with only pre-existing warnings.

- [ ] **Step 3: Run smoke parser**

Run:

```powershell
powershell -NoProfile -Command "$null=[System.Management.Automation.PSParser]::Tokenize((Get-Content -Raw scripts/fallback-smoke.ps1), [ref]$null); 'parser ok'"
```

Expected: prints `parser ok`.

- [ ] **Step 4: Run final branch review**

Create a review package from merge-base to `HEAD` and dispatch a final code-review subagent. Fix Critical/Important findings before merge.

- [ ] **Step 5: Commit any final cleanup**

Commit only relevant source, test, docs, and build asset changes. Do not commit `web/default/node_modules`, `web/default/package-lock.json`, or `one-api.exe`.
