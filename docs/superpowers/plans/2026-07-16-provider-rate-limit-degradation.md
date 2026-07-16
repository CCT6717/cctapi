# Persistent Provider Rate-Limit Degradation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:executing-plans` to implement this plan task-by-task. Every
> production behavior change follows RED, GREEN, REFACTOR.

**Goal:** Detect confirmed provider HTTP 429 failures that recur after separate
cooldowns, lower that provider in every automatic routing strategy, expose safe
runtime diagnostics, and restore priority automatically.

**Architecture:** Extend the existing mutex-protected deployment runtime state
with an explicit degradation state machine. Keep relay code orchestration-only:
it records one provider episode after model-level rotation is exhausted and the
deployment cooldown is applied. Sorting reads an immutable snapshot. The current
runtime endpoint and `/fallback/status` GatewayStatus component expose the safe
diagnostic.

**Tech stack:** Go 1.26.5, Gin, GORM/SQLite tests, React, Semantic UI React,
Vitest, Playwright, PowerShell, Python paced-soak tooling.

---

## Task 1: Provider degradation runtime state machine

**Files:**

- Create: `fallback/provider_rate_limit_degradation.go`
- Create: `fallback/provider_rate_limit_degradation_test.go`
- Modify: `fallback/quota.go`
- Modify: `fallback/state.go`

**Produces:**

```go
type ProviderRateLimitDegradationSnapshot struct {
    Active                       bool       `json:"active"`
    Level                        int        `json:"level"`
    EpisodeCount                 int        `json:"episode_count"`
    Reason                       string     `json:"reason,omitempty"`
    LastRateLimitedAt            *time.Time `json:"last_rate_limited_at,omitempty"`
    NextRecoveryAt               *time.Time `json:"next_recovery_at,omitempty"`
    ConsecutiveRecoverySuccesses int        `json:"consecutive_recovery_successes"`
}

func RecordProviderRateLimitEpisode(deploymentID string, cooldownDuration time.Duration)
func ResetProviderRateLimitDegradation(deploymentID string)
func SnapshotProviderRateLimitDegradation(deploymentID string) ProviderRateLimitDegradationSnapshot
```

Internal timestamp-aware helpers use explicit `now` and `cooldownUntil` values
for deterministic tests. Public wrappers use `time.Now()`.

- [ ] **Step 1: Write state transition tests first**

Add table-driven tests for first observation, second post-cooldown episode L1,
same-cooldown deduplication, 15-minute observation expiry, L3 cap, level-0
success reset, L3 three-success recovery, ten-minute time decay, fixed safe
reason, target-only manual reset, and concurrent record/success/snapshot/decay.

- [ ] **Step 2: Run RED**

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'ProviderRateLimitDegradation' -count=1
```

Expected: compile failure because the snapshot and transition functions do not
exist.

- [ ] **Step 3: Implement the minimal mutex-protected state machine**

Add these runtime fields to `DeploymentRuntimeState`:

```go
RateLimitEpisodeCount              int
RateLimitDegradationLevel          int
LastProviderRateLimitedAt          time.Time
RateLimitEpisodeCooldownUntil      time.Time
NextDegradationRecoveryAt          time.Time
ConsecutiveRecoverySuccesses       int
```

Use constants of 15 minutes, 10 minutes, level 3, penalty 25, and the fixed
reason `repeated rate limits`. Reuse `runtimeStatesMu`; do not add a second lock
for the same deployment state. `RecordSuccess` calls the locked recovery helper.
`DecayRateLimitScores` also calls the locked time-recovery helper. Extend
`RuntimeStateSnapshot` with the nested safe snapshot.

Call `ResetProviderRateLimitDegradation` from `ResetDeploymentState` only after
persistent reset and cooldown clearing succeed, matching existing model-runtime
reset semantics.

- [ ] **Step 4: Run GREEN and race**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'ProviderRateLimitDegradation' -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback -run 'ProviderRateLimitDegradation' -count=5
```

Expected: all selected tests pass with no race report.

- [ ] **Step 5: Commit**

```powershell
git add fallback/provider_rate_limit_degradation.go fallback/provider_rate_limit_degradation_test.go fallback/quota.go fallback/state.go
git commit -m "feat(fallback): track repeated provider rate limits"
```

---

## Task 2: Apply degradation to every automatic routing strategy

**Files:**

