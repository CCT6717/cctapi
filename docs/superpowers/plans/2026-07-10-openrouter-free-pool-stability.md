# OpenRouter Free Pool Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep OpenRouter free-pool routing on the stable `openrouter/free` alias, reconcile generated resources correctly, and make the real-token smoke reject requests that bypass fallback.

**Architecture:** `fallback` remains the owner of generated free-provider deployments and channels. Dynamic discovery updates the channel's visible model inventory, while a provider-specific routing policy selects the deployment model; gateway saves are normalized through the same ownership rules so derived runtime state cannot become an accidental override. The smoke test proves routing through fallback with positive metrics and usage deltas, while SPA reachability is reported separately from static HTML marker detection.

**Tech Stack:** Go 1.20-compatible code, GORM/SQLite test fixtures, Gin handler tests, PowerShell smoke scripts, React production build, Git worktrees.

## Global Constraints

- Work from latest `main` on `fix/openrouter-free-pool-stability` in an isolated worktree.
- Follow RED-GREEN-REFACTOR for every production behavior change.
- Never print, commit, or persist real API/admin/provider tokens outside the existing ignored runtime files.
- Do not delete or overwrite `.env`, `one-api.db`, or valid provider configuration.
- Keep `data/fallback.json` ignored and out of commits.
- Rebuild `web/default` before rebuilding `one-api.exe`.
- Do not merge when any required test, review gate, or real-token smoke check fails.

---

### Task 1: Make Provider Configuration Own OpenRouter Routing

**Files:**
- Modify: `fallback/free_provider_sync.go`
- Test: `fallback/free_pool_test.go`
- Test: `router/fallback_gateway_test.go`

**Interfaces:**
- Consumes: `BuiltinFreeProviders`, `FreeProviderConfig.Models`, generated `DeploymentConfig` values.
- Produces: `routingModelForFetchedModels(providerName string, fetchedModels []string) string` and `providerConfigOwnsAutoRealModel(cfg *Config, providerName string) bool`.

- [ ] **Step 1: Add failing fallback tests for stable routing ownership**

Add these cases to `fallback/free_pool_test.go` using `setupFreePoolTestDB`:

```go
func TestRoutingModelForFetchedModelsKeepsOpenRouterFreeAlias(t *testing.T) {
	got := routingModelForFetchedModels("openrouter", []string{
		"cognitivecomputations/dolphin-mistral-24b-venice-edition:free",
		"openai/gpt-oss-20b:free",
	})
	if got != "openrouter/free" {
		t.Fatalf("routing model = %q, want openrouter/free", got)
	}
}

func TestSyncFreePoolConfiguredModelsOverridePersistedAutoRealModel(t *testing.T) {
	cleanupDB := setupFreePoolTestDB(t)
	defer cleanupDB()

	key := "gsk-configured-model-owner"
	keyHash := SafeKeyHash(key)
	channel := dbmodel.Channel{
		Name: channelName("groq", keyHash), Type: BuiltinFreeProviders["groq"].ChannelType,
		Key: key, Models: "stale-model", Status: dbmodel.ChannelStatusEnabled,
	}
	if err := dbmodel.DB.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	depID := deploymentID("groq", keyHash)
	cfg := &Config{
		Enabled: true,
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {Enabled: true, Keys: []string{key}, Models: []string{"configured-model"}},
		},
		Deployments: map[string]DeploymentConfig{
			depID: {ID: depID, Enabled: true, ChannelID: channel.Id, RealModel: "stale-model", Pool: "free"},
		},
	}
	if err := SyncFreePool(cfg); err != nil {
		t.Fatalf("SyncFreePool: %v", err)
	}
	if got := cfg.Deployments[depID].RealModel; got != "configured-model" {
		t.Fatalf("real model = %q, want configured-model", got)
	}
}
```

- [ ] **Step 2: Add a failing gateway round-trip regression test**

Add `TestGatewayUpdateConfig_OpenRouterRoundTripKeepsStableAlias` to `router/fallback_gateway_test.go`. Build a temp config containing one OpenRouter key, an `openrouter/auto` virtual model using pool `free`, and an auto deployment whose persisted `real_model` is a specific `:free` model. PUT the same projected configuration through `updateGatewayConfig`, then assert:

