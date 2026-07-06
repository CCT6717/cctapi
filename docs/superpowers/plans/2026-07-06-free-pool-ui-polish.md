# Free Pool UI Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the Fallback / Free Pool admin UI in three focused passes: provider operation ergonomics, shared visual consistency, and clearer readiness guidance.

**Architecture:** Keep current behavior and API calls unchanged. Improve the existing React components and CSS in place, using the established `fallback-*` and `free-*` class naming, Semantic UI table controls, and the existing AntD wrapper layer only where it already exists. Do not rewrite the page or migrate the provider table to AntD in this slice.

**Tech Stack:** React 18, CRA/Jest, Semantic UI React, existing AntD wrapper primitives, Storybook, Go embedded frontend build.

## Global Constraints

- Do not change backend APIs.
- Do not change free-pool save, sync, reload, cleanup, runtime, or usage data behavior.
- Do not expose stored raw keys.
- Do not commit generated `web/build/default` artifacts.
- Keep local preview available on port `3008`; restart only after frontend build succeeds.
- Keep the UI operational and compact: no marketing hero, no large decorative cards, no new color-heavy palette.
- Use Chinese user-facing copy on `/fallback/free-pool`, while preserving provider brand names and technical abbreviations such as RPM, RPD, TPM, TPD, JSON, API key, and token.

---

### Task 1: Free Pool Provider Operation Polish

**Files:**
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProviderRow.js`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.test.js`
- Modify: `web/default/src/pages/Fallback/Fallback.css`

**Interfaces:**
- Consumes: existing `filterFreeProviderRows`, `buildBulkEnabledProviderConfig`, row selection props, and usage rows.
- Produces: a sticky provider operation bar, clearer selected/visible counts, compact row status classes, and warning styling for invalid or key-blocked provider rows.

- [ ] **Step 1: Add component expectations**

Update `web/default/src/components/fallback-gateway/FreeModelPool.test.js` to assert visible operation controls still exist after styling class changes:

```javascript
expect(screen.getByLabelText('搜索免费供应商')).toBeInTheDocument();
expect(screen.getByText('选择可见项')).toBeInTheDocument();
expect(screen.getByText('批量启用')).toBeInTheDocument();
expect(screen.getByText('批量停用')).toBeInTheDocument();
```

- [ ] **Step 2: Add row state classes**

In `FreeProviderRow.js`, compute a compact row class name:

```javascript
const rowClasses = [
  'free-provider-row',
  provider.enabled ? 'is-enabled' : 'is-disabled',
  invalidLimits ? 'has-invalid-limits' : '',
  provider.requires_key && !provider.keyless && keyCount === 0 && stagedKeys.length === 0
    ? 'needs-key'
    : '',
].filter(Boolean).join(' ');
```

Use it on the existing row:

```jsx
<Table.Row key={key} className={rowClasses} warning={invalidLimits}>
```

- [ ] **Step 3: Improve operation bar markup**

In `FreeProvidersEditor.js`, keep the current state logic but wrap the count and selected count with machine-readable classes:

```jsx
<span className='free-provider-count'>
  <strong>{visibleProviderRows.length}</strong> / {providerRows.length} 个供应商
</span>
```

```jsx
<Label basic size='mini' className='free-provider-selected-count'>
  已选择 {selectedProviders.length} 项
</Label>
```

- [ ] **Step 4: Add compact sticky styles**

In `Fallback.css`, update the existing `.free-provider-*` block so `.free-provider-ops` is sticky inside the page, filter controls align predictably, and row state classes have subtle left borders:

```css
.free-provider-ops {
  position: sticky;
  top: 0;
  z-index: 3;
  display: grid;
  gap: 10px;
  margin-bottom: 12px;
  padding: 10px;
  border: 1px solid var(--fb-border);
  border-radius: var(--fb-radius-sm);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: var(--fb-shadow-sm);
}

.free-provider-row.needs-key td:first-child,
.free-provider-row.has-invalid-limits td:first-child {
  box-shadow: inset 3px 0 0 var(--fb-warning);
}

.free-provider-row.is-disabled {
  color: var(--fb-text-muted);
}
```

- [ ] **Step 5: Verify Task 1**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false src/components/fallback-gateway/FreeModelPool.test.js
```

Expected: target test suite passes.

### Task 2: Fallback Visual Consistency Polish

**Files:**
- Modify: `web/default/src/pages/Fallback/Fallback.css`
- Modify: `web/default/src/ui/ui.css`

**Interfaces:**
- Consumes: existing `--fb-*` CSS tokens and existing `cct-*` UI wrapper classes.
- Produces: consistent compact table, section, message, label, button, and input treatment across fallback panels.

- [ ] **Step 1: Normalize section surfaces**

Update `.fallback-virtual-panel`, `.fallback-subsection`, and existing `.fallback-content-panel` descendants to share the same border, radius, shadow, and spacing scale:

```css
.fallback-virtual-panel,
.fallback-subsection {
  border: 1px solid var(--fb-border);
  border-radius: var(--fb-radius-sm);
  background: var(--fb-surface);
  box-shadow: var(--fb-shadow-sm);
}
```

- [ ] **Step 2: Normalize form controls**

Add fallback-scoped form control rules:

```css
.fallback-page .ui.input > input,
.fallback-page .ui.form textarea,
.fallback-page select {
  border-color: var(--fb-border) !important;
  border-radius: var(--fb-radius-sm) !important;
  color: var(--fb-text) !important;
}
```

- [ ] **Step 3: Normalize dense table behavior**

Add table layout rules that keep long provider names, hashes, and model names readable without causing horizontal layout jumps:

```css
.fallback-page .ui.table td,
.fallback-page .ui.table th {
  vertical-align: top !important;
}

