# Precise Attempt Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace switch-log-derived failure rankings with exact one-hour attempt-event aggregation and add a bounded process-local view of complete recent routing chains.

**Architecture:** SQLite remains the source for exact persisted failure and skip aggregation. A mutex-protected bounded ring records every relay attempt before the existing persistence policy, preserving terminal successes without writing routine successes to SQLite. An admin-only API returns explicit safe DTOs, and the existing runtime panel consumes them through a focused polling hook.

**Tech Stack:** Go 1.26.5, Gin, GORM/SQLite, React 18, Semantic UI React, Vitest, Testing Library, Playwright.

## Global Constraints

- Work only in `D:\ct\worktrees\attempt-observability` on `codex/attempt-observability`.
- Keep controllers and routers as orchestration boundaries; aggregation stays in `fallback`.
- Do not persist routine success events to SQLite.
- Do not return raw upstream errors, request/response bodies, Authorization, API keys, tokens, cookies, or database errors.
- Keep UI copy Chinese while preserving provider names and technical abbreviations.
- Do not add a dashboard shortcut or standalone connectivity-test panel.
- Use TDD for every behavior change and conventional commits for every task.
- Use `CGO_ENABLED=1` and `D:\ct\tools\w64devkit-1.23.0\bin` for Go tests.

---

### Task 1: Bounded Recent Attempt Chains

**Files:**
- Create: `fallback/attempt_trace.go`
- Create: `fallback/attempt_trace_test.go`
- Modify: `fallback/attempt_event.go`

**Interfaces:**
- Consumes: `fallback.AttemptEvent` and `RecordAttemptEventIfWorthy(event AttemptEvent) error`.
- Produces: `SnapshotRecentAttemptChains(limit int) []AttemptRequestChain` and package-private `resetRecentAttemptTrace()`.

- [ ] **Step 1: Write failing chain-order and terminal-success tests**

Create DTOs expected by the test:

```go
type AttemptTraceStep struct {
    CreatedAt            time.Time      `json:"created_at"`
    Provider             string         `json:"provider"`
    DeploymentID         string         `json:"deployment_id"`
    RealModel            string         `json:"real_model"`
    Outcome              AttemptOutcome `json:"outcome"`
    StatusCode           int            `json:"status_code"`
    ErrorCategory        string         `json:"error_category"`
    DurationMs           int64          `json:"duration_ms"`
    StreamWritten        bool           `json:"stream_written"`
    PlanIndex            int            `json:"plan_index"`
    UpstreamAttemptIndex int            `json:"upstream_attempt_index"`
}

type AttemptRequestChain struct {
    RequestID    string             `json:"request_id"`
    VirtualModel string             `json:"virtual_model"`
    StartedAt    time.Time          `json:"started_at"`
    FinishedAt   time.Time          `json:"finished_at"`
    Steps        []AttemptTraceStep `json:"steps"`
}
```

Tests must record `kilo/a` model 429, `kilo/b` success, and a later request through `RecordAttemptEventIfWorthy`; assert success is absent from SQLite but present as the terminal chain step and chains are newest-first.

- [ ] **Step 2: Run the focused test and verify RED**

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestRecentAttempt' -count=1
```

Expected: compilation fails because `SnapshotRecentAttemptChains` does not exist.

- [ ] **Step 3: Implement the bounded trace store**

Use one package-level store:

```go
const (
    recentAttemptEventLimit = 1000
    recentAttemptChainLimit = 20
    recentAttemptStepLimit  = 50
)

var recentAttemptTrace = struct {
    sync.RWMutex
    events []AttemptEvent
}{}
```

Append a value copy under the write lock and retain only the newest 1000 events. Snapshot a copied slice under the read lock, then group and sort outside the lock. Cap caller limits to 20 and per-request steps to 50. Call `recordRecentAttempt(event)` at the start of `RecordAttemptEventIfWorthy`, before the success persistence switch.

- [ ] **Step 4: Add bounds, deep-copy, and concurrency tests**

Tests must assert 1001 events retain 1000, limit 0 defaults to 20, limit 200 caps at 20, mutating one returned snapshot cannot mutate a later snapshot, and concurrent record/snapshot/reset completes under `-race`.

- [ ] **Step 5: Run GREEN and race verification**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestRecentAttempt' -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback -run 'TestRecentAttempt' -count=1
```

