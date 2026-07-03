# FreeLLMAPI Native Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make cctapi's native free pool ready to absorb FreeLLMAPI core behavior without depending on a separate FreeLLMAPI proxy.

**Architecture:** Keep cctapi's existing fallback gateway as the routing surface. Upgrade the free pool in layers: first repair correctness bugs, then add provider metadata/catalog structure, then add persistent quota/key/catalog services in later tasks.

**Tech Stack:** Go 1.22, GORM/SQLite-compatible tests, existing One API channel/adaptor system, React frontend under `web/default`.

## Global Constraints

- Work on branch `cleanup/structure-boundaries`.
- Preserve existing `cct/high`, `cct/low`, and `cct/free` virtual model behavior.
- Do not remove existing free providers while adding FreeLLMAPI-compatible providers.
- Use TDD for behavior changes: write a failing test, run it red, implement, run green.
- Keep existing fallback gateway and relay controller surfaces; do not introduce a second proxy service.
- Do not store new real provider keys in examples or docs.

---

## Big Task Framework

### Phase 0: Stability Fixes

- Repair quota pre-check window reset.
- Repair expected auto-resource calculation for keyless providers.
- Repair invalid example JSON.
- Add tests so these failures cannot regress.

### Phase 1: Native FreeLLMAPI Provider Foundation

- Extend provider metadata so cctapi can describe FreeLLMAPI-style providers with auth mode, keyless support, model fetch mode, and catalog identity.
- Add currently missing FreeLLMAPI providers to the registry as OpenAI-compatible/Gemini-compatible entries where cctapi already has a compatible channel type.
- Keep unknown or risky providers disabled-by-config unless explicitly enabled by user config.

### Phase 2: Model Catalog and Dynamic Routing

- Add a cctapi-native catalog model containing provider, model, capabilities, limits, context length, and health hints.
- Make model sync update routing-visible deployment real models, not just DB channel model strings.
- Preserve local overrides so user config wins over remote/default catalog data.

### Phase 3: Key Security and Quota Ledger

- Move free provider keys away from long-term plaintext config storage.
- Add encrypted key references and a migration-compatible fallback for old `fallback.json` keys.
- Add persistent per-provider/per-model/per-key RPM/RPD/TPM/TPD usage ledger.

### Phase 4: UI and Operations

- Expose provider metadata, key count, provider health, and quota windows in the gateway API.
- Update the free pool UI so users manage providers/models/keys without seeing raw secrets.
- Add docs for enabling FreeLLMAPI-native providers and merging the branch back.

---

### Task 1: Runtime Quota and Keyless Resource Correctness

**Files:**
- Modify: `fallback/quota.go`
- Modify: `fallback/integration_test.go`
- Modify: `fallback/free_provider_sync.go`
- Modify: `fallback/free_pool_test.go`

**Interfaces:**
- Consumes: `PassQuotaCheck(dep DeploymentConfig, state *DeploymentRuntimeState, estimatedTokens int) bool`
- Consumes: `computeExpectedAutoResources(cfg *Config) (map[string]bool, map[string]bool)`
- Produces: quota pre-check that resets expired minute/day windows before evaluating limits.
- Produces: keyless providers with zero configured keys still produce one expected auto channel/deployment using `SafeKeyHash("")`.

- [ ] **Step 1: Write failing quota reset test**

Add to `fallback/integration_test.go`:

```go
func TestIntegrationQuotaPreCheckResetsExpiredWindows(t *testing.T) {
	resetRuntimeForTest()
	dep := DeploymentConfig{ID: "groq", RPMLimit: 1, RPDLimit: 1, TPMLimit: 100, TPDLimit: 100}
	state := GetRuntimeState("groq")
	state.MinuteRequests = 1
	state.DayRequests = 1
	state.MinuteTokens = 100
	state.DayTokens = 100
	state.LastResetMinute = time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	state.LastResetDay = truncateToDay(time.Now().AddDate(0, 0, -1))

	if !PassQuotaCheck(dep, state, 50) {
		t.Fatalf("expected expired quota windows to reset before pre-check")
	}

	snap := SnapshotRuntimeState("groq")
	if snap.MinuteRequests != 0 || snap.DayRequests != 0 || snap.MinuteTokens != 0 || snap.DayTokens != 0 {
		t.Fatalf("expected expired counters reset, got req %d/%d tokens %d/%d",
			snap.MinuteRequests, snap.DayRequests, snap.MinuteTokens, snap.DayTokens)
	}
}
```

- [ ] **Step 2: Write failing keyless resource test**

Add to `fallback/free_pool_test.go`:

```go
func TestComputeExpectedAutoResources_KeylessProvider(t *testing.T) {
	cfg := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"pollinations": {Enabled: true},
		},
	}

	channels, deployments := computeExpectedAutoResources(cfg)
	keyHash := SafeKeyHash("")
	wantChannel := channelName("pollinations", keyHash)
	wantDeployment := deploymentID("pollinations", keyHash)

	if !channels[wantChannel] {
		t.Fatalf("expected keyless channel %q in desired resources", wantChannel)
	}
	if !deployments[wantDeployment] {
		t.Fatalf("expected keyless deployment %q in desired resources", wantDeployment)
	}
}
```

- [ ] **Step 3: Run tests to verify RED**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback -run "TestIntegrationQuotaPreCheckResetsExpiredWindows|TestComputeExpectedAutoResources_KeylessProvider" -count=1
```

Expected: FAIL. The quota test fails because stale counters are still evaluated. The keyless test fails because `computeExpectedAutoResources` skips providers with `len(Keys)==0`.

- [ ] **Step 4: Implement minimal fix**

In `fallback/quota.go`, make `PassQuotaCheck` acquire the runtime-state lock and call `maybeResetWindows` before checking limits.

In `fallback/free_provider_sync.go`, change `computeExpectedAutoResources` so enabled known providers with zero keys add one expected resource using `SafeKeyHash("")`.

- [ ] **Step 5: Run tests to verify GREEN**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback -run "TestIntegrationQuotaPreCheckResetsExpiredWindows|TestComputeExpectedAutoResources_KeylessProvider" -count=1
go test ./fallback ./router ./controller
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add fallback/quota.go fallback/integration_test.go fallback/free_provider_sync.go fallback/free_pool_test.go
git commit -m "fix: repair free pool quota and keyless resource checks"
```

### Task 2: FreeLLMAPI Provider Metadata Foundation

**Files:**
- Modify: `fallback/free_provider_registry.go`
- Modify: `fallback/free_provider_fetch.go`
- Modify: `fallback/free_pool_test.go`

**Interfaces:**
- Produces: richer `FreeProviderMeta` fields for provider identity and fetch behavior.
- Produces: missing FreeLLMAPI provider names accepted by `ValidateFreeProviderName`.
- Produces: `knownFreeProviders` derived from `BuiltinFreeProviders` to avoid drift.

- [ ] **Step 1: Write failing registry coverage test**

Add to `fallback/free_pool_test.go`:

```go
func TestBuiltinFreeProviderRegistry_FreeLLMAPICoreProvidersPresent(t *testing.T) {
	want := []string{
		"google", "nvidia", "cohere", "huggingface", "ollama",
		"llm7", "opencode", "aihorde", "routeway", "bazaarlink",
		"ainative", "agnes", "reka",
	}
	for _, name := range want {
		if err := ValidateFreeProviderName(name); err != nil {
			t.Errorf("expected FreeLLMAPI provider %q to be accepted: %v", name, err)
		}
	}
}
```

- [ ] **Step 2: Write failing drift test**

Add to `fallback/free_pool_test.go`:

```go
func TestKnownFreeProvidersMatchesBuiltinRegistry(t *testing.T) {
	for name := range BuiltinFreeProviders {
		if _, ok := knownFreeProviders[name]; !ok {
			t.Errorf("knownFreeProviders missing builtin provider %q", name)
		}
	}
	for name := range knownFreeProviders {
		if _, ok := BuiltinFreeProviders[name]; !ok {
			t.Errorf("knownFreeProviders has provider %q missing from BuiltinFreeProviders", name)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify RED**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback -run "TestBuiltinFreeProviderRegistry_FreeLLMAPICoreProvidersPresent|TestKnownFreeProvidersMatchesBuiltinRegistry" -count=1
```

Expected: FAIL because several FreeLLMAPI provider names are missing.

- [ ] **Step 4: Implement provider metadata foundation**

Add conservative metadata fields to `FreeProviderMeta`:

```go
ProviderID     string
RequiresKey    bool
Keyless        bool
ModelFetchMode string
```

Add constants:

```go
const (
	ModelFetchStatic        = "static"
	ModelFetchOpenAIModels  = "openai_models"
	ModelFetchOpenRouterFree = "openrouter_free"
	ModelFetchKiloFree      = "kilo_free"
)
```

Populate existing providers and add missing FreeLLMAPI provider entries using cctapi channel types that already exist.

Replace the hand-written `knownFreeProviders` map with an `initKnownFreeProviders()` helper that copies keys from `BuiltinFreeProviders`.

- [ ] **Step 5: Update fetch behavior**

In `fallback/free_provider_fetch.go`, route providers by `ModelFetchMode` where possible. Keep existing explicit behavior for OpenRouter, Groq, Kilo, Pollinations, OVH, and adapter-backed providers. For newly added OpenAI-compatible providers, use `fetchOpenAICompatModels(meta.DefaultBaseURL, key)`.

