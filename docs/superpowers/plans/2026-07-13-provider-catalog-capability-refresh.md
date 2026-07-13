# Provider Catalog And Model Capability Refresh Implementation Plan

> **For Codex:** Execute each task with test-driven development and commit after
> its focused verification passes.

**Goal:** Refresh OVH/Kilo catalogs safely, apply model-level capabilities, and
show refresh health in the free-pool admin UI.

**Architecture:** Build and validate typed catalog candidates, then apply channel
and deployment changes before publishing a deep-copied runtime snapshot. Failed
refreshes preserve old routing data and only update stale/error diagnostics.

**Tech stack:** Go 1.22, GORM, Gin, React, Semantic UI React, Vitest, Playwright.

---

## Task 1: Typed Catalog Parsing And Validation

**Files:**
- Create: `fallback/free_provider_catalog_runtime.go`
- Modify: `fallback/free_provider_fetch.go`
- Test: `fallback/free_provider_catalog_runtime_test.go`
- Test: `fallback/free_pool_test.go`

1. Add failing tests for Kilo tools/JSON/vision/context parsing and catalog
   validation failures.
2. Run the focused tests and confirm they fail because typed catalog support is
   absent.
3. Add `FreeModelCapabilities`, candidate/snapshot types, deep-copy helpers,
   normalization, and validation.
4. Refactor fetch modes to return typed entries while retaining `fetchModels`
   as a compatibility wrapper.
5. Run focused tests to green and commit.

## Task 2: Atomic Refresh And Capability Application

**Files:**
- Modify: `fallback/config.go`
- Modify: `fallback/free_provider_sync.go`
- Modify: `fallback/free_provider_registry.go`
- Test: `fallback/config_routing_test.go`
- Test: `fallback/free_pool_test.go`

1. Add failing tests proving a refresh failure preserves the previous channel,
   deployment model/capabilities, and successful snapshot.
2. Add failing tests proving Kilo metadata updates the selected deployment while
   OVH inherits conservative defaults.
3. Add one config-lock helper that updates real model and capabilities together.
4. Apply a validated candidate only after the channel transaction succeeds,
   publish snapshot last, and mark failures stale without clearing old data.
5. Change OVH to dynamic `openai_models` mode while keeping static defaults as
   fallback inventory.
6. Run focused tests to green and commit.

## Task 3: Gateway API Projection

**Files:**
- Modify: `router/fallback_gateway_types.go`
- Modify: `router/fallback_gateway_projection.go`
- Modify: `router/fallback.go`
- Test: `router/fallback_gateway_test.go`
- Test: `router/fallback_test.go`

1. Add a failing projection test for catalog status and model capabilities.
2. Add read-only response fields; do not add them to update input types.
3. Return attempted/succeeded/failed counts from manual sync so partial failures
   are not reported as complete success.
4. Deep-copy all maps/slices and preserve key redaction.
5. Run router tests to green and commit.

## Task 4: Chinese Admin Visibility

**Files:**
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProviderRow.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.js`
- Test: `web/default/src/components/fallback-gateway/freePoolUtils.test.js`
- Test: `web/default/src/components/fallback-gateway/FreeModelPool.test.jsx`

1. Add failing tests for current, stale, failed, and never-refreshed labels.
2. Normalize `catalog_status` and `model_capabilities` into provider rows.
3. Add a compact status section with model count, last success, and a
   tooltip/message for the latest error. Reuse the existing sync button.
4. Show a warning when manual sync reports any failed provider refresh.
5. Run focused Vitest tests to green and commit.

## Task 5: Integration And Runtime Verification

**Files:**
- Modify if needed: `docs/evidence/`
- Modify: `AGENTS.md`

1. Run `gofmt`, focused Go tests, full Go tests, and Go build.
2. Run frontend lint, all Vitest tests, Vite build, Storybook build, and
   Playwright desktop/mobile checks.
3. Rebuild `web/build/default` and `one-api.exe` from the integrated branch.
4. Restart localhost port 3008, trigger manual free-pool sync, and verify OVH
   model inventory, Kilo model-level metadata, stale/error behavior, and HTTP
   200 pages without exposing keys.
5. Update handoff/evidence, run `git diff --check`, request code review, and fix
   findings.
6. Merge the feature branch into `main`, push, and confirm all CI jobs.
