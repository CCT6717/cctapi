# cctapi Structure Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the highest-risk structure debt in cctapi without changing user-visible behavior.

**Architecture:** Start with a small security parity fix across frontend themes, then isolate fallback responsibilities behind focused service/helper files. Keep public API behavior and config JSON shape stable. Avoid broad rewrites of the inherited One API codebase.

**Tech Stack:** Go 1.22+, Gin, GORM, SQLite, React themes under `web/default`, `web/air`, and `web/berry`.

## Global Constraints

- Keep the service runnable on Windows from `D:\ct\project`.
- Do not move or rename existing public routes.
- Do not change `data/fallback.json` schema unless a compatibility adapter is added in the same task.
- Do not commit `.env`, `one-api.exe`, `one-api.db`, logs, or `D:\ct\tools`.
- Each task ends with `go test ./...` or the narrowest equivalent plus any affected frontend build/test command.
- Prefer small commits so the work can be copied back to the original computer safely.

---

## File Structure

- `web/default/src/helpers/sanitize.js`: shared sanitizer for default theme HTML rendering.
- `web/berry/src/utils/sanitize.js`: shared sanitizer for berry theme HTML rendering.
- `fallback/free_provider_registry.go`: provider metadata and provider name validation.
- `fallback/free_provider_sync.go`: channel/deployment sync and stale cleanup.
- `fallback/free_provider_fetch.go`: upstream model and credit HTTP fetchers.
- `fallback/free_provider_scheduler.go`: background free-provider sync lifecycle.
- `fallback/orchestrator.go`: pure fallback attempt planning and state transitions.
- `controller/relay.go`: keep Gin request/response handling; call fallback orchestration helpers.
- `router/fallback_config_service.go`: editor config normalization, backup, channel upsert, and save/reload workflow.
- `router/fallback_config.go`: keep HTTP handlers only.

---

### Task 1: Create Safety Baseline

**Files:**
- Modify: none

**Interfaces:**
- Consumes: current checkout at `923219c756ca7db47f1f35a4c6156db078e9d33d`.
- Produces: clean branch and baseline verification output.

- [ ] **Step 1: Check working tree**

Run:

```powershell
git status --short --branch
```

Expected: only known local changes, currently `scripts/start-cctapi.ps1`.

- [ ] **Step 2: Create a working branch**

Run:

```powershell
git switch -c cleanup/structure-boundaries
```

Expected: branch switches to `cleanup/structure-boundaries`.

- [ ] **Step 3: Run backend baseline tests**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 4: Commit the existing startup script fix separately**

Run:

```powershell
git add scripts/start-cctapi.ps1
git commit -m "chore: make local start script wait for server readiness"
```

Expected: one commit containing only `scripts/start-cctapi.ps1`.

---

### Task 2: Sanitize HTML Rendering Across Frontend Themes

**Files:**
- Modify: `web/default/package.json`
- Modify: `web/berry/package.json`
- Modify: `web/default/src/pages/Home/index.js`
- Modify: `web/default/src/pages/About/index.js`
- Modify: `web/default/src/components/Footer.js`
- Modify: `web/berry/src/views/Home/index.js`
- Modify: `web/berry/src/views/About/index.js`
- Modify: `web/berry/src/ui-component/Footer.js`
- Create: `web/default/src/helpers/sanitize.js`
- Create: `web/berry/src/utils/sanitize.js`

**Interfaces:**
- Consumes: Markdown/HTML strings from `/api/home_page_content`, `/api/about`, and footer status data.
- Produces: `sanitizeHtml(html: string): string` helper in each affected theme.

- [ ] **Step 1: Add `dompurify` to default and berry**

Run:

```powershell
cd D:\ct\project\web\default
npm install dompurify@^3.4.11
cd D:\ct\project\web\berry
npm install dompurify@^3.4.11
```

Expected: `package.json` and lock files update for both themes.

- [ ] **Step 2: Add sanitizer helpers**

Create `web/default/src/helpers/sanitize.js`:

```javascript
import DOMPurify from 'dompurify';

export const sanitizeHtml = (html) => DOMPurify.sanitize(html || '');
```

Create `web/berry/src/utils/sanitize.js`:

```javascript
import DOMPurify from 'dompurify';

export const sanitizeHtml = (html) => DOMPurify.sanitize(html || '');
```

- [ ] **Step 3: Sanitize Markdown parse results**

Replace `marked.parse(data)` usages in default and berry home/about pages with `sanitizeHtml(marked.parse(data))`.

