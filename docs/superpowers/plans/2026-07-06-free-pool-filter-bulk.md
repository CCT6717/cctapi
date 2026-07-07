# Free Pool Provider Filtering And Bulk Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add low-risk filtering, selection, and bulk enable/disable controls to the Free Pool provider table.

**Architecture:** Keep the current Semantic UI table. Add pure helper functions in `freePoolUtils.js`, keep state and actions in `FreeProvidersEditor.js`, and pass selection props into `FreeProviderRow.js`. Persist behavior remains unchanged: bulk actions only stage config changes until the existing save button is clicked.

**Tech Stack:** React 18, CRA/Jest, Semantic UI React, existing Free Pool helpers.

## Global Constraints

- Do not add backend APIs.
- Do not bulk clear or bulk replace keys.
- Do not expose stored raw keys.
- Do not migrate this table to Ant Design in this slice.
- Do not commit generated `web/build/default` artifacts.
- Use TDD: tests first, verify failing, then implement.

---

### Task 1: Provider Filter And Bulk Helper Functions

**Files:**
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.js`
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.test.js`

**Interfaces:**
- Produces: `matchesFreeProviderFilters(provider, filters) => boolean`
- Produces: `filterFreeProviderRows(rows, filters) => Array`
- Produces: `buildBulkEnabledProviderConfig(freeProviders, providers, enabled) => Object`
- Produces: `isFreeProviderReady(provider) => boolean`
- Produces: `freeProviderNeedsKey(provider) => boolean`

- [ ] **Step 1: Add failing helper tests**

Add tests showing text search, ready filtering, needs-key filtering, capability filtering, and bulk enabled-state updates.

- [ ] **Step 2: Run helper tests and verify RED**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false src/components/fallback-gateway/freePoolUtils.test.js
```

Expected: fails because the helper exports do not exist.

- [ ] **Step 3: Implement helpers**

Add focused pure helper functions to `freePoolUtils.js`. The helpers must not mutate the original config object and must preserve existing keys and limits.

- [ ] **Step 4: Run helper tests and verify GREEN**

Run the same test command. Expected: target test file passes.

### Task 2: Provider Table Controls And Selection

**Files:**
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProviderRow.js`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.test.js`

**Interfaces:**
- Consumes: helper functions from Task 1.
- Produces: search/status/capability controls, visible row count, select visible, clear selection, bulk enable, and bulk disable.

- [ ] **Step 1: Add failing component test**

Add a test that renders two providers, searches for one provider, selects visible providers, clicks bulk disable, and verifies only the visible provider is staged as disabled.

- [ ] **Step 2: Run component test and verify RED**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false src/components/fallback-gateway/FreeModelPool.test.js
```

Expected: fails because the filter and bulk controls do not exist.

- [ ] **Step 3: Implement editor state and controls**

Add `searchText`, `statusFilter`, `capabilityFilter`, `selectedProviders`, and `bulkMessage` state in `FreeProvidersEditor.js`. Filter rows before rendering. Prune selection when visible rows change. Bulk enable/disable selected rows by calling `onChange(buildBulkEnabledProviderConfig(...))`.

- [ ] **Step 4: Add row selection checkbox**

Update `FreeProviderRow.js` to render a selection checkbox before the existing enabled toggle when selection props are provided.

- [ ] **Step 5: Run component test and verify GREEN**

Run the same component test command. Expected: target test file passes.

### Task 3: Styling, Full Verification, Commit, Preview

**Files:**
- Modify: `web/default/src/pages/Fallback/Fallback.css`

**Interfaces:**
- Consumes: CSS classes emitted by Task 2.
- Produces: compact provider operations toolbar and staged message styling.

- [ ] **Step 1: Add compact styles**

Add styles for `.free-provider-ops`, `.free-provider-filter-row`, `.free-provider-bulk-row`, `.free-provider-count`, and `.free-provider-bulk-message`.

- [ ] **Step 2: Run full frontend tests**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false
```

Expected: all test suites pass.

- [ ] **Step 3: Run production build**

Run:

```powershell
npm run build
```

Expected: exits 0. Existing unrelated ESLint warnings may remain.

- [ ] **Step 4: Restart local preview**

Run:

```powershell
Set-Location D:\ct\project
go build -o one-api.exe .
```

Restart `one-api.exe --port 3008` and verify `http://127.0.0.1:3008/` returns HTTP 200.

- [ ] **Step 5: Commit and push source changes only**

Do not commit generated `web/build/default` artifacts. Commit the docs, helper tests, helper code, UI code, and CSS.