Expected: all focused tests pass with no race report.

- [ ] **Step 6: Commit**

```powershell
git add fallback/attempt_trace.go fallback/attempt_trace_test.go fallback/attempt_event.go
git commit -m "feat(fallback): retain recent attempt chains"
```

---

### Task 2: Exact One-Hour Failure Aggregation

**Files:**
- Create: `fallback/attempt_observability.go`
- Create: `fallback/attempt_observability_test.go`

**Interfaces:**
- Consumes: `AttemptEvent`, `SnapshotRecentAttemptChains` and the active `model.DB`.
- Produces: `SnapshotAttemptObservability() (AttemptObservabilitySnapshot, error)` and deterministic package-private `snapshotAttemptObservabilityAt(now time.Time)`.

- [ ] **Step 1: Write failing classification and aggregation tests**

Define explicit safe DTOs:

```go
type AttemptAggregateItem struct {
    Key          string    `json:"key"`
    DeploymentID string    `json:"deployment_id,omitempty"`
    Provider     string    `json:"provider,omitempty"`
    RealModel    string    `json:"real_model,omitempty"`
    Category     string    `json:"category,omitempty"`
    Outcome      string    `json:"outcome,omitempty"`
    Count        int64     `json:"count"`
    LastSeenAt   time.Time `json:"last_seen_at"`
}

type AttemptObservabilitySnapshot struct {
    GeneratedAt          time.Time              `json:"generated_at"`
    FailureWindowSeconds int64                  `json:"failure_window_seconds"`
    FailureEventCount    int64                  `json:"failure_event_count"`
    SkipEventCount       int64                  `json:"skip_event_count"`
    TopDeployments       []AttemptAggregateItem `json:"top_deployments"`
    TopProviders         []AttemptAggregateItem `json:"top_providers"`
    TopModels            []AttemptAggregateItem `json:"top_models"`
    ErrorCategories      []AttemptAggregateItem `json:"error_categories"`
    Outcomes             []AttemptAggregateItem `json:"outcomes"`
    RecentChains         []AttemptRequestChain  `json:"recent_chains"`
    RecentChainScope     string                 `json:"recent_chain_scope"`
}
```

Insert events before, at, and after the one-hour boundary. Assert only `created_at >= now-1h` is included. Assert real failures require `upstream_attempt_index > 0` and one of `failure`, `model_rate_limited`, `non_fallbackable`, or `model_capability_false_positive`; `skipped_*` contributes only to `SkipEventCount` and outcome rows.

- [ ] **Step 2: Run the focused test and verify RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestAttemptObservability' -count=1
```

Expected: compilation fails because the snapshot types/functions do not exist.

- [ ] **Step 3: Implement SQL aggregation helpers**

Initialize the store through `initAttemptEventStore()`. Use GORM `Select`, `Where`, `Group`, `Order`, `Limit(5)` and `Scan` rather than loading the full event window. Use stable order `count DESC, last_seen_at DESC, key ASC`. Return empty non-nil slices for an empty database. Keep the window fixed at `time.Hour` and set `RecentChainScope` to `process`.

- [ ] **Step 4: Add stable-order, Top limit, empty DB, and safe-field tests**

Marshal the snapshot and assert it does not contain sentinels `Bearer`, `sk-`, `password`, `raw_upstream_body`, `content_preview`, or injected error text. Insert six equal-count groups and assert deterministic five-item output.

- [ ] **Step 5: Run GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run 'TestAttemptObservability' -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -count=1
```

Expected: focused and full fallback tests pass.

- [ ] **Step 6: Commit**

```powershell
git add fallback/attempt_observability.go fallback/attempt_observability_test.go
git commit -m "feat(fallback): aggregate precise attempt failures"
```

---

### Task 3: Admin Observability API

**Files:**
- Modify: `router/fallback.go`
- Modify: `router/fallback_test.go`

**Interfaces:**
- Consumes: `fallback.SnapshotAttemptObservability()`.
- Produces: admin-only `GET /api/fallback/attempt-observability`.

- [ ] **Step 1: Write failing handler tests**