- [ ] **Step 4: Sanitize all direct HTML injection sites touched by this task**

Wrap `dangerouslySetInnerHTML` values in default and berry home/about/footer components with `sanitizeHtml(...)`.

- [ ] **Step 5: Build affected themes**

Run:

```powershell
cd D:\ct\project\web\default
npm run build
cd D:\ct\project\web\berry
npm run build
```

Expected: both builds complete successfully.

- [ ] **Step 6: Run backend tests**

Run:

```powershell
cd D:\ct\project
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 7: Commit**

Run:

```powershell
git add web/default web/berry
git commit -m "fix: sanitize configurable html in frontend themes"
```

---

### Task 3: Split Free Provider Code by Responsibility

**Files:**
- Modify: `fallback/free_pool.go`
- Create: `fallback/free_provider_registry.go`
- Create: `fallback/free_provider_sync.go`
- Create: `fallback/free_provider_fetch.go`
- Create: `fallback/free_provider_scheduler.go`
- Modify: `fallback/free_pool_test.go`

**Interfaces:**
- Consumes: existing exported functions `SyncFreePool`, `StartFreeSync`, `ValidateFreeProviderName`, `ApplyLimitsOverride`, `ValidateFreeProviderLimits`, `SafeKeyHash`, `IsAutoDeploymentID`, `IsAutoChannelName`.
- Produces: same exported functions with the same signatures, redistributed across focused files.

- [ ] **Step 1: Move provider metadata only**

Move `FreeProviderMeta`, `BuiltinFreeProviders`, `channelName`, `deploymentID`, `SafeKeyHash`, provider validation, and limit validation into `fallback/free_provider_registry.go`.

- [ ] **Step 2: Move DB sync and cleanup only**

Move `SyncFreePool`, `computeExpectedAutoResources`, `DryRunCleanStale`, and stale cleanup structs into `fallback/free_provider_sync.go`.

- [ ] **Step 3: Move upstream HTTP fetchers only**

Move `fetchModels`, `fetchOpenRouterModels`, `parseFreeModels`, `fetchKiloModels`, `parseKiloFreeModels`, `fetchOpenAICompatModels`, `parseOpenAICompatModels`, `queryOpenRouterCredits`, `parseCreditsBalance`, and `syncOpenRouterCredits` into `fallback/free_provider_fetch.go`.

- [ ] **Step 4: Move scheduler only**

Move `StartFreeSync`, `runFreeSyncModels`, `runFreeSyncCredits`, and scheduler globals into `fallback/free_provider_scheduler.go`.

- [ ] **Step 5: Keep compatibility**

Leave `fallback/free_pool.go` with either no code or only compatibility comments. Do not change exported names used by `main.go`, `router`, or tests.

- [ ] **Step 6: Run focused tests**

Run:

```powershell
go test ./fallback -run "Free|Provider|Sync|Credits|Models|Stale" -count=1
```

Expected: fallback package tests pass.

- [ ] **Step 7: Run full backend tests**

Run:

```powershell
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 8: Commit**

Run:

```powershell
git add fallback
git commit -m "refactor: split free provider fallback responsibilities"
```

---

### Task 4: Extract Fallback Relay Orchestration from Gin Controller

**Files:**
- Modify: `controller/relay.go`
- Create: `fallback/orchestrator.go`
- Create: `fallback/orchestrator_test.go`

**Interfaces:**
- Consumes: virtual model name, request capabilities, deployment state, channel lookup results.
- Produces: pure helper functions for ordering/filtering deployments without requiring `*gin.Context`.

- [ ] **Step 1: Extract deployment preparation**

Create `fallback.PrepareDeploymentsForRequest(virtualModel string, caps RequestCapabilities) ([]DeploymentConfig, error)` in `fallback/orchestrator.go`. It should combine virtual model lookup, sticky routing, capability filtering, health filtering, strategy sorting, and preferred deployment promotion.

- [ ] **Step 2: Write tests for preparation ordering**

Add tests in `fallback/orchestrator_test.go` covering:

```go
func TestPrepareDeploymentsPromotesPreferredDeployment(t *testing.T) {}
func TestPrepareDeploymentsAppliesCapabilityFilter(t *testing.T) {}
func TestPrepareDeploymentsPreservesStickyWhenNoPreferred(t *testing.T) {}
```

- [ ] **Step 3: Replace duplicated logic in `relayWithFallback`**

In `controller/relay.go`, keep request body parsing and response writing there, but replace deployment filtering/sorting blocks with a call to `fallback.PrepareDeploymentsForRequest`.