```go
cfg := fallback.GetConfig()
depID := "free:openrouter-" + fallback.SafeKeyHash(key)
if got := cfg.Deployments[depID].RealModel; got != "openrouter/free" {
	t.Fatalf("round-trip real model = %q, want openrouter/free", got)
}
```

Use only a fake test key and construct the JSON with `json.Marshal`; do not copy runtime credentials.

- [ ] **Step 3: Run the new tests and verify RED**

Run:

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./fallback ./router -run 'TestRoutingModelForFetchedModelsKeepsOpenRouterFreeAlias|TestSyncFreePoolConfiguredModelsOverridePersistedAutoRealModel|TestGatewayUpdateConfig_OpenRouterRoundTripKeepsStableAlias' -count=1
```

Expected: FAIL because the routing helper does not exist and current sync preserves stale generated models.

- [ ] **Step 4: Implement the minimum ownership policy**

Add to `fallback/free_provider_sync.go`:

```go
func routingModelForFetchedModels(providerName string, fetchedModels []string) string {
	meta, ok := BuiltinFreeProviders[providerName]
	if ok && meta.ModelFetchMode == ModelFetchOpenRouterFree && len(meta.DefaultModels) > 0 {
		return meta.DefaultModels[0]
	}
	if len(fetchedModels) == 0 {
		return ""
	}
	return fetchedModels[0]
}

func providerConfigOwnsAutoRealModel(cfg *Config, providerName string) bool {
	if cfg == nil {
		return false
	}
	fp, ok := cfg.FreeProviders[providerName]
	if !ok {
		return false
	}
	if len(fp.Models) > 0 {
		return true
	}
	meta, ok := BuiltinFreeProviders[providerName]
	return ok && meta.ModelFetchMode == ModelFetchOpenRouterFree
}
```

When merging generated deployments, preserve an existing `RealModel` only when `providerConfigOwnsAutoRealModel` is false. In both keyed and keyless dynamic-sync branches, pass `routingModelForFetchedModels(providerName, models)` to `shouldSyncDeploymentRealModel` and `UpdateDeploymentRealModel` instead of `models[0]`.

- [ ] **Step 5: Verify GREEN and existing override compatibility**

Run:

```powershell
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./fallback ./router -run 'TestRoutingModelForFetchedModelsKeepsOpenRouterFreeAlias|TestSyncFreePoolConfiguredModelsOverridePersistedAutoRealModel|TestGatewayUpdateConfig_OpenRouterRoundTripKeepsStableAlias|TestSyncFreePoolPreservesDeploymentRealModelOverride' -count=1
```

Expected: PASS; the existing Groq manual override test must remain green.

- [ ] **Step 6: Commit Task 1**

```powershell
git add fallback/free_provider_sync.go fallback/free_pool_test.go router/fallback_gateway_test.go
git commit -m "fix: stabilize openrouter free routing"
```

### Task 2: Reconcile Existing Auto-Channel Model Inventories

**Files:**
- Modify: `fallback/free_provider_sync.go`
- Test: `fallback/free_pool_test.go`

**Interfaces:**
- Consumes: `desiredFreeProviderResource.ch` and an existing `model.Channel` with the same key hash.
- Produces: independent key, model-list, and channel-type reconciliation; calls `UpdateAbilities` whenever models change.

- [ ] **Step 1: Add a failing same-key model update test**

Add `TestSyncFreePoolUpdatesChannelModelsWhenKeyIsUnchanged`. Create an enabled auto channel whose key already matches but whose `Models` is `old-model`; configure the provider with `Models: []string{"new-model"}`, call `SyncFreePool`, reload the channel, and assert `Models == "new-model"`.

- [ ] **Step 2: Verify RED**

```powershell
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./fallback -run TestSyncFreePoolUpdatesChannelModelsWhenKeyIsUnchanged -count=1
```

Expected: FAIL with the stale `old-model` value because current reconciliation only updates models when the key changes.

- [ ] **Step 3: Implement field-by-field reconciliation**

Replace the key-only update block with an `updates := map[string]interface{}{}` that independently compares `Key`, `Models`, and `Type`. Persist only changed fields. Track `keyChanged` and `modelsChanged`; call `existingCh.UpdateAbilities()` when `modelsChanged`, and emit separate logs for key rotation and model inventory refresh.

- [ ] **Step 4: Verify GREEN and free-pool regressions**

```powershell
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./fallback -run 'TestSyncFreePoolUpdatesChannelModelsWhenKeyIsUnchanged|TestSyncAllProviderModelsKeepsConfiguredModelOverride|TestMultiKey' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```powershell
git add fallback/free_provider_sync.go fallback/free_pool_test.go
git commit -m "fix: reconcile free provider channel models"
```

