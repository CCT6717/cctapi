# Kilo Model-Level Rate-Limit Rotation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rotate a generated Kilo deployment to another compatible catalog model after a pre-response HTTP 429, and penalize the provider only when all compatible Kilo models are unavailable.

**Architecture:** Keep provider deployment configuration immutable during requests. Add an in-memory model runtime registry and a request-scoped Kilo model planner, then let the controller execute explicit model attempts while preserving existing provider quota, cooldown, score, sticky, and fallback behavior. Project model runtime snapshots through the existing deployment runtime endpoint and render a compact Chinese diagnostic in the free-pool UI.

**Tech Stack:** Go 1.26.5, Gin, GORM/SQLite test fixtures, React, Semantic UI React, Vitest, Playwright.

## Global Constraints

- Apply rotation only to generated Kilo free-pool deployments.
- Rotate only structured `ErrorCategoryRateLimit` failures before response bytes are written.
- Never mutate shared `DeploymentConfig`, catalog snapshots, or operator configuration during a request.
- Filter every alternative by stream, tools, JSON, vision, and context requirements.
- Increment provider failure and `RateLimitScore` once only when the compatible Kilo model set is exhausted.
- Keep short model cooldowns in memory; preserve existing persistent deployment cooldown as provider isolation.
- Never expose API keys, raw upstream bodies, or unredacted secrets in runtime data.
- Keep all free-pool user-facing copy in Chinese while preserving provider names and technical abbreviations.
- Keep the existing `3008` service running until final integration; implement in an isolated worktree.
- Use `D:\ct\worktrees\kilo-model-rate-limit-rotation` as the isolated worktree path.
- Use `D:\ct\tools\go\bin\go.exe` with `D:\ct\tools\w64devkit-1.23.0\bin` for CGO/SQLite verification.

---

### Task 1: Model Runtime Registry

**Files:**
- Create: `fallback/free_provider_model_runtime.go`
- Create: `fallback/free_provider_model_runtime_test.go`

**Interfaces:**
- Consumes: `CalculateRelayCooldownDuration(input RelayCooldownInput) time.Duration` from `fallback/cooldown.go`.
- Produces: `MarkFreeProviderModelRateLimited(deploymentID, modelID, reason string, input RelayCooldownInput) time.Duration`.
- Produces: `RecordFreeProviderModelSuccess(deploymentID, modelID string)`.
- Produces: `IsFreeProviderModelCooling(deploymentID, modelID string) bool`.
- Produces: `SnapshotFreeProviderModelRuntime(deploymentID string) FreeProviderModelRuntimeSummary`.
- Produces: `ResetFreeProviderModelRuntime(deploymentID string)` for the existing manual deployment reset path.

- [ ] **Step 1: Write failing registry tests**

Create table-driven tests covering Retry-After clamping, default 60-second cooldown, expiry, success reset, deep-copy snapshots, deployment reset, and concurrent readers/writers. The primary assertion shape is:

```go
func TestFreeProviderModelRuntimeRateLimitExpires(t *testing.T) {
    resetFreeProviderModelRuntimeForTest()
    t.Cleanup(resetFreeProviderModelRuntimeForTest)
    now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
    freeProviderModelRuntimeNow = func() time.Time { return now }

    retryAfter := 120
    got := MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", "rate limited", RelayCooldownInput{
        Category: ErrorCategoryRateLimit, StatusCode: http.StatusTooManyRequests,
        RetryAfterSeconds: &retryAfter, Attempt: 1,
    })
    if got != 120*time.Second || !IsFreeProviderModelCooling("free:kilo-test", "model-a") {
        t.Fatalf("unexpected cooldown: %s", got)
    }
    now = now.Add(121 * time.Second)
    if IsFreeProviderModelCooling("free:kilo-test", "model-a") {
        t.Fatal("expired model cooldown remained active")
    }
}
```

- [ ] **Step 2: Run the tests and confirm RED**

