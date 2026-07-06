# Technical Debt Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce low-risk technical debt after the FreeLLMAPI Route A merge prep without changing runtime behavior.

**Architecture:** Keep this pass narrow. Add characterization tests for existing usage filtering, remove obvious Fallback UI warning sources, then verify and rebuild the local 3008 preview.

**Tech Stack:** Go 1.22 tests, React CRA build, PowerShell on Windows.

## Global Constraints

- Keep the local preview available on port `3008`; restart only at the end.
- Do not commit generated `web/build/default` artifacts.
- Do not expand FreeLLMAPI scope in this cleanup pass.
- Preserve existing `cct/free`, `cct/high`, and `cct/low` behavior.

---

### Task 1: Usage API Characterization Coverage

**Files:**
- Modify: `fallback/free_provider_ledger_test.go`
- Modify: `router/fallback_usage_test.go`

**Interfaces:**
- Consumes: `fallback.ListFreeProviderUsage(filter FreeProviderUsageFilter)`
- Consumes: `GET /api/fallback/free-pool/usage`
- Produces: stronger regression coverage for `provider`, `key_hash`, `model`, combined filters, and raw-key redaction.

- [ ] **Step 1: Add ledger filter tests**

Add tests that seed multiple providers, keys, and models, then assert `key_hash`, `model`, and combined filters return only the expected row.

- [ ] **Step 2: Add usage API query tests**

Add router tests that call `/api/fallback/free-pool/usage?provider=...&key_hash=...&model=...` and assert the returned row identity.

- [ ] **Step 3: Add provider-neutral raw-key sentinel**

Use a sentinel string that is not provider-prefix-specific and assert it is absent from serialized usage responses.

- [ ] **Step 4: Verify**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
$env:GOMAXPROCS='1'
go test ./fallback ./router -count=1
```

Expected: PASS.

### Task 2: Fallback Frontend Warning Cleanup

**Files:**
- Modify: `web/default/src/pages/Fallback/hooks/useFallbackPage.js`

**Interfaces:**
- Consumes: `../utils/fallbackHelpers`
- Consumes: `../utils/scoreUtils`
- Produces: same hook API with no unused Fallback score helper imports.

- [ ] **Step 1: Remove dead imports**

Remove the unused `sortScoreItems` import from `fallbackHelpers` and the unused `clampScore` import from `scoreUtils`. Keep `sortScoreItems as sortScoreItemsFn`.

- [ ] **Step 2: Verify Fallback tests**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false --runTestsByPath src/pages/Fallback/index.test.js src/components/fallback-gateway/freePoolUtils.test.js src/components/fallback-gateway/freeProviderDisplay.test.js src/components/fallback-gateway/FreeModelPool.test.js
```

Expected: PASS.

### Task 3: Final Verification and Preview Refresh

**Files:**
- No source file changes expected beyond Tasks 1-2.

**Interfaces:**
- Consumes: committed source changes.
- Produces: refreshed local preview at `http://127.0.0.1:3008/`.

- [ ] **Step 1: Run focused backend verification**

Run:

```powershell
go test ./fallback ./router ./controller ./relay/model -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend build**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm run build
```

Expected: build exits 0. Existing unrelated warnings may remain.

- [ ] **Step 3: Commit source/test changes only**

Run:

```powershell
git add fallback/free_provider_ledger_test.go router/fallback_usage_test.go web/default/src/pages/Fallback/hooks/useFallbackPage.js docs/superpowers/plans/2026-07-06-technical-debt-cleanup.md
git commit -m "test: cover free provider usage filters"
```

Expected: commit succeeds without generated build artifacts.

- [ ] **Step 4: Rebuild backend and restart preview**

Run:

```powershell
go build -o one-api.exe .
```

Then restart `one-api.exe --port 3008` and verify `http://127.0.0.1:3008/` returns HTTP 200.