.fallback-page .ui.table td {
  overflow-wrap: anywhere;
}
```

- [ ] **Step 4: Keep AntD wrappers visually aligned**

Update `web/default/src/ui/ui.css` only for shared wrapper token alignment, not business-page behavior:

```css
.cct-admin-card {
  border-radius: 8px;
}
```

- [ ] **Step 5: Verify Task 2**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false src/pages/Fallback/Fallback.test.js
```

Expected: target test suite passes.

### Task 3: Information Hierarchy And Guidance Polish

**Files:**
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.js`
- Modify: `web/default/src/components/fallback-gateway/FreePoolWorkflowDashboard.js`
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.js`
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.test.js`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.test.js`
- Modify: `web/default/src/pages/Fallback/Fallback.css`

**Interfaces:**
- Consumes: existing `buildFreePoolWorkflowSummary`.
- Produces: clearer readiness summary, stronger next-action area, and one compact status strip above the detailed provider/deployment tables.

- [ ] **Step 1: Add summary status fields**

Extend `buildFreePoolWorkflowSummary` in `freePoolUtils.js` with:

```javascript
statusTone: riskCount > 0 ? 'warning' : readinessScore === readinessTotal ? 'success' : 'info',
statusText: riskCount > 0
  ? `还有 ${riskCount} 项需要处理`
  : readinessScore === readinessTotal
    ? '免费池已就绪'
    : '免费池接入中',
```

- [ ] **Step 2: Add tests for status fields**

Update `freePoolUtils.test.js` to assert:

```javascript
expect(summary.statusTone).toBe('warning');
expect(summary.statusText).toContain('需要处理');
```

and for a ready state:

```javascript
expect(summary.statusTone).toBe('success');
expect(summary.statusText).toBe('免费池已就绪');
```

- [ ] **Step 3: Render status strip**

In `FreePoolWorkflowDashboard.js`, render a compact status strip near the existing readiness meter:

```jsx
<div className={`free-pool-status-strip ${summary.statusTone}`}>
  <strong>{summary.statusText}</strong>
  <span>{summary.nextActions.length > 0 ? summary.nextActions[0] : '可以进入小流量验证。'}</span>
</div>
```

- [ ] **Step 4: Style guidance hierarchy**

In `Fallback.css`, add:

```css
.free-pool-status-strip {
  display: grid;
  gap: 3px;
  padding: 10px 12px;
  border-radius: var(--fb-radius-sm);
  border: 1px solid var(--fb-border);
  background: var(--fb-bg);
}

.free-pool-status-strip.success {
  border-color: var(--fb-success-border);
  background: var(--fb-success-bg);
}

.free-pool-status-strip.warning {
  border-color: var(--fb-warning-border);
  background: var(--fb-warning-bg);
}
```

- [ ] **Step 5: Verify Task 3**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false src/components/fallback-gateway/freePoolUtils.test.js src/components/fallback-gateway/FreeModelPool.test.js
```

Expected: target test suites pass.

### Task 4: Full Verification, Preview, Commit

**Files:**
- Source files modified by Tasks 1-3.

**Interfaces:**
- Produces: verified frontend, rebuilt local preview, clean source-only commit.

- [ ] **Step 1: Run full frontend tests**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false
```

Expected: all test suites pass.

- [ ] **Step 2: Run production build**

Run:

```powershell
npm run build
```

Expected: exits 0. Existing unrelated ESLint warnings may remain.

- [ ] **Step 3: Run Storybook build**

Run:

```powershell
$env:CI='true'; $env:STORYBOOK_DISABLE_TELEMETRY='1'; npm run build-storybook
```

Expected: exits 0. Existing Storybook asset-size warnings may remain.

- [ ] **Step 4: Restart local preview**

Run:

```powershell
Set-Location D:\ct\project
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
$env:GOMAXPROCS='1'
go build -o one-api.exe .
```

Restart `one-api.exe --port 3008` and verify `http://127.0.0.1:3008/` returns HTTP 200.

- [ ] **Step 5: Commit and push source changes only**

Do not commit generated `web/build/default` artifacts. Commit the plan and source/test/CSS changes, then push `cleanup/structure-boundaries`.