Run:

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestFreeProviderModelRuntime' -count=1
```

Expected: compile failure because the model runtime types/functions do not exist.

- [ ] **Step 3: Implement the locked runtime registry**

Define snapshots with JSON-safe fields and private mutable entries:

```go
type FreeProviderModelRuntimeEntrySnapshot struct {
    ModelID             string     `json:"model_id"`
    CooldownUntil       *time.Time `json:"cooldown_until,omitempty"`
    CooldownActive      bool       `json:"cooldown_active"`
    Reason              string     `json:"reason,omitempty"`
    LastRateLimitedAt   *time.Time `json:"last_rate_limited_at,omitempty"`
    Consecutive429Count int        `json:"consecutive_429_count"`
    SuccessCount        int        `json:"success_count"`
    FailureCount        int        `json:"failure_count"`
    LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
}

type FreeProviderModelRuntimeSummary struct {
    ActiveCooldownCount int                                         `json:"active_cooldown_count"`
    LastAttemptedModel  string                                      `json:"last_attempted_model,omitempty"`
    LastSuccessfulModel string                                      `json:"last_successful_model,omitempty"`
    Models              []FreeProviderModelRuntimeEntrySnapshot     `json:"models"`
}
```

Use one `sync.RWMutex`, a nested `map[deploymentID]map[modelID]*entry`, and `freeProviderModelRuntimeNow = time.Now`. Prune expired entries while producing snapshots, sort snapshots by model ID, clone all time pointers, and never return internal maps or pointers.

- [ ] **Step 4: Run focused tests and race tests GREEN**

Run:

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestFreeProviderModelRuntime' -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback -run 'TestFreeProviderModelRuntime' -count=3
```

Expected: all selected tests pass and the race detector reports no race.

- [ ] **Step 5: Commit the runtime registry**

```powershell
git add fallback/free_provider_model_runtime.go fallback/free_provider_model_runtime_test.go
git commit -m "feat(fallback): track Kilo model cooldowns"
```

---

### Task 2: Capability-Aware Kilo Model Attempt Planner

**Files:**
- Create: `fallback/free_provider_model_plan.go`
- Create: `fallback/free_provider_model_plan_test.go`
- Modify: `fallback/orchestrator.go:3-65`
- Modify: `fallback/orchestrator_test.go`

**Interfaces:**
- Consumes: `GetFreeProviderCatalogSnapshot(deploymentID string) (FreeProviderCatalogSnapshot, bool)`.
- Consumes: `applyFreeModelCapabilities(dep DeploymentConfig, entry FreeModelCatalogEntry) DeploymentConfig`.
- Consumes: `FilterByCapability(deployments []DeploymentConfig, caps RequestCapabilities) []DeploymentConfig`.
- Consumes: `IsFreeProviderModelCooling(deploymentID, modelID string) bool` from Task 1.
- Produces: `DeploymentModelAttempt` and `DeploymentModelPlan`.
- Produces: `PrepareDeploymentModelPlan(dep DeploymentConfig, caps RequestCapabilities) DeploymentModelPlan`.
- Produces: `PrepareDeploymentModelAttempts(deployments []DeploymentConfig, caps RequestCapabilities) []DeploymentModelAttempt`.

- [ ] **Step 1: Write failing planner tests**

Cover these exact cases:

1. Non-Kilo deployment yields one non-rotatable attempt.
2. Kilo configured model is first, followed by catalog order.
3. A cooling model is removed while `CompatibleCount` still includes it.
4. Tools/JSON/vision/context requests exclude incompatible models.
5. An alternative capable Kilo model keeps the provider in the orchestrator plan even when the configured model lacks the requested capability.
6. Catalog absence falls back to the configured model without rotation.

Use these public shapes:

```go
type DeploymentModelAttempt struct {
    Deployment       DeploymentConfig
    ProviderName     string
    ModelIndex       int
    ModelCount       int
    CompatibleCount int
    CoolingCount    int
    Rotatable        bool
}

type DeploymentModelPlan struct {
    Attempts         []DeploymentModelAttempt
    CompatibleCount int
    CoolingCount    int
    Rotatable       bool
}
```

- [ ] **Step 2: Run the planner tests and confirm RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestPrepareDeploymentModel|TestPrepareDeploymentsKeepsKiloAlternative' -count=1
```

Expected: compile failure for missing model plan types/functions.

- [ ] **Step 3: Implement deterministic candidate planning**

Implement the Kilo guard with the existing deployment-ID parser:

```go
providerName, generated := FreeProviderNameFromDeploymentID(dep.ID)
if !generated || providerName != "kilo" {
    candidates := FilterByCapability([]DeploymentConfig{dep}, caps)
    // Return zero or one non-rotatable attempt.
}
```

For Kilo, clone each catalog entry into a copied deployment using
`applyFreeModelCapabilities`, filter with `FilterByCapability`, promote
`dep.RealModel`, then remove actively cooling models. `CompatibleCount` is
counted before cooldown filtering. Assign `ModelIndex` and `ModelCount` only
after filtering so controller boundary checks are stable.

- [ ] **Step 4: Integrate model-aware capability filtering in the orchestrator**

Replace direct `FilterByCapability` use with a helper that retains a deployment
when `PrepareDeploymentModelPlan(dep, caps).CompatibleCount > 0`:

```go
func filterDeploymentsWithModelCapabilities(deployments []DeploymentConfig, caps RequestCapabilities) []DeploymentConfig {
    out := make([]DeploymentConfig, 0, len(deployments))
    for _, dep := range deployments {
        if PrepareDeploymentModelPlan(dep, caps).CompatibleCount > 0 {
            out = append(out, dep)
        }
    }
    return out
}
```

Do not change provider sorting, preferred deployment, or sticky promotion.

- [ ] **Step 5: Run focused and package tests GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestPrepareDeploymentModel|TestPrepareDeployments' -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -count=1
```

Expected: focused and full fallback package tests pass.

- [ ] **Step 6: Commit the model planner**

```powershell
git add fallback/free_provider_model_plan.go fallback/free_provider_model_plan_test.go fallback/orchestrator.go fallback/orchestrator_test.go
git commit -m "feat(fallback): plan compatible Kilo model attempts"
```

---

### Task 3: Relay Kilo 429 Rotation And Provider Exhaustion

**Files:**
- Modify: `controller/relay.go:43-435`
- Create: `controller/relay_fallback_model_rotation_test.go`
- Modify: `fallback/free_provider_model_runtime.go`
- Modify: `fallback/free_provider_model_runtime_test.go`

**Interfaces:**
- Consumes: `PrepareDeploymentModelAttempts(deployments []DeploymentConfig, caps RequestCapabilities) []DeploymentModelAttempt`.
- Consumes: `MarkFreeProviderModelRateLimited`, `RecordFreeProviderModelSuccess`, and existing provider cooldown/accounting functions.
- Produces: `type fallbackRelayExecutor func(*gin.Context, int) *model.ErrorWithStatusCode`.
- Produces: `relayWithFallbackUsing(c *gin.Context, execute fallbackRelayExecutor)`; production `relayWithFallback` passes `relayHelper`.
- Produces: `HasNextModel() bool` and `RemainingModelAttempts() int` methods on `DeploymentModelAttempt`.

- [ ] **Step 1: Add an injectable relay boundary without changing behavior**

Refactor only the function entry:

```go
type fallbackRelayExecutor func(*gin.Context, int) *model.ErrorWithStatusCode

func relayWithFallback(c *gin.Context) {
    relayWithFallbackUsing(c, relayHelper)
}

func relayWithFallbackUsing(c *gin.Context, execute fallbackRelayExecutor) {
    // Existing body, with execute(c, relayMode) replacing relayHelper(c, relayMode).
}
```

