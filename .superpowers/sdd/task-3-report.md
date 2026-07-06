Status: DONE

Task: 3 - Information Hierarchy And Guidance Polish

Summary:
- Extended `buildFreePoolWorkflowSummary` to expose compact presentation fields for workflow state without changing the underlying readiness/risk/action logic.
- Added a compact guidance/status strip above the existing workflow metrics in the free pool dashboard.
- Added focused regression coverage for both summary-field generation and rendered guidance hierarchy.

Files changed:
- `web/default/src/components/fallback-gateway/freePoolUtils.js`
- `web/default/src/components/fallback-gateway/freePoolUtils.test.js`
- `web/default/src/components/fallback-gateway/FreePoolWorkflowDashboard.js`
- `web/default/src/components/fallback-gateway/FreeModelPool.test.js`
- `web/default/src/pages/Fallback/Fallback.css`

Implementation details:
1. Summary status fields
- Added `statusTone` to `buildFreePoolWorkflowSummary`.
  - `warning` when any risk exists
  - `success` when all readiness checks pass
  - `info` otherwise
- Added `statusText` to `buildFreePoolWorkflowSummary`.
  - `还有 X 项需要处理`
  - `免费池已就绪`
  - `免费池接入中`
- Kept existing workflow summary inputs, derived counts, risks, next actions, and backend/API interactions unchanged.

2. Dashboard guidance hierarchy
- Inserted a `free-pool-status-strip` directly below the readiness hero area and above the metric cards.
- The strip shows:
  - primary status via `summary.statusText`
  - secondary guidance via the first `nextActions` item
  - fallback copy `可以进入小流量验证。` when no next action exists

3. Styling
- Added compact grid styling for `.free-pool-status-strip`.
- Added tone-specific variants for:
  - `.free-pool-status-strip.success`
  - `.free-pool-status-strip.warning`
- Reused existing fallback theme tokens to keep the new strip visually consistent with the current page.

Test coverage:
- `freePoolUtils.test.js`
  - verifies warning tone/text when workflow has unresolved risks
  - verifies success tone/text when workflow is fully ready
- `FreeModelPool.test.js`
  - verifies the warning strip renders in the workflow dashboard
  - verifies the strip surfaces actionable guidance tied to the current next step

Verification:
- Ran:
  - `Set-Location D:\ct\project\web\default`
  - `npm test -- --watchAll=false src/components/fallback-gateway/freePoolUtils.test.js src/components/fallback-gateway/FreeModelPool.test.js`
- Result:
  - PASS `src/components/fallback-gateway/freePoolUtils.test.js`
  - PASS `src/components/fallback-gateway/FreeModelPool.test.js`
  - 2 suites passed, 20 tests passed

Constraints check:
- No backend API changes
- No data-loading or action behavior changes
- New user-facing copy kept in Chinese
- Provider brand names and technical abbreviations preserved
- No generated `web/build/default` artifacts were touched or committed

Concerns:
- None at implementation time. The new `info` tone is available in data now but is intentionally left without a dedicated visual variant because the brief only required `success` and `warning` styling.