- Modify: `fallback/sorting.go`
- Modify: `fallback/integration_test.go`

**Produces:**

```go
func providerRateLimitDegradationPenalty(deploymentID string) float64
```

- [ ] **Step 1: Write routing tests first**

For `quality_first`, `cost_first`, and `free_first`, create otherwise-equivalent
healthy and L1 degraded deployments. Assert the healthy deployment sorts first.
Also assert level penalties are exactly 0, 25, 50, and 75 and that a degraded
deployment remains in the returned candidate list.

Add a plan-level test that preferred-start promotion still places the configured
deployment first after automatic sorting.

- [ ] **Step 2: Run RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'ProviderDegradation|PreferredDeployment' -count=1
```

Expected: healthy peers are not preferred by quality/cost sorting and the new
penalty helper is missing.

- [ ] **Step 3: Implement strategy-independent subtraction**

Refactor `strategyScore` to compute a local score for each strategy and subtract
`level * 25` once before returning. Apply the same penalty in `CalculateScore`
so the admin score endpoint matches actual ordering. Keep the existing
`RateLimitScore` term in `free_first` as the immediate pressure signal.

Do not remove candidates and do not change `PreferDeployment` ordering.

- [ ] **Step 4: Run GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'ProviderDegradation|PreferredDeployment' -count=1
```

- [ ] **Step 5: Commit**

```powershell
git add fallback/sorting.go fallback/integration_test.go
git commit -m "feat(fallback): demote repeatedly limited providers"
```

---

## Task 3: Record only confirmed provider-level relay episodes

**Files:**

- Modify: `controller/relay.go`
- Modify: `controller/relay_fallback_model_rotation_test.go`

**Depends on:**

```go
RecordProviderRateLimitEpisode(deploymentID string, cooldownDuration time.Duration)
```

- [ ] **Step 1: Extend controller tests first**

Add assertions proving:

- first Kilo model 429 followed by another Kilo model success leaves provider
  episode count and degradation level at zero;
- all compatible Kilo models exhausted records one provider episode, even though
  multiple model attempts returned 429;
- a second request after simulated cooldown expiry records episode two and L1;
- HTTP 500 with rate-limit-shaped text records no provider episode;
- Pollinations success changes only Pollinations recovery state.

- [ ] **Step 2: Run RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./controller -run 'RelayWithFallback.*Kilo|ProviderRateLimitDegradation' -count=1
```

Expected: provider episode assertions fail because relay never records the new
state.

- [ ] **Step 3: Record after cooldown application**

In `relayWithFallbackUsing`, after `ApplyRelayCooldown`, call
`RecordProviderRateLimitEpisode` only when:

```go
isConfirmedHTTPRateLimit && cooldownErr == nil && cooldownDuration > 0
```

This location is after intermediate Kilo rotation returns and before continuing
to the next deployment. Do not call it for model capability false positives,
local concurrency skips, quota pre-checks, or non-429 classifications.

- [ ] **Step 4: Run GREEN and focused race**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./controller -run 'RelayWithFallback.*Kilo|ProviderRateLimitDegradation' -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback ./controller -count=1
```

- [ ] **Step 5: Commit**

```powershell
git add controller/relay.go controller/relay_fallback_model_rotation_test.go
git commit -m "feat(fallback): record provider rate-limit episodes"
```

---

## Task 4: Expose safe runtime diagnostics and manual reset

**Files:**

- Modify: `router/fallback.go`
- Modify: `router/fallback_test.go`
- Modify: `fallback/state_test.go`

- [ ] **Step 1: Write API and reset tests first**

Extend runtime-row tests to assert the nested
`provider_rate_limit_degradation` object contains active, level, episode count,
fixed reason, last-rate-limit time, next-recovery time, and recovery-success
count. Marshal the row and reject bearer values, keys, raw bodies, and injected
upstream text.

Extend state reset tests to prove successful manual recovery clears the target
provider degradation and leaves another deployment unchanged. Preserve the
existing contract that in-memory reset does not happen when persistent reset
fails.

- [ ] **Step 2: Run RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./router -run 'ResetDeploymentState|RuntimeStatusRows.*Degradation' -count=1
```

- [ ] **Step 3: Project the existing safe snapshot**

Add this field to `buildFallbackRuntimeStatusRows`:

```go
"provider_rate_limit_degradation": rt.ProviderRateLimitDegradation,
```

Do not add raw error text to the nested object. Manual and batch recover already
call `ResetDeploymentState`, so no duplicate reset logic belongs in the router.

- [ ] **Step 4: Run GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./router -run 'ResetDeploymentState|RuntimeStatusRows.*Degradation' -count=1
```