Run `& 'D:\ct\tools\go\bin\go.exe' test ./controller -count=1` and confirm the
package remains green. Commit this seam together with the completed rotation in
Step 8.

- [ ] **Step 2: Write failing controller rotation tests**

Use an in-memory channel DB, a Kilo catalog snapshot, a fallback config with
Kilo then Pollinations, and an injected executor that records
`ctxkey.FallbackRealModel`. Cover:

```go
executor := func(c *gin.Context, _ int) *relaymodel.ErrorWithStatusCode {
    attempted = append(attempted, c.GetString(ctxkey.FallbackRealModel))
    if len(attempted) == 1 {
        retryAfter := 30
        return &relaymodel.ErrorWithStatusCode{
            StatusCode: http.StatusTooManyRequests,
            RetryAfterSeconds: &retryAfter,
            Error: relaymodel.Error{Message: "model limited", Type: "rate_limit_error", Code: "rate_limit"},
        }
    }
    return nil
}
```

Assertions:

- first Kilo model 429 then second Kilo model succeeds;
- Kilo deployment cooldown remains inactive after that success;
- provider failure count and rate-limit score remain unchanged;
- all Kilo models 429 applies one provider failure/cooldown then Pollinations is attempted;
- a Kilo 500 skips remaining Kilo models and moves directly to Pollinations;
- a writer with already-started stream output is not replayed;
- fallback switch events are not emitted between two models of the same deployment.

- [ ] **Step 3: Run controller tests and confirm RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./controller -run 'TestRelayWithFallback.*Kilo' -count=1
```

Expected: assertions fail because the controller still cools the whole Kilo deployment after the first 429.

- [ ] **Step 4: Execute request-scoped model attempts**

Flatten provider plans with `PrepareDeploymentModelAttempts`. Keep separate
`deploymentCount := len(plan.Deployments)` and `upstreamAttemptCount` counters.
Suppress switch logs when `prevDeployment == dep.ID`.

After classification and the written-stream guard, handle rotatable Kilo 429:

```go
modelCooldown := fallback.MarkFreeProviderModelRateLimited(
    dep.ID, dep.RealModel, getRelayErrorMessage(bizErr), fallback.RelayCooldownInput{
        Category: errClass.Category, StatusCode: bizErr.StatusCode,
        RetryAfterSeconds: bizErr.RetryAfterSeconds, Attempt: 1,
    },
)
if attempt.HasNextModel() {
    logger.Infof(ctx, "[fallback] Kilo model %s cooling down for %.0fs; rotating within deployment %s",
        dep.RealModel, modelCooldown.Seconds(), dep.ID)
    continue
}
```

This branch must run before provider `RecordDeploymentError`, `RecordFailure`,
monitor disable/emission, and `ApplyRelayCooldown`. On the final candidate, fall
through to those existing provider-level operations once. On any non-429
provider failure, add `attempt.RemainingModelAttempts()` to the flat loop index
so unused Kilo alternatives are not tried.

- [ ] **Step 5: Record success against the actual request model**

On Kilo success call `RecordFreeProviderModelSuccess(dep.ID, dep.RealModel)` and
keep existing provider success, sticky deployment, quota, and usage accounting.
Ensure every `ctxkey.FallbackRealModel`, log, and `RecordFallbackDeploymentSuccess`
uses the attempt copy's model.

- [ ] **Step 6: Add reset integration**

Call `ResetFreeProviderModelRuntime(deploymentID)` from the same manual recovery
path that clears deployment cooldown/runtime state. Add a focused test proving
manual reset removes active Kilo model cooldowns without affecting another
deployment.

- [ ] **Step 7: Run controller, fallback, and race tests GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./controller -run 'TestRelayWithFallback.*Kilo' -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./controller -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback ./controller -count=1
```

Expected: all tests pass and the race detector reports no race.

- [ ] **Step 8: Commit relay rotation**