### Task 3: Make the OpenRouter Smoke Prove Fallback Usage

**Files:**
- Create: `scripts/fallback_openrouter_auto_smoke_test.go`
- Modify: `scripts/fallback-openrouter-auto-smoke.ps1`
- Modify: `docs/openrouter-auto-stability-runbook.md`
- Modify: `docs/openrouter-auto-acceptance-checklist.md`
- Modify: `docs/openrouter-auto-deployment-acceptance.md`
- Modify: `docs/openrouter-auto-stability-submission.md`
- Modify: `docs/openrouter-auto-final-submission-template.md`

**Interfaces:**
- Consumes: smoke JSON fields, `/metrics`, provider usage rows, and the SPA route response.
- Produces: strict positive `fallbackRequestsDelta`, `usageRequestDelta`, `usageSuccessDelta`, `pageReachable`, and informational `pageContainsOpenRouterAuto`.

- [ ] **Step 1: Add failing black-box smoke tests**

Create Go tests in package `main` under `scripts`. Start an `httptest.Server` that implements the smoke endpoints and invoke the PowerShell script with fake tokens via `os/exec`.

`TestOpenRouterAutoSmokeRejectsZeroFallbackMetricDelta` returns the same `fallback_requests_total` before and after while all other endpoints succeed; assert the PowerShell process exits non-zero.

`TestOpenRouterAutoSmokeReportsSPAReachabilitySeparately` returns positive metrics and usage deltas plus a 200 SPA shell without literal OpenRouter text; assert exit zero and parse the trailing JSON with:

```go
var summary struct {
	Pass                       bool    `json:"pass"`
	FallbackRequestsDelta      float64 `json:"fallbackRequestsDelta"`
	UsageRequestDelta          int     `json:"usageRequestDelta"`
	UsageSuccessDelta          int     `json:"usageSuccessDelta"`
	PageReachable              bool    `json:"pageReachable"`
	PageContainsOpenRouterAuto bool    `json:"pageContainsOpenRouterAuto"`
}
```

Skip only when neither `powershell.exe` nor `pwsh` exists. Keep server responses deterministic and never use real credentials.

- [ ] **Step 2: Verify RED**

```powershell
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./scripts -run 'TestOpenRouterAutoSmokeRejectsZeroFallbackMetricDelta|TestOpenRouterAutoSmokeReportsSPAReachabilitySeparately' -count=1
```

Expected: FAIL because zero metric delta currently passes and `pageReachable`/usage delta fields do not exist.

- [ ] **Step 3: Enforce positive metrics and usage deltas**

In the PowerShell script, compute provider request/success totals before and after the chat calls. Reject `fallbackRequestsDelta -le 0`, `usageRequestDelta -le 0`, or `usageSuccessDelta -le 0`. Keep the existing deployment-prefix and provider-row checks.

- [ ] **Step 4: Separate SPA reachability from static markers**

Set `pageReachable = $true` only after a 2xx response. Keep `pageContainsOpenRouterAuto` as informational because an authenticated React page cannot be proved by raw shell HTML. Add all three delta fields and `pageReachable` to `-OutputJson`.

- [ ] **Step 5: Align all acceptance documents**

Change the mandatory UI condition to `/fallback/free-pool` HTTP reachability. State that provider/catalog/runtime checks prove OpenRouter configuration and that an authenticated browser check is optional when visible UI validation is required. Update result templates with `usageRequestDelta`, `usageSuccessDelta`, and `pageReachable`.

- [ ] **Step 6: Verify GREEN and PowerShell syntax**

```powershell
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./scripts -run 'TestOpenRouterAutoSmoke' -count=1
$tokens=$null; $errors=$null
[System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path 'scripts/fallback-openrouter-auto-smoke.ps1'), [ref]$tokens, [ref]$errors) | Out-Null
if ($errors.Count -gt 0) { $errors | Format-List; exit 1 }
```