- [ ] **Step 5: Commit**

```powershell
git add router/fallback.go router/fallback_test.go fallback/state_test.go
git commit -m "feat(fallback): expose provider degradation runtime"
```

---

## Task 5: Render compact Chinese diagnostics on `/fallback/status`

**Files:**

- Modify: `web/default/src/components/gateway-status/GatewayStatus.jsx`
- Modify: `web/default/src/components/gateway-status/GatewayStatus.css`
- Create: `web/default/src/components/gateway-status/GatewayStatus.test.jsx`

- [ ] **Step 1: Write Vitest first**

Mock one free deployment with active L2 degradation and assert the expanded card
contains `持续限流降权 L2`, `跨冷却限流 3 次`, and `预计恢复`. Add an inactive row and
assert it does not render degradation copy. Keep the existing health, quota,
refresh, and recover behavior intact.

- [ ] **Step 2: Run RED**

```powershell
cd web/default
npm test -- src/components/gateway-status/GatewayStatus.test.jsx
```

Expected: the active diagnostic text is absent.

- [ ] **Step 3: Implement compact diagnostic**

Add a small `ProviderDegradationDiagnostic` component used by `DeploymentCard`.
Read only `dep.provider_rate_limit_degradation`; render nothing unless `active`
is true. Use the existing `formatTime` helper and responsive card styles. Keep
all user-facing copy Chinese and do not create a new page or shortcut.

- [ ] **Step 4: Run GREEN and focused frontend tests**

```powershell
npm test -- src/components/gateway-status/GatewayStatus.test.jsx src/pages/Fallback/Fallback.test.jsx src/pages/Fallback/index.test.js
```

- [ ] **Step 5: Commit**

```powershell
git add web/default/src/components/gateway-status/GatewayStatus.jsx web/default/src/components/gateway-status/GatewayStatus.css web/default/src/components/gateway-status/GatewayStatus.test.jsx
git commit -m "feat(fallback): show provider degradation diagnostics"
```

---

## Task 6: Verification, paced soak, review, and integration

**Files:**

- Modify: `scripts/soak-test.py` only if the existing script cannot capture the
  degradation snapshot without secrets
- Create: `docs/evidence/provider-rate-limit-degradation-2026-07-16.json`
- Modify: `AGENTS.md`

- [ ] **Step 1: Run backend gates**

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./controller ./router -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback ./controller ./router -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./... -count=1
& 'D:\ct\tools\go\bin\go.exe' vet ./...
& 'D:\ct\tools\go\bin\go.exe' build ./...
```

- [ ] **Step 2: Run frontend gates**

```powershell
cd web/default
npm run lint
npm test
npm run build
$env:STORYBOOK_DISABLE_TELEMETRY='1'; npm run build-storybook
npm run test:e2e
```

- [ ] **Step 3: Rebuild and restart localhost**

Build `web/default`, build `one-api.exe`, stop only the existing `one-api`
process bound to port 3008, restart with `--port 3008`, and verify HTTP 200.
Do not delete `.env`, `one-api.db`, `.workbuddy/`, or active logs.

- [ ] **Step 4: Run sanitized paced validation**

Run paced traffic through `openrouter/auto` with no retries. Capture the runtime
snapshot before and after. Prove successful normal-provider traffic remains L0
and does not retain an observation episode. Use deterministic Go/controller
fixtures as the authoritative repeated-429 escalation evidence so production
quota timing is not required.

Evidence may contain request IDs, provider IDs, counts, levels, timestamps, and
status categories. It must not contain tokens, keys, authorization headers, raw
upstream bodies, or administrator credentials.

- [ ] **Step 5: Independent review and fixes**

Review state locking, episode deduplication, Kilo boundary, sorting semantics,
manual reset, API redaction, UI mobile behavior, and tests. Any finding is fixed
with a new failing test before production code changes.

- [ ] **Step 6: Update handoff and create PR**

Record fresh verification and soak results in `AGENTS.md`, run `git diff --check`
and a credential-pattern scan, commit with conventional commits, push the branch,
create a non-draft PR, and wait for every required CI job. Merge only after green
CI, then delete local and remote feature branches.