```powershell
git add controller/relay.go controller/relay_fallback_model_rotation_test.go fallback/free_provider_model_runtime.go fallback/free_provider_model_runtime_test.go
git commit -m "feat(fallback): rotate Kilo models after rate limits"
```

---

### Task 4: Runtime API And Chinese Admin Visibility

**Files:**
- Modify: `router/fallback.go:481-539`
- Modify: `router/fallback_test.go`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.js:94-143`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.test.jsx`
- Modify: `web/default/src/pages/Fallback/Fallback.css`

**Interfaces:**
- Consumes: `SnapshotFreeProviderModelRuntime(deploymentID string) FreeProviderModelRuntimeSummary`.
- Produces: runtime row field `model_runtime`.
- Produces: `renderModelRuntimeDiagnostics(runtime.model_runtime)` inside the existing free deployment diagnostic area.

- [ ] **Step 1: Write failing router projection test**

Seed one active Kilo model cooldown, call `buildFallbackRuntimeStatusRows`, and
assert the Kilo row contains:

```go
modelRuntime := row["model_runtime"].(fallback.FreeProviderModelRuntimeSummary)
if modelRuntime.ActiveCooldownCount != 1 || len(modelRuntime.Models) != 1 {
    t.Fatalf("unexpected model runtime: %#v", modelRuntime)
}
```

Marshal the row and assert that known key material and raw upstream body text
are absent.

