# AGENTS.md

This file gives Codex agents current, practical guidance for working in this repository.

## Project

`cctapi` is a CCT fork of `songquanpeng/one-api` with a virtual model fallback system.

The user usually works in Chinese and verifies the local app at:

```powershell
http://127.0.0.1:3008
```

Current local checkout used by Codex:

```powershell
D:\ct\project
```

Current integration branch:

```text
cleanup/structure-boundaries
```

## Build And Run

Always rebuild the default frontend before rebuilding the Go binary, because the Go server serves the generated `web/build/default` assets.

```powershell
cd D:\ct\project\web\default
npm run build

cd D:\ct\project
go build -o one-api.exe .
```

To replace the running local server on port `3008`, stop the existing `one-api` process, rebuild `one-api.exe`, then start it with `--port 3008`.

Useful checks:

```powershell
go build ./...
go test ./fallback
cd D:\ct\project\web\default; npm run build
```

The frontend build has existing ESLint warnings in unrelated files. A successful build with warnings is expected.

## Current Handoff

Last verified handoff: 2026-07-06.

- Branch `cleanup/structure-boundaries` is pushed to `origin/cleanup/structure-boundaries`.
- Latest handoff commit: `efc3573 fix: complete free pool ui review fixes`.
- Local preview route: `http://127.0.0.1:3008/fallback/free-pool`.
- Verify the free-pool UI visually before merging this branch into the main branch.

Final verification from the free-pool UI batch:

```powershell
cd D:\ct\project\web\default
npm test -- --watchAll=false
npm run build
$env:CI='true'; $env:STORYBOOK_DISABLE_TELEMETRY='1'; npm run build-storybook

cd D:\ct\project
go build -o one-api.exe .
curl.exe -I http://127.0.0.1:3008/
```

Expected result from the final pass:

- Frontend tests: 7 suites passed, 33 tests passed.
- Frontend production build: passed with existing unrelated ESLint warnings.
- Storybook build: passed with asset-size warnings only.
- Local server on port 3008: HTTP 200.

## Runtime Files

Do not delete:

- `.env`
- `one-api.db`
- the running `one-api.exe`
- logs that are currently locked by a running process

Safe cleanup targets are old ignored logs, old backup binaries such as `one-api.exe~`, and stale redirected stdout/stderr files after confirming they are not locked.

## Fallback UI

The custom fallback UI is concentrated under the deployment status panel:

```text
/fallback/status
```

The independent dashboard shortcut cards were intentionally removed. Do not re-add fallback shortcut cards to the main overview/dashboard page.

Current fallback panel navigation has five sections:

- Deployment status
- Runtime data
- Model scoring
- Alert records
- Switch logs

There is no separate "connectivity test" panel. Connectivity testing lives in the virtual model configuration module on `/fallback/status`.

## Free Pool UI

The FreeLLMAPI/free-model-pool admin UI is available at:

```text
/fallback/free-pool
```

Keep user-facing copy on this page in Chinese. Preserve provider brand names and technical abbreviations such as RPM, RPD, TPM, TPD, JSON, API key, and token.

Recent completed UI work on `cleanup/structure-boundaries`:

- Provider operation polish: search, bulk enable/disable, row state classes, and stable test selectors.
- Fallback visual consistency: inputs, focus states, table density, card density, long-content handling, and moderated color intensity.
- Information hierarchy: compact readiness/status strip, workflow summary, and clearer next-action guidance.
- Storybook fixtures for `FreePoolWorkflowDashboard` include `statusTone` and `statusText`; keep them in sync with `buildFreePoolWorkflowSummary`.

Important frontend files:

```text
web/default/src/components/fallback-gateway/FreeModelPool.js
web/default/src/components/fallback-gateway/FreeProvidersEditor.js
web/default/src/components/fallback-gateway/FreeProviderRow.js
web/default/src/components/fallback-gateway/FreePoolWorkflowDashboard.js
web/default/src/components/fallback-gateway/freePoolUtils.js
web/default/src/components/fallback-gateway/gatewayConfigApi.js
web/default/src/pages/Fallback/Fallback.css
web/default/src/pages/Fallback/index.js
```

Focused tests:

```powershell
cd D:\ct\project\web\default
npm test -- --watchAll=false --runTestsByPath src/components/fallback-gateway/freePoolUtils.test.js src/components/fallback-gateway/freeProviderDisplay.test.js src/components/fallback-gateway/FreeModelPool.test.js src/pages/Fallback/index.test.js src/pages/Fallback/Fallback.test.js
```