Add a direct handler helper test for success and store failure. The success body must contain `failure_window_seconds`, `top_deployments`, `recent_chains`, and `recent_chain_scope`. The failure body must be HTTP 503 with exactly the safe message `attempt observability unavailable` and must not include the database error sentinel.

- [ ] **Step 2: Run the router test and verify RED**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./router -run 'TestAttemptObservability' -count=1
```

Expected: fails because the handler/route is absent.

- [ ] **Step 3: Implement the route and handler**

Register inside the existing `adminGroup`:

```go
adminGroup.GET("/attempt-observability", getAttemptObservability)
```

The handler calls the fallback snapshot and returns either `{success:true,data:snapshot}` or `{success:false,message:"attempt observability unavailable"}`. Log the concrete error through the existing project logger; do not use `fmt.Println`.

- [ ] **Step 4: Verify admin inheritance and JSON safety**

Add a route-level test using the existing admin test router pattern: unauthenticated access must not return the data payload. Marshal the success response and scan for security sentinels.

- [ ] **Step 5: Run GREEN**

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./router -run 'TestAttemptObservability' -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./router -count=1
```

- [ ] **Step 6: Commit**

```powershell
git add router/fallback.go router/fallback_test.go
git commit -m "feat(router): expose attempt observability"
```

---

### Task 4: Focused Frontend Polling Hook

**Files:**
- Create: `web/default/src/pages/Fallback/hooks/useAttemptObservability.js`
- Create: `web/default/src/pages/Fallback/hooks/useAttemptObservability.test.jsx`

**Interfaces:**
- Consumes: `GET /api/fallback/attempt-observability` through the existing `API` helper.
- Produces: `useAttemptObservability()` returning `{ data, error, loading, refresh }`.

- [ ] **Step 1: Write failing hook tests**

Mock `API.get`. Assert first load fetches once, a successful payload replaces data, a later failure preserves the last successful data and sets the Chinese error `精准尝试数据暂时不可用`, and unmount clears the 30-second timer.

- [ ] **Step 2: Run the hook test and verify RED**

```powershell
cd web/default
npm test -- src/pages/Fallback/hooks/useAttemptObservability.test.jsx
```

Expected: fails because the hook module does not exist.

- [ ] **Step 3: Implement the minimal hook**

Use `useCallback`, `useEffect`, and `useState`. Guard against state updates after unmount. Fetch immediately and every 30 seconds. Normalize absent arrays to empty arrays and never expose Axios error text to the component.

- [ ] **Step 4: Run GREEN**

```powershell
npm test -- src/pages/Fallback/hooks/useAttemptObservability.test.jsx
```

- [ ] **Step 5: Commit**

```powershell
git add web/default/src/pages/Fallback/hooks/useAttemptObservability.js web/default/src/pages/Fallback/hooks/useAttemptObservability.test.jsx
git commit -m "feat(ui): load precise attempt diagnostics"
```

---

### Task 5: Precise Chinese Runtime Diagnostics UI

**Files:**
- Modify: `web/default/src/pages/Fallback/panels/MetricsPanel.js`
- Create: `web/default/src/pages/Fallback/panels/MetricsPanel.test.jsx`
- Modify: `web/default/src/pages/Fallback/Fallback.css`

**Interfaces:**
- Consumes: `useAttemptObservability()`.
- Produces: precise Top lists, separate skip counts, error-category list, and recent route chains in the existing runtime panel.

- [ ] **Step 1: Write failing component tests**

Mock the hook with one Kilo model 429 step, one Kilo success step, one skipped quota count, and Top deployment/provider/model rows. Assert Chinese headings `精准失败诊断`, `最近请求链路`, `真实上游失败`, and `本地跳过`; assert both model names and terminal `成功` appear. Assert injected raw error and token sentinels do not render.

- [ ] **Step 2: Run the component test and verify RED**

```powershell
npm test -- src/pages/Fallback/panels/MetricsPanel.test.jsx
```

Expected: fails because the panel still uses switch-log-derived rankings.

- [ ] **Step 3: Replace approximate rankings**

Convert `MetricsPanel` to a block function, call the new hook, and replace only the existing “Top 失败模型/渠道” section. Keep switch counts and the separate switch-log panel behavior unchanged. Render fixed safe DTO fields as text; do not use `dangerouslySetInnerHTML`.

- [ ] **Step 4: Add compact responsive styles**