- [ ] **Step 6: Run tests to verify GREEN**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add fallback/free_provider_registry.go fallback/free_provider_fetch.go fallback/free_pool_test.go
git commit -m "feat: add freellmapi provider metadata foundation"
```

### Task 3: Dynamic Model Sync Updates Routing-Visible Real Model

**Files:**
- Modify: `fallback/config.go`
- Modify: `fallback/config_routing_test.go`
- Modify: `fallback/free_provider_sync.go`

**Interfaces:**
- Produces: `UpdateDeploymentRealModel(deploymentID string, realModel string) bool`
- Consumes: `syncAllProviderModels(cfg *Config)`

- [ ] **Step 1: Write failing config update test**

Add to `fallback/config_routing_test.go`:

```go
func TestUpdateDeploymentRealModelUpdatesLiveConfig(t *testing.T) {
	t.Cleanup(func() { resetConfigForTest(nil) })
	resetConfigForTest(&Config{
		Enabled: true,
		Deployments: map[string]DeploymentConfig{
			"free:provider-key": {ID: "free:provider-key", Enabled: true, ChannelID: 1, RealModel: "old-model", Pool: "free"},
		},
	})

	if !UpdateDeploymentRealModel("free:provider-key", "new-model") {
		t.Fatalf("expected real model update to report changed=true")
	}
	dep, ok := CloneDeployment("free:provider-key")
	if !ok {
		t.Fatalf("expected deployment to exist")
	}
	if dep.RealModel != "new-model" {
		t.Fatalf("expected real model new-model, got %q", dep.RealModel)
	}
	if UpdateDeploymentRealModel("free:provider-key", "new-model") {
		t.Fatalf("expected unchanged real model to report changed=false")
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback -run TestUpdateDeploymentRealModelUpdatesLiveConfig -count=1
```

Expected: FAIL because `UpdateDeploymentRealModel` does not exist.

- [ ] **Step 3: Implement config update helper**

Add `UpdateDeploymentRealModel` to `fallback/config.go`. It must lock `configLock`, ignore empty `realModel`, return false for nil/missing/unchanged deployments, and update the map entry for changed deployments.

- [ ] **Step 4: Wire model sync**

In `syncAllProviderModels`, after a successful non-empty model list, call `UpdateDeploymentRealModel(depID, models[0])` for keyless and keyed provider deployments. Log when the routing-visible real model changes.

- [ ] **Step 5: Run tests to verify GREEN**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add fallback/config.go fallback/config_routing_test.go fallback/free_provider_sync.go
git commit -m "fix: sync free provider real models into routing config"
```

### Task 4: Valid Example Config and Gateway Surface Cleanup

**Files:**
- Modify: `data/fallback.json.example`
- Modify: `router/fallback_gateway.go`
- Modify: `router/fallback_gateway_test.go`

**Interfaces:**
- Produces: valid JSON example parseable by `encoding/json`.
- Produces: gateway response that can expose provider metadata without raw keys.

- [ ] **Step 1: Write failing JSON example test**

Add to `router/fallback_gateway_test.go`:

```go
func TestFallbackExampleConfigIsValidJSON(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "data", "fallback.json.example"))
	if err != nil {
		t.Fatalf("failed to read example config: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("fallback.json.example must be valid JSON: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./router -run TestFallbackExampleConfigIsValidJSON -count=1
```

Expected: FAIL because the example file currently contains invalid strings.

- [ ] **Step 3: Replace example with valid JSON**

Keep the same top-level structure, remove broken prose keys, and use placeholder values only:

```json
{
  "enabled": true,
  "virtual_models": {},
  "deployments": {},
  "free_providers": {},
  "alert": {},
  "smart_sort": {},
  "blocked_error_codes": ["internal_server_error"]
}
```

Use real cct defaults in the final file, but keep all keys fake and disabled.

- [ ] **Step 4: Run tests to verify GREEN**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./router ./fallback ./controller -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add data/fallback.json.example router/fallback_gateway_test.go router/fallback_gateway.go
git commit -m "chore: refresh fallback example for native free providers"
```

### Task 5: Final Verification and Branch Readiness

**Files:**
- No intended production edits.

**Interfaces:**
- Produces: verified branch ready for merge or PR.

- [ ] **Step 1: Run focused tests**

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./fallback ./router ./controller -count=1
```

- [ ] **Step 2: Run full Go tests**

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./... -count=1
```

- [ ] **Step 3: Build Windows binary**

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go build -o one-api.exe
```

- [ ] **Step 4: Commit any final docs/status changes**

```powershell
git status --short
git log --oneline -5
```

Expected: clean working tree after commits.
