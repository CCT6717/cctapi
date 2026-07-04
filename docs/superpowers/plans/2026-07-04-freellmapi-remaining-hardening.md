# FreeLLMAPI Remaining Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining gaps that keep the FreeLLMAPI-style free pool from being safe to operate and easy to manage in cctapi.

**Architecture:** Keep the existing fallback/free-provider boundaries. Add small, testable hardening layers around config persistence first, then expose already-available provider catalog data in the frontend, then add real-provider smoke checks without storing secrets in the repo.

**Tech Stack:** Go 1.22 backend, GORM-backed state tables, React/Semantic UI frontend, PowerShell smoke scripts.

## Global Constraints

- Keep existing `data/fallback.json` plaintext `keys` readable for compatibility until a full encrypted secret store is introduced.
- Do not expose raw free-provider keys through gateway/editor responses, backup files, examples, docs, or test output.
- Preserve current free-provider routing behavior while hardening persistence.
- Use test-first changes for backend behavior.
- Keep provider-specific request quirks centralized or clearly marked until the import-cycle cleanup is safe.

---

### Task 1: Backup Sanitization

**Files:**
- Modify: `router/fallback_config_service.go`
- Modify: `router/fallback_test.go`

**Interfaces:**
- Produces: `sanitizeFallbackBackupData(data []byte) ([]byte, error)`
- Consumes: existing `fallback.SafeKeyHash(key string) string`

- [ ] Add a failing test proving `backupFallbackEditorConfig` does not write raw `free_providers.*.keys[]` values into `data/backups`.
- [ ] Run `go test ./router -run TestBackupFallbackEditorConfigSanitizesFreeProviderKeys -count=1` and confirm RED.
- [ ] Implement backup sanitization by replacing provider `keys` with an empty list and adding non-secret `key_hashes` for operator reference.
- [ ] Run the same router test and confirm GREEN.
- [ ] Run `go test ./router -count=1`.
- [ ] Commit as `fix: sanitize fallback config backups`.

### Task 2: Frontend Free Provider Management

**Files:**
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.js`
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.js`
- Test: `web/default/src/components/fallback-gateway/freePoolUtils.test.js`

**Interfaces:**
- Consumes: backend `free_provider_catalog`, `free_providers`, `key_count`, `quirks`, and effective limits.
- Produces: UI helpers that merge catalog defaults with saved provider overrides without displaying raw keys.

- [ ] Add tests for catalog/config merge behavior, including key counts, enabled state, effective limits, and quirks.
- [ ] Render provider rows from catalog so disabled/unconfigured FreeLLMAPI providers are visible and can be enabled.
- [ ] Display capability/quirk/limit metadata without raw keys.
- [ ] Preserve masked/empty key semantics on save.
- [ ] Run frontend tests for fallback-gateway components.
- [ ] Commit as `feat: surface free provider catalog in gateway ui`.

### Task 3: Real Provider Smoke Path

**Files:**
- Modify: `scripts/fallback-smoke.ps1`
- Modify: `docs/fallback-real-test-checklist.md`
- Modify: `docs/free-pool-ops.md`

**Interfaces:**
- Consumes: local server URL, optional environment-provided API token, and configured providers.
- Produces: a smoke workflow that validates metadata, keyless providers, and one real chat request without printing secrets.

- [ ] Add script options for free-provider catalog inspection and optional model request.
- [ ] Ensure logs redact Authorization and provider keys.
- [ ] Document env vars and expected safe outputs.
- [ ] Run script in metadata-only mode against local config.
- [ ] Commit as `chore: add free provider smoke checklist`.

### Task 4: Quirk Metadata De-duplication

**Files:**
- Create or modify: a neutral package that both `fallback` and `relay/adaptor/openai` can import without cycles.
- Modify: `fallback/free_provider_registry.go`
- Modify: `relay/adaptor/openai/adaptor.go`
- Test: `fallback/free_pool_test.go`, `relay/adaptor/openai/adaptor_test.go`

**Interfaces:**
- Produces: one source of truth for provider runtime quirks.

- [ ] Add a failing test that registry and runtime quirk tables cannot drift.
- [ ] Move runtime-safe quirk definitions into a neutral package.
- [ ] Make fallback registry and OpenAI-compatible adaptor consume the same quirk definitions.
- [ ] Run fallback and adaptor tests.
- [ ] Commit as `refactor: share free provider quirk metadata`.
