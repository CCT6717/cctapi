# Provider Catalog And Model Capability Refresh Design

## Goal

Replace static free-provider model assumptions with a scheduled, validated
catalog refresh. The first concrete targets are OVH model churn and Kilo's
per-model tools, JSON, vision, and context metadata.

## Scope

- Keep the existing six-hour model refresh lifecycle and manual free-pool sync.
- Fetch a typed provider catalog instead of only a string slice.
- Validate the complete result before applying any model or capability change.
- Keep the previous channel inventory and deployment configuration on failure.
- Track the last attempt, last success, stale state, source, model count, and
  last error for each provider.
- Apply model-level capability metadata to the generated deployment's selected
  real model, with conservative provider defaults when metadata is unavailable.
- Expose refresh state and model capability metadata through the gateway v2 API
  and the Chinese free-pool admin UI.

## Non-goals

- Persisting upstream catalogs in `fallback.json`.
- Adding a new database table or a general provider plugin protocol.
- Probing every model with paid completion requests.
- Claiming unsupported capabilities when an upstream catalog does not publish
  them.

## Options Considered

### 1. Persist catalogs in `fallback.json`

This survives restarts, but mixes operator configuration with volatile upstream
data and causes frequent file churn. It also makes manual edits and merge-back
work harder.

### 2. Add a small catalog database table

This is the selected approach after inspecting restart behavior. The existing
channel row preserves model IDs, but it cannot preserve Kilo's model-level
capabilities, and startup resource sync can otherwise replace a dynamic catalog
with a placeholder before the first refresh. One snapshot row per automatic
deployment keeps the last successful typed catalog without modifying
`fallback.json`.

### 3. In-memory snapshot only

This has the smallest implementation, but loses model-level metadata across a
restart exactly when the upstream catalog is unavailable. It is retained only
as a read cache over the persisted snapshots.

## Data Model

`FreeModelCapabilities` contains model ID plus stream, tools, JSON, vision, and
context length. Capability booleans use pointers while parsing so unknown values
can inherit conservative provider defaults instead of being confused with an
explicit false value.

`FreeProviderCatalogSnapshot` contains:

- provider and source
- normalized model entries
- last attempt and last success timestamps
- stale flag and last error

Snapshots are persisted as JSON in a small table keyed by automatic deployment
ID. A package-level read/write-locked cache is loaded during fallback state
initialization and returned as deep copies.

A dynamic catalog is stale when its latest attempt failed or when the last
success is older than twelve hours (twice the six-hour schedule). Static,
disabled, and operator-overridden model lists are not marked stale.

## Fetch And Validation

- `openai_models`: parse IDs from `/models`; capability fields remain unknown.
- `openrouter_free`: retain the current `:free` filtering.
- `kilo_free`: parse `isFree`, `supported_parameters`, architecture input
  modalities, and context length.
- `static`: convert registry defaults to catalog entries.
- OVH moves from static mode to `openai_models` while keeping its current static
  list as the startup/failure fallback.

Validation rejects an empty catalog, empty/control-character model IDs,
duplicate IDs after normalization, and unreasonable catalog size. Models are
sorted for deterministic channel updates.

## Apply Semantics

For each enabled provider/key:

1. Fetch and fully validate a candidate catalog.
2. Select the routing model using the existing override rules.
3. Resolve the selected model's capabilities over provider defaults.
4. Under the config write lock, update the persisted snapshot, channel models,
   and abilities in one database transaction.
5. After that transaction succeeds, update real model and capabilities together
   before releasing the config lock.
6. Publish the successful cache snapshot and clear only catalog-sync diagnostics.

Any failure before step 5 leaves the previous deployment and snapshot intact.
Failures mark the provider catalog stale and record a runtime error without
destroying unrelated health information.

## API And UI

Each gateway v2 free-provider object gains read-only `catalog_status` and
`model_capabilities`. Secret keys remain excluded. The provider row shows a
compact Chinese status for current, stale, failed, or not-yet-refreshed state,
including model count and last successful time. The existing page-level sync
button remains the manual refresh action. Manual sync returns attempted,
succeeded, and failed provider counts so a partial refresh is shown as a warning
instead of a false success.

## Testing

- Parser tests for Kilo capability metadata and generic OpenAI catalogs.
- Validation tests for empty, duplicate, malformed, and oversized catalogs.
- Red/green sync tests proving failure preserves channel/deployment/snapshot.
- Capability resolution tests proving Kilo model metadata overrides provider
  defaults while OVH inherits conservative defaults.
- Gateway projection tests proving status/capabilities are exposed without keys.
- Frontend utility/component tests for current, stale, failed, and unknown state.
- Full Go, frontend, Storybook, Playwright, build, live localhost, and CI checks.