Add `.fallback-attempt-*` classes using the existing CSS variables. Desktop uses up to three diagnostic columns; at the existing mobile breakpoint use one column, `min-width: 0`, `overflow-wrap: anywhere`, and no fixed widths.

- [ ] **Step 5: Run GREEN and focused frontend tests**

```powershell
npm test -- src/pages/Fallback/hooks/useAttemptObservability.test.jsx src/pages/Fallback/panels/MetricsPanel.test.jsx src/pages/Fallback/Fallback.test.jsx
npm run lint
```

Expected: all focused tests pass and ESLint reports zero warnings.

- [ ] **Step 6: Commit**

```powershell
git add web/default/src/pages/Fallback/panels/MetricsPanel.js web/default/src/pages/Fallback/panels/MetricsPanel.test.jsx web/default/src/pages/Fallback/Fallback.css
git commit -m "feat(ui): show precise attempt diagnostics"
```

---

### Task 6: Browser and Mobile Regression Coverage

**Files:**
- Modify: `web/default/e2e/smoke.spec.js`

**Interfaces:**
- Consumes: authenticated `/fallback/status` and the attempt-observability API.
- Produces: desktop and Mobile Chrome acceptance for the precise diagnostics panel.

- [ ] **Step 1: Add the failing Playwright assertion**

Extend the existing fallback status test to open “运行数据”, verify `精准失败诊断` and `最近请求链路`, and assert:

```js
expect(
  await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)
).toBe(true);
```

Use deterministic seeded attempt events or route interception; never depend on live upstream capacity for E2E.

- [ ] **Step 2: Run the focused E2E test and verify RED if UI wiring is incomplete**

```powershell
cd web/default
npx playwright test e2e/smoke.spec.js --grep "fallback status"
```

- [ ] **Step 3: Make only fixture/selector corrections required by the test**

Do not weaken text, mobile overflow, or authentication assertions.

- [ ] **Step 4: Run GREEN and commit**

```powershell
npx playwright test e2e/smoke.spec.js --grep "fallback status"
git add web/default/e2e/smoke.spec.js
git commit -m "test(ui): cover attempt diagnostics on mobile"
```

---

### Task 7: Full Verification, Production Soak, Review, and PR

**Files:**
- Create when credentials are available: `docs/evidence/attempt-observability-soak-2026-07-17.json`
- Modify only if evidence is produced: `AGENTS.md`

**Interfaces:**
- Consumes: final branch, intended production credential, local `3008` runtime.
- Produces: fresh validation evidence, independent review, PR, green CI, and merge readiness.

- [ ] **Step 1: Run complete backend gates**

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./controller ./router -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback ./controller ./router -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./... -count=1
& 'D:\ct\tools\go\bin\go.exe' vet ./...
& 'D:\ct\tools\go\bin\go.exe' build ./...
```

- [ ] **Step 2: Run complete frontend gates**

```powershell
cd web/default
npm run lint
npm test
npm run build
$env:STORYBOOK_DISABLE_TELEMETRY='1'; npm run build-storybook
npm run test:e2e
```

- [ ] **Step 3: Run paced soak only with intended production capacity**

Use `scripts/soak-test.py` with a token supplied through environment variables, 30 to 60 minutes of paced traffic, and Chat Completions, Responses, stream, and tools coverage. Verify aggregate counts against persisted events and verify recent chains show model rotation, provider fallback, and terminal success. If no intended credential is available, record the soak as externally blocked and do not substitute anonymous capacity.

- [ ] **Step 4: Scan evidence and changed files for sensitive content**

Search for `Bearer`, `sk-`, `password`, `content_preview`, raw upstream bodies, admin credentials, and long error text. Evidence must contain only safe identifiers, counts, timestamps, categories, status codes, and latency.

- [ ] **Step 5: Request independent code review**

Review against `docs/superpowers/specs/2026-07-17-attempt-observability-design.md` and this plan. Fix every Critical or Important finding with a failing regression test before proceeding.

- [ ] **Step 6: Push and create PR**

```powershell
git diff --check
git status --short
git push -u origin codex/attempt-observability
```

Create a non-draft PR summarizing architecture, security boundaries, test evidence, and any explicit production-soak blocker. Wait for all CI checks before merge.
