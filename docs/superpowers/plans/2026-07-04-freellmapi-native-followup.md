# FreeLLMAPI Native Follow-Up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Continue the FreeLLMAPI-native integration by adding durable usage tracking, safer provider metadata, and provider-quirk plumbing without introducing a second proxy service.

**Architecture:** Keep cctapi's fallback gateway and existing relay path as the integration surface. Add small backend primitives that are testable in isolation: provider catalog projection, per-key quota ledger records, provider quirk metadata, and safe gateway responses.

**Tech Stack:** Go 1.22, GORM with SQLite-compatible tests, existing fallback package, existing router gateway tests.

## Global Constraints

- Work on branch `cleanup/structure-boundaries`.
- Preserve existing `cct/high`, `cct/low`, and `cct/free` behavior.
- Do not remove existing free providers while adding FreeLLMAPI-compatible behavior.
- Use TDD for behavior changes: write a failing test, run it red, implement, run green.
- Keep existing fallback gateway and relay controller surfaces; do not introduce a second proxy service.
- Do not store new real provider keys in examples or docs.

---

### Task 1: Provider Catalog Projection

**Files:**
- Modify: `fallback/free_provider_registry.go`
- Create: `fallback/free_provider_catalog.go`
- Test: `fallback/free_pool_test.go`

**Interfaces:**
- Produces: `type FreeProviderCatalogEntry`
- Produces: `BuildFreeProviderCatalog(cfg *Config) []FreeProviderCatalogEntry`

- [ ] Add a failing test that enabled providers expose provider id, models, default model count, capabilities, limits, key count, and fetch mode without exposing keys.
- [ ] Run `go test ./fallback -run TestBuildFreeProviderCatalog -count=1` and confirm RED.
- [ ] Implement `BuildFreeProviderCatalog` from `BuiltinFreeProviders` and optional `Config.FreeProviders`.
- [ ] Run `go test ./fallback -run TestBuildFreeProviderCatalog -count=1` and confirm GREEN.
- [ ] Commit as `feat: add free provider catalog projection`.

### Task 2: Persistent Free Provider Quota Ledger

**Files:**
- Create: `fallback/free_provider_ledger.go`
- Test: `fallback/free_provider_ledger_test.go`
- Modify: `fallback/state.go`

**Interfaces:**
- Produces: `type FreeProviderUsageLedger`
- Produces: `InitFreeProviderLedgerStore() error`
- Produces: `RecordFreeProviderUsage(deploymentID string, modelName string, usage UsageInfo) error`
- Produces: `GetFreeProviderUsage(provider string, keyHash string, modelName string, period string) (*FreeProviderUsageLedger, error)`

- [ ] Add a failing test that records two successful requests for `free:groq-001122ff` and stores one row keyed by provider `groq`, key hash `001122ff`, model name, and current quota period.
- [ ] Run `go test ./fallback -run TestRecordFreeProviderUsage -count=1` and confirm RED.
- [ ] Implement the GORM model and upsert logic using the same quota period as `DeploymentState`.
- [ ] Wire `InitStateStore` to migrate the ledger table.
- [ ] Run `go test ./fallback -run TestRecordFreeProviderUsage -count=1` and confirm GREEN.
- [ ] Commit as `feat: persist free provider usage ledger`.

### Task 3: Relay Usage Hook

**Files:**
- Modify: `fallback/state.go`
- Modify: `controller/relay.go`
- Test: `fallback/free_provider_ledger_test.go`

**Interfaces:**
- Consumes: `RecordFreeProviderUsage`
- Produces: successful fallback requests on auto free deployments also update the free provider ledger.

- [ ] Add a failing test for `RecordFreeProviderUsage` ignoring non-auto/manual deployment IDs and accepting hash-format auto IDs.
- [ ] Run focused fallback tests and confirm RED.
- [ ] Call `RecordFreeProviderUsage(dep.ID, dep.RealModel, usage)` from the existing fallback success usage path.
- [ ] Run `go test ./fallback ./controller -count=1` and confirm GREEN.
- [ ] Commit as `fix: record free provider ledger from fallback success`.

### Task 4: Provider Quirk Metadata

**Files:**
- Modify: `fallback/free_provider_registry.go`
- Modify: `router/fallback_gateway.go`
- Test: `fallback/free_pool_test.go`
- Test: `router/fallback_gateway_test.go`

**Interfaces:**
- Produces: `FreeProviderQuirks`
- Produces gateway-visible, non-secret provider quirk metadata.

- [ ] Add failing tests that NVIDIA exposes `force_parallel_tool_calls=false`, Routeway exposes a default User-Agent hint, and AIHorde exposes stream/max token constraints.
- [ ] Run focused fallback/router tests and confirm RED.
- [ ] Add quirk metadata to provider registry and gateway projection.
- [ ] Run focused fallback/router tests and confirm GREEN.
- [ ] Commit as `feat: expose free provider quirk metadata`.

### Task 5: Verification and Push

**Files:**
- No intended production edits.

**Interfaces:**
- Produces a tested branch ready to merge from another machine.

- [ ] Run `go test ./fallback ./router ./controller -count=1`.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `go build -o one-api.exe`.
- [ ] Confirm `git status --short` is clean.
- [ ] Push `cleanup/structure-boundaries` to origin.