- [ ] **Step 2: Run router test RED, add projection, run GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./router -run 'TestBuildFallbackRuntimeStatusRows.*ModelRuntime' -count=1
```

Add:

```go
"model_runtime": fallback.SnapshotFreeProviderModelRuntime(id),
```

Expected after implementation: router test passes.

- [ ] **Step 3: Write failing frontend diagnostic test**

Extend the existing runtime diagnostics fixture with:

```js
model_runtime: {
  active_cooldown_count: 1,
  last_attempted_model: 'kilo/model-a:free',
  last_successful_model: 'kilo/model-b:free',
  models: [{
    model_id: 'kilo/model-a:free',
    cooldown_active: true,
    reason: 'rate limited',
    consecutive_429_count: 2,
  }],
},
```

Assert visible Chinese copy includes `1 个 Kilo 模型冷却中`, both model IDs,
and `连续 429：2`.

- [ ] **Step 4: Run Vitest RED and implement compact diagnostics**

```powershell
Set-Location D:\ct\worktrees\kilo-model-rate-limit-rotation\web\default
npm test -- src/components/fallback-gateway/FreeModelPool.test.jsx
```

In `renderRuntimeDiagnostics`, include model runtime in `hasDiagnostics`, add a
yellow mini label, and render only actively cooling model entries in a compact
list. Add `.free-deployment-model-runtime` and
`.free-deployment-model-runtime-entry` rules to the existing fallback stylesheet
for wrapping long model IDs on desktop and mobile; do not add nested cards.

- [ ] **Step 5: Run focused frontend and router tests GREEN**

```powershell
Set-Location D:\ct\worktrees\kilo-model-rate-limit-rotation\web\default
npm test -- src/components/fallback-gateway/FreeModelPool.test.jsx
Set-Location D:\ct\worktrees\kilo-model-rate-limit-rotation
& 'D:\ct\tools\go\bin\go.exe' test ./router -count=1
```

Expected: focused Vitest and router package tests pass.

- [ ] **Step 6: Commit observability**

```powershell
git add router/fallback.go router/fallback_test.go web/default/src/components/fallback-gateway/FreeModelPool.js web/default/src/components/fallback-gateway/FreeModelPool.test.jsx web/default/src/pages/Fallback/Fallback.css
git commit -m "feat(fallback): show Kilo model cooldowns"
```

---

### Task 5: Full Verification, Live Acceptance, Review, And Integration

**Files:**
- Create: `docs/evidence/kilo-model-rotation-2026-07-14.json`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: the production Kilo/OpenAI-compatible relay, existing `3008` deployment, authenticated free-pool admin API, and CI workflow.
- Produces: reproducible acceptance evidence with tokens and secrets removed.

- [ ] **Step 1: Format and run complete backend verification**

```powershell
gofmt -w fallback/free_provider_model_runtime.go fallback/free_provider_model_runtime_test.go fallback/free_provider_model_plan.go fallback/free_provider_model_plan_test.go fallback/orchestrator.go fallback/orchestrator_test.go controller/relay.go controller/relay_fallback_model_rotation_test.go router/fallback.go router/fallback_test.go
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./... -count=1
& 'D:\ct\tools\go\bin\go.exe' vet ./...
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback ./controller ./router -count=1
& 'D:\ct\tools\go\bin\go.exe' build ./...
```

Expected: all commands exit 0 with no test, vet, race, or build failures.

- [ ] **Step 2: Run complete frontend verification**

```powershell
Set-Location D:\ct\worktrees\kilo-model-rate-limit-rotation\web\default
npm run lint
npm test
npm run build
$env:STORYBOOK_DISABLE_TELEMETRY='1'; npm run build-storybook
npm run test:e2e
```

Expected: ESLint has zero errors/warnings; all Vitest and Playwright tests pass;
Vite and Storybook builds exit 0. Asset-size warnings are informational.

- [ ] **Step 3: Review security and behavior boundaries**

Dispatch two independent reviewers:

1. correctness/concurrency reviewer for request attempts, accounting-once,
   cooldown expiry, stream replay, and race safety;
2. architecture/security reviewer for controller boundaries, secret redaction,
   API compatibility, and UI scope.

Fix every P0-P2 finding with a failing regression test first. Re-run the focused
test plus Steps 1-2 after fixes.

- [ ] **Step 4: Build the integrated binary without interrupting `3008` early**

Build to a temporary path first:

```powershell
Set-Location D:\ct\worktrees\kilo-model-rate-limit-rotation
& 'D:\ct\tools\go\bin\go.exe' build -o one-api.next.exe .
```

Only after all static verification passes, stop the existing `one-api` process,
atomically replace `one-api.exe`, and restart with the existing environment on
port `3008`.

- [ ] **Step 5: Run live protocol acceptance**

Using a temporary local API token that is never written to the repository:

1. Confirm `/`, `/fallback/free-pool`, and `/api/status` return HTTP 200.
2. Run Chat Completions non-stream and stream through Kilo.
3. Run Responses non-stream and stream through Kilo.
4. Run one structured tools request and verify only tool-capable catalog models
   are attempted.
5. Exercise model rotation with a deterministic local test upstream or accepted
   Kilo 429 evidence: first model 429, second compatible model success.
6. Exercise all-model exhaustion and verify fallback reaches Pollinations or
   another enabled provider while Kilo gains one provider penalty/cooldown.
7. Wait through a short test cooldown and verify automatic recovery.
8. Verify the admin runtime endpoint/UI shows active model cooldowns and no keys.

Record timestamps, request mode, attempted model sequence, status, provider
fallback, cooldown deadline, and recovery result in the evidence JSON. Replace
tokens, credentials, and request bodies with redacted summaries.

- [ ] **Step 6: Update handoff and perform final repository checks**

Update `AGENTS.md` with the new feature, evidence path, exact verification
counts, runtime PID, and remaining external quota caveats. Then run:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors and only intentional tracked changes plus the
pre-existing untracked `.workbuddy/` in the main checkout.

- [ ] **Step 7: Commit, merge, push, and confirm CI**

```powershell
git add AGENTS.md docs/evidence/kilo-model-rotation-2026-07-14.json
git commit -m "docs: record Kilo model rotation acceptance"
```

Fast-forward the reviewed feature branch into `main`, push `origin/main`, wait
for the GitHub Actions run for the pushed SHA, and confirm every job succeeds.
Remove the isolated worktree and local feature branch only after the pushed SHA
and CI result are verified.
