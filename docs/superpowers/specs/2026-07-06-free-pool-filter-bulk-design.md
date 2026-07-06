# Free Pool Provider Filtering And Bulk Toggle Design

## Goal

Improve the Free Pool provider table so administrators can quickly find providers and stage low-risk bulk enable or disable changes without changing backend APIs or exposing stored API keys.

## Scope

This slice affects only the Free Pool provider management table in the Fallback area.

Included:
- Text search across provider key, display title, capability flags, and model fetch mode.
- Status filters for all, enabled, disabled, ready, needs key, configured, and catalog rows.
- Capability filters for keyless, stream, tools, JSON, and vision support.
- Row selection on the currently visible provider rows.
- Bulk enable and bulk disable for selected provider rows.
- Clear visible selection and select all visible rows.

Excluded:
- Bulk clearing stored keys.
- Bulk replacing keys.
- New backend endpoints.
- Migrating this table to Ant Design.
- Changing save semantics.

## User Experience

The provider section gains a compact operations toolbar above the table. The toolbar contains a search input, status filter, capability filter, result count, and selection actions. Each provider row gets a checkbox next to its existing enabled toggle.

Bulk enable and disable only update the in-memory gateway config. The existing "save config" button remains the only operation that persists the staged changes. After a bulk action, the table displays a small staged-change note so the administrator knows the changes are pending save.

## Architecture

Filtering and bulk mutation are pure helpers in `freePoolUtils.js`. `FreeProvidersEditor.js` owns search/filter/selection state and calls the pure helpers. `FreeProviderRow.js` remains a row renderer and receives selection props. `FreeModelPool.js` remains unchanged except for consuming the improved editor through the existing props.

## Data Rules

- A ready provider is enabled and either keyless or has at least one stored or staged key.
- A needs-key provider requires a key, is not keyless, and has no key count.
- Bulk enable and disable create or update provider config entries by provider key.
- Selection applies only to provider names currently visible after filters.
- When filters change, selection is pruned to visible provider names.

## Testing

Unit tests cover:
- Text filtering by provider name and capability.
- Status filtering for ready and needs-key rows.
- Capability filtering.
- Bulk enabled-state updates without touching keys.

Component tests cover:
- Search narrowing visible providers.
- Select visible providers and stage bulk disable.
- Staged bulk action text appears before save.

## Success Criteria

- Existing Free Pool load and save behavior remains unchanged.
- No raw key values are displayed.
- Existing stored keys are not cleared by bulk actions.
- `npm test -- --watchAll=false` passes.
- `npm run build` exits 0.
- Local preview on `http://127.0.0.1:3008/` returns HTTP 200 after rebuild.