- [ ] **Step 4: Run focused tests**

Run:

```powershell
go test ./fallback ./controller -count=1
```

Expected: fallback and controller tests pass.

- [ ] **Step 5: Run full backend tests**

Run:

```powershell
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 6: Commit**

Run:

```powershell
git add fallback controller
git commit -m "refactor: move fallback deployment planning out of relay controller"
```

---

### Task 5: Move Fallback Config Editor Workflow out of Router

**Files:**
- Modify: `router/fallback_config.go`
- Create: `router/fallback_config_service.go`
- Modify: `router/fallback_gateway_test.go`
- Modify: `router/fallback_test.go`

**Interfaces:**
- Consumes: `fallbackEditorConfig` payload and existing `fallback.Config`.
- Produces: `saveFallbackEditorConfig(payload fallbackEditorConfig) (fallbackEditorConfig, string, error)`.

- [ ] **Step 1: Create service function**

Move normalization, channel upsert, config construction, backup, file write, and reload from `updateFallbackEditorConfig` into:

```go
func saveFallbackEditorConfig(payload fallbackEditorConfig) (fallbackEditorConfig, string, error)
```

- [ ] **Step 2: Keep handler thin**

Change `updateFallbackEditorConfig` so it only reads JSON, calls `saveFallbackEditorConfig`, and writes the JSON response.

- [ ] **Step 3: Preserve existing response shape**

Ensure success response still includes:

```json
{
  "success": true,
  "message": "fallback config saved",
  "data": {},
  "backup_path": ""
}
```

Include `backup_path` only when a backup exists, matching current behavior.

- [ ] **Step 4: Run router tests**

Run:

```powershell
go test ./router -count=1
```

Expected: router package tests pass.

- [ ] **Step 5: Run full backend tests**

Run:

```powershell
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 6: Commit**

Run:

```powershell
git add router
git commit -m "refactor: isolate fallback editor save workflow"
```

---

### Task 6: Harden Config and State Boundaries

**Files:**
- Modify: `fallback/config.go`
- Modify: `fallback/config_test.go` or create it if missing
- Modify: `fallback/state.go`
- Modify: `fallback/state_test.go` if needed

**Interfaces:**
- Consumes: existing exported config readers.
- Produces: copy-returning config helpers for callers that should not mutate global state.

- [ ] **Step 1: Add copy helpers**

Add:

```go
func CloneConfig() *Config
func CloneDeployment(id string) (*DeploymentConfig, bool)
func CloneVirtualModel(modelName string) (*VirtualModelConfig, bool)
```

These should deep-copy maps and slices used by fallback config.

- [ ] **Step 2: Use copy helpers in router read paths**

Update config editor read paths to use cloned config data so router code cannot accidentally mutate global maps.

- [ ] **Step 3: Add mutation isolation tests**

Add tests proving that mutating a returned config/deployment/virtual model does not mutate the global config.

- [ ] **Step 4: Run fallback and router tests**

Run:

```powershell
go test ./fallback ./router -count=1
```

Expected: fallback and router tests pass.

- [ ] **Step 5: Run full backend tests**

Run:

```powershell
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 6: Commit**

Run:

```powershell
git add fallback router
git commit -m "refactor: return fallback config copies across boundaries"
```

---

### Task 7: Final Verification and Portable Package

**Files:**
- Modify: none unless verification finds failures.

**Interfaces:**
- Consumes: all previous commits.
- Produces: verified local working tree and portable archive guidance.

- [ ] **Step 1: Run backend test suite**

Run:

```powershell
go test ./...
```

Expected: all Go packages pass.

- [ ] **Step 2: Build Windows executable**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
go build -trimpath -ldflags "-s -w" -o one-api.exe .
```

Expected: `D:\ct\project\one-api.exe` is created.

- [ ] **Step 3: Smoke test local service**

Run:

```powershell
scripts\start-cctapi.ps1 -Port 3008 -ProjectRoot D:\ct\project -NoBrowser
Invoke-WebRequest -Uri http://localhost:3008 -UseBasicParsing
```

Expected: HTTP status `200` and page title `One API`.

- [ ] **Step 4: Check working tree**

Run:

```powershell
git status --short
```

Expected: only intended generated files, or clean if generated files are ignored/removed.

- [ ] **Step 5: Create copy-back archive without local secrets**

Archive `D:\ct\project` while excluding:

```text
.env
logs/
one-api.db
one-api.exe
D:\ct\tools
```

Expected: archive contains source, `.git`, docs, frontend packages, and all committed cleanup work.