## Added CCT API Features

Important additions over upstream One API:

- Virtual models: one exposed model name maps to multiple real upstream deployments.
- Weighted routing, sequential routing, and fixed routing.
- The current default-theme gateway editor at `/fallback/gateway` uses "preferred start deployment" semantics for the `固定` button:
  - it saves `routing_mode = fallback`
  - it sets `preferred_deployment`
  - runtime upstream failure should still fallback to other deployments inside the same VM
- True backend fixed routing still exists, but do not assume the gateway editor's `固定` button means no-fallback fixed routing.
- Per-deployment token quota, soft limit, hard limit, and concurrency limit.
- Manual cooldown and recover actions.
- Smart score trend chart for deployment ordering.
- Runtime health panel with recent success rate, failure rate, cooldown count, exhausted quota count, and Top failure aggregations.
- Alert history and fallback switch logs.
- Frontend and backend validation before saving fallback config, including fixed-route target checks.
- Smoke test script for real client testing.

## FreeLLMAPI Integration Notes

`cctapi` already has an `OpenAICompatible` channel path. A standards-compliant OpenAI-style upstream such as FreeLLMAPI can be added without rewriting the relay core in most cases.

Keep protocol translation and compatibility code in relay/model-style boundaries where possible. Controllers should orchestrate request capture, conversion, relay invocation, and final response emission; avoid duplicating routing, billing, fallback, or retry logic in controllers.

FreeLLMAPI is not just a thin proxy. Treat the target feature set as:

- OpenAI-compatible `/v1/chat/completions` and `/v1/models`.
- Responses compatibility.
- Anthropic-style messages compatibility where needed.
- Model auto-routing and sticky routing.
- Retries, cooldown, and circuit-breaker behavior.
- Tool-call rescue.
- Timing-safe bearer or x-api-key auth behavior.
- Admin visibility for real provider health, usage, sync, and runtime errors.

Recommended next backend tasks after UI acceptance:

- Real free-provider health checks and surfaced error reasons.
- Model sync/status refresh that updates admin-visible state.
- Retry, cooldown, and circuit-breaker behavior aligned with existing fallback routing.
- Sticky routing and automatic route selection for free providers.
- Tool-call rescue compatibility.
- Admin UI/API display of real runtime failures rather than local-only sync errors.

## Important Files

```text
fallback/                         Core fallback package
router/fallback.go                Fallback admin API and built-in HTML fallback pages
controller/relay.go               Main fallback relay loop
common/metrics.go                 Prometheus text metrics
web/default/src/pages/Fallback/   Default-theme fallback panel
web/default/src/components/fallback-gateway/  Free-pool and gateway editor components
web/default/src/components/FallbackConfigPanel.js
web/default/src/components/Footer.js
scripts/fallback-smoke.ps1        Real client smoke test script
```

## Smoke Test

Use a real API token and a virtual fallback model:

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "sk-..."
$env:CCT_API_MODEL = "high/auto"
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1
```

The script checks:

- non-stream `/v1/chat/completions`
- stream `/v1/chat/completions`
- `/metrics`

Do not hardcode real tokens in repo files.

## CI

`.github/workflows/ci.yml` includes:

- Go tests with coverage
- default frontend `npm ci` and `npm run build`
- commit lint

The frontend CI build sets `CI=false` because this inherited codebase has existing ESLint warnings that should not block build verification.

## Footer Attribution

Default footer should preserve upstream attribution and add CCT fork attribution:

```text
CCT API is forked by CCT based on One API.
One API is built by JustSong and licensed under MIT.
```

Do not remove upstream One API / JustSong / MIT attribution.

## Implementation Notes

- Keep fallback admin features in `/fallback/status` unless there is a strong reason to expose them elsewhere.
- Prefer existing project patterns and Semantic UI React for the default theme.
- Do not add another standalone connectivity-test panel; extend the existing virtual model config module instead.
- In the gateway editor, deployment ownership and per-VM mode control should be derived from `fallback_order` first, with `pools` only as a fallback when no explicit `fallback_order` exists.
- In the gateway editor, selecting a new `fixed` or `quota` deployment inside one VM must clear any existing config-derived or draft `fixed` / `quota` selection in that same VM, so the UI never shows multiple active rows for the same exclusive mode.
- Top failure model/channel in the runtime panel is currently derived from switch logs. It is approximate. Exact failure ranking would require a backend deployment-attempt event table or a dedicated health aggregation endpoint.