Expected: tests PASS and parser error count is zero.

- [ ] **Step 7: Commit Task 3**

```powershell
git add scripts/fallback_openrouter_auto_smoke_test.go scripts/fallback-openrouter-auto-smoke.ps1 docs/openrouter-auto-*.md
git commit -m "test: require real openrouter fallback traffic"
```

### Task 4: Review, Build, and Real-Token Acceptance

**Files:**
- Modify only files required by review findings.

**Interfaces:**
- Consumes: committed Task 1-3 behavior and local ignored runtime credentials/configuration.
- Produces: review-clean branch, rebuilt binary, and authoritative real smoke evidence.

- [ ] **Step 1: Run focused and full Go verification**

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$go='D:\ct\tools\go1.22.12\bin\go.exe'
& $go test ./fallback ./router -count=1
& $go test ./... -count=1
& $go build ./...
git diff --check
```

Expected: all exit 0; Git may report existing line-ending warnings but no whitespace errors.

- [ ] **Step 2: Request independent code review**

Use `superpowers:requesting-code-review` against the complete branch diff. Resolve every correctness, security, secret-handling, regression, or missing-test blocker with RED-GREEN verification.

- [ ] **Step 3: Build frontend before the release binary**

```powershell
Set-Location D:\ct\project\web\default
npm run build
Set-Location D:\ct\project
& $go build -o one-api.exe .
```

Expected: frontend build succeeds with only known inherited warnings, then binary build exits 0.

- [ ] **Step 4: Restart port 3008 safely**

Stop only the process currently listening on port 3008 after verifying its executable is `one-api.exe`. Start the newly built binary hidden with `--port 3008`, then require HTTP 200 from `/` and `/fallback/free-pool`.

- [ ] **Step 5: Run real-token smoke without exposing credentials**

Read one enabled admin access token and one enabled user API token from `one-api.db` inside a single PowerShell process. Set process-local `CCT_ADMIN_TOKEN`, `CCT_API_TOKEN`, and `CCT_API_BASE_URL`, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

Remove the environment variables in `finally`. Require `pass=true`, a `free:openrouter-*` deployment, positive usage deltas, and `fallbackRequestsDelta > 0`.

- [ ] **Step 6: Commit any review-only fixes**

```powershell
git add fallback/free_provider_sync.go fallback/free_pool_test.go router/fallback_gateway_test.go scripts/fallback_openrouter_auto_smoke_test.go scripts/fallback-openrouter-auto-smoke.ps1 docs/openrouter-auto-*.md
git commit -m "fix: address openrouter stability review"
```

Skip this commit when review requires no changes.

### Task 5: Finish and Merge the Branch

**Files:**
- No additional source changes unless the completion audit finds a missing requirement.

**Interfaces:**
- Consumes: green verification evidence and clean review.
- Produces: pushed fix branch, merged/pushed `main`, and final handoff evidence.

- [ ] **Step 1: Audit branch state and secret safety**

```powershell
git status --short --branch
git diff origin/main...HEAD --check
git diff --name-only origin/main...HEAD
git grep -n -I -E 'sk-or-v1-|CCT_ADMIN_TOKEN=.+' -- ':!docs/superpowers/plans/2026-07-10-openrouter-free-pool-stability.md'
```

Confirm no runtime database, `.env`, `data/fallback.json`, provider key, or generated binary is staged.

- [ ] **Step 2: Push the fix branch**

```powershell
git push -u origin fix/openrouter-free-pool-stability
```

- [ ] **Step 3: Finish via the Superpowers branch workflow**

Invoke `superpowers:finishing-a-development-branch`. Because the goal explicitly authorizes integration, merge only after all gates are green:

```powershell
Set-Location D:\ct\project
git switch main
git pull --ff-only origin main
git merge --no-ff fix/openrouter-free-pool-stability
git push origin main
```

- [ ] **Step 4: Verify merged main**

```powershell
git status --short --branch
git log -3 --oneline
curl.exe -I http://127.0.0.1:3008/
curl.exe -I http://127.0.0.1:3008/fallback/free-pool
```

Report the branch commits, merge commit, test totals, smoke JSON summary, live URLs, and any residual external-provider risk.
