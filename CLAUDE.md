# CLAUDE.md

This file gives Claude Code current, practical guidance for working in this repository.

## Project

`cctapi` is a CCT fork of `songquanpeng/one-api` with a virtual model fallback system.

The user usually works in Chinese and verifies the local app at:

```powershell
http://localhost:3007
```

## Current Verified Local State

Last locally checked state:

- Project path: `D:\project\cctapi`.
- Default port: `3007`.
- Local service helpers exist:
  - `scripts\start-cctapi.ps1`
  - `scripts\stop-cctapi.ps1`
- Current start command:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\start-cctapi.ps1 -NoBrowser
```

- Current stop command:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\stop-cctapi.ps1
```

- `http://localhost:3007` was checked locally and returned HTTP 200 while a process was listening on port `3007`.
- The repository was not clean at the last check: `web/build/default` generated assets were modified and `one-api-check.exe` was untracked. Treat these as likely build/check artifacts unless the user says otherwise.

Recently verified feature locations:

- Structured fallback error classification: `fallback/error.go`; `controller/relay.go` calls `fallback.ClassifyRelayError`.
- Per-deployment concurrency guard: `fallback/concurrency.go`; relay loop uses `fallback.TryAcquireDeploymentSlot`.
- Sticky warm-up: `fallback/state.go` contains `WarmUpStickyState`.
- Fallback history cleanup: `fallback/cleanup.go`; `main.go` calls `fallback.StartHistoryCleanup()`.
- Fixed routing: `fallback/config.go`, `router/fallback_config.go`, and related tests.
- Soft limit default: backend config loading defaults `soft_limit_ratio` to `0.95`.
- Free/upstream-managed quota mode: local token quota is skipped when `quota_mode == "free"` or `daily_limit_tokens == 0`.
- Doubao 24-hour cooldown: relay code calls `MarkDeploymentCooldownForDuration(..., 24*time.Hour)` for Doubao quota/limit skip paths.
- Real routed model logging: `model.Log.RealModelName`, `relay/controller/helper.go`, and `web/default/src/components/LogsTable.js`.
- Default frontend fallback panel: `web/default/src/pages/Fallback/`.

Priority observation points during trial use:

- Usage statistics should show both the virtual model and the actual routed real model, with correct token usage.
- Doubao quota recovery should be counted from the moment the deployment is skipped, using a relative 24-hour cooldown, not a natural-day reset.
- OpenRouter/free deployments should not be blocked by local daily quota logic.
- Fixed routing should stay pinned to the selected deployment and not drift to another deployment.
- If the virtual model list only shows one model, inspect `data/fallback.json`, fallback config normalization, API filtering, and frontend rendering conditions first.

## Recent SenseNova Fix

On 2026-06-10, channel `商汤2` / model `sensenova-6.7-flash-lite` was fixed for real relay traffic through `core/go`.

What was fixed:

- `controller/relay.go`: non-fallbackable errors must write a JSON response before returning; otherwise clients can see an empty body and fail with `Unexpected end of JSON input`.
- `fallback/error.go`: provider payloads with `code = "internal_server_error"` should be treated as temporary upstream errors even when the HTTP status is 400. SenseNova wraps upstream 500 this way.
- `relay/adaptor/openai/adaptor.go`: for generic `OpenAICompatible` channels, strip Anthropic-style `cache_control` fields from `messages.content[]` before sending upstream. SenseNova accepts normal OpenAI content parts but can fail on this extension.
- `relay/controller/text.go`: debug logs should not print the full converted request body. Log request metadata only, because real requests can contain large prompts and private context.
- `fallback/state.go`: deployment state creation is idempotent under startup concurrency via `ON CONFLICT DO NOTHING`, preventing `UNIQUE constraint failed: deployment_states.deployment_id, deployment_states.date`.
- `common/message/email.go`: SMTP address construction uses `net.JoinHostPort`, so IPv6 SMTP hosts are valid.

Current SenseNova config:

- `data/fallback.json` contains deployment `core-go-model-16` for channel `10`, real model `sensenova-6.7-flash-lite`.
- `core/go` is currently fixed to `core-go-model-16`.
- Do not add a custom model ratio unless the user asks; falling back to the default ratio is acceptable.

Verification after the fix:

```powershell
go test ./relay/adaptor/openai ./relay/controller ./fallback/...
go vet ./...
go test ./...
go build -o one-api.exe .
powershell -ExecutionPolicy Bypass -File scripts\stop-cctapi.ps1
powershell -ExecutionPolicy Bypass -File scripts\start-cctapi.ps1 -NoBrowser
```

The local service was restarted on port `3007`, `http://localhost:3007` returned HTTP 200, and the user confirmed SenseNova responded normally.

## Git Convention

- Commit messages must be written in **Chinese**.
- Do not push to remote without asking.
- If a commit fails due to hooks, fix and create a **new** commit — do not amend.
- The user should confirm before any `git push` or force push.

## Build And Run

Always rebuild the default frontend before rebuilding the Go binary, because the Go server serves the generated `web/build/default` assets.

```powershell
cd D:\project\cctapi\web\default
npm run build

cd D:\project\cctapi
go build -o one-api-new.exe .
```

To replace the running local server on port `3007`, stop the process on that port, move `one-api-new.exe` over `one-api.exe`, then start with `PORT=3007`.

Important: this repository embeds `web/build/default` into the Go binary. If the user says a frontend change is not visible, verify the page is loading the latest hashed JS/CSS from `web/build/default/index.html`, rebuild the Go binary, replace the running `one-api.exe`, and restart the `3007` server.

Useful checks:

```powershell
go build ./...
go test ./fallback
cd D:\project\cctapi\web\default; npm run build
```

The frontend build has existing ESLint warnings in unrelated files. A successful build with warnings is expected.

## Windows Local Service Helpers

Use the scripts in `scripts/` for local startup:

```powershell
cd D:\project\cctapi
powershell -ExecutionPolicy Bypass -File scripts\start-cctapi.ps1
powershell -ExecutionPolicy Bypass -File scripts\stop-cctapi.ps1
powershell -ExecutionPolicy Bypass -File scripts\install-cctapi-autostart.ps1
powershell -ExecutionPolicy Bypass -File scripts\uninstall-cctapi-autostart.ps1
```

The autostart installer creates a Windows scheduled task named `CCT API Local Server` that runs `scripts/start-cctapi.ps1 -NoBrowser` at user logon.

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

The model scoring page uses a bounded trend view:

- The score trend area is no longer a line chart. It is a grouped leaderboard, one group per virtual model, so deployments from `high/auto`, `low/auto`, and `all/auto` are not mixed together.
- The score table uses compact horizontal leaderboard bars plus the latest delta instead of the older full-width score bar.
- Real deployment base URLs should be visible in the virtual model edit/detail UI so administrators can identify which upstream each real model uses.

## Added CCT API Features

Important additions over upstream One API:

- Virtual models: one exposed model name maps to multiple real upstream deployments.
- Weighted routing, sequential routing, and fixed routing.
- Fixed routing pins a virtual model to one selected real deployment through `fixed_deployment`; runtime upstream errors do not rotate to another deployment unless the administrator changes the fixed target.
- Sticky routing: after a virtual model selects a healthy deployment, keep using that deployment until quota threshold, hard limit, cooldown, or an upstream error forces a switch. Do not rotate on every request just because another deployment is also available.
- Per-deployment token quota, soft limit, hard limit, and concurrency limit.
- Quota modes:
  - controlled deployments use configured daily token quota and should switch before exhaustion, around the 95% soft threshold.
  - free/upstream-limited deployments can use `daily_limit_tokens = 0`; they are skipped only after upstream errors, cooldown, or hard failure state.
- Manual cooldown and recover actions.
- Smart score trend chart for deployment ordering, including compact table trends and a simplified top-deployment chart.
- Runtime health panel with recent success rate, failure rate, cooldown count, exhausted quota count, and Top failure aggregations.
- Alert history and fallback switch logs.
- Frontend and backend validation before saving fallback config, including fixed-route target checks.
- Smoke test script for real client testing.
- **Free Pool (cct/free)**: Automatic free LLM provider aggregation. Supports 18 providers: OpenRouter Free (dynamic `:free` models, needs API key), Groq (static list, needs API key), Kilo (keyless, dynamic `isFree:true` models), Pollinations (keyless, static `openai-fast` model), OVH (keyless, 15 static chat models), SiliconFlow (adaptor ModelList, keyless ok), Zhipu (adaptor ModelList, keyless ok), and 11 pre-built disabled providers (Mistral, Together AI, Novita, Cloudflare, Cerebras, SambaNova, GitHub Models, Chutes, Fireworks, Nebius, Lambda Labs). Sync timer runs every 6h for models, 15m for OpenRouter credits. Configuration in `data/fallback.json` under `free_providers`.

## Important Files

```text
fallback/                         Core fallback package
fallback/free_pool.go             Free pool provider registry, model fetch, quota sync, StartFreeSync
fallback/config.go                Config loading, validation, UpdateDeploymentDailyLimit
fallback/health.go                Health checker (free deployments now ping normally)
fallback/alert.go                 Alert manager
fallback/quota.go                 Runtime quota state, RPM/RPD/TPM/TPD enforcement
router/fallback.go                Fallback admin API and built-in HTML fallback pages
controller/relay.go               Main fallback relay loop
common/metrics.go                 Prometheus text metrics
web/default/src/pages/Fallback/   Default-theme fallback panel
web/default/src/components/FallbackConfigPanel.js
web/default/src/components/fallback-gateway/FreeProvidersEditor.js
web/default/src/components/fallback-gateway/FreeModelPool.js
web/default/src/components/Footer.js
scripts/fallback-smoke.ps1        Real client smoke test script
```

## Smoke Test

Use a real API token and a virtual fallback model:

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3007"
$env:CCT_API_TOKEN = "sk-..."
$env:CCT_API_MODEL = "high/auto"
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1
```

The script checks:

- non-stream `/v1/chat/completions`
- stream `/v1/chat/completions`
- `/metrics`

Do not hardcode real tokens in repo files.

## Key Environment Variables

Most settings are controlled via `.env` at the project root:

| Variable | Default | Description |
|---|---|---|
| `PORT` | 3007 | Server listen port |
| `FALLBACK_CONFIG_PATH` | data/fallback.json | Fallback config file |
| `SESSION_SECRET` | (auto) | Session encryption key |
| `GIN_MODE` | release | Gin debug/release mode |
| `GLOBAL_API_RATE_LIMIT` | 480 | Global API rate limit |
| `ENABLE_METRIC` | false | Enable Prometheus metrics |
| `THEME` | default | Frontend theme: default/air/berry |
| `SQLITE_PATH` | one-api.db | SQLite database location |
| `RELAY_TIMEOUT` | 0 | Relay timeout in seconds |
| `DEBUG` | false | Enable debug logging |
| `CHANNEL_TEST_FREQUENCY` | — | Auto test channels (seconds) |

## Frontend Themes

Three themes available, toggled via `THEME` env or admin settings:

| Theme | Framework | Source |
|---|---|---|
| `default` | Semantic UI React | `web/default/` |
| `air` | Semi UI (ByteDance) | `web/air/` |
| `berry` | MUI (Material) | `web/berry/` |

To build a specific theme:
```powershell
cd D:\project\cctapi\web\<theme>
npm run build
# Result goes to web/build/<theme>/
```

## Router Structure (Gin)

| Route | Package | Description |
|---|---|---|
| `/api/fallback/*` | `router/fallback.go` | Fallback admin API + built-in HTML pages |
| `/api/*` | `router/api.go` | Main API (channels, tokens, logs, etc.) |
| `/relay/*` | `router/relay.go` | Relay proxy endpoints |
| `/dashboard/*` | `router/dashboard.go` | Dashboard data endpoints |
| `/web/*` | `router/web.go` | Static files and settings UI |

The fallback router has admin endpoints for reading deployment states, managing cooldown/recover, batch operations, switch logs, alert records, model scores, and config editing.

## CI

`.github/workflows/ci.yml` includes:

- Go tests with coverage
- default frontend `npm ci` and `npm run build`
- commit lint

The frontend CI build sets `CI=false` because this inherited codebase has existing ESLint warnings that should not block build verification.

## Current Fallback Config

The file `data/fallback.json` is intentionally committed as the current working fallback skeleton. It currently uses an object-based config shape:

```json
{
  "enabled": true,
  "virtual_models": {
    "high/auto": {
      "enabled": true,
      "routing_mode": "weighted",
      "fallback_order": ["doubao-pro", "doubao-code", "doubao-vision", "glm-47"]
    },
    "low/auto": {
      "enabled": true,
      "routing_mode": "weighted",
      "fallback_order": [
        "doubao-lite",
        "doubao-mini",
        "doubao-18",
        "doubao-16",
        "openrouter-new-free",
        "openrouter-old",
        "openrouter-new"
      ]
    },
    "all/auto": {
      "enabled": true,
      "routing_mode": "sequential",
      "fallback_order": [
        "doubao-code",
        "doubao-18",
        "doubao-16",
        "openrouter-new-free",
        "openrouter-old",
        "openrouter-new",
        "all-auto-model-7"
      ]
    }
  },
  "deployments": {}
}
```

Key fields per virtual model: `enabled`, `routing_mode` (`weighted`, `sequential`, or `fixed`), `fixed_deployment` for fixed routing, and `fallback_order`.

Key fields per deployment: `enabled`, `channel_id`, `real_model`, `priority` (lower = tried first), `weight` (for weighted routing), `daily_limit_tokens` (0 = upstream-managed), `soft_limit_ratio`, `hard_limit_ratio`, `max_concurrent_requests`, `max_context`, and `min_context`.

Current deployment groups:

- `high/auto`: premium/high capability Doubao + GLM group, weighted routing.
- `low/auto`: lightweight Doubao models plus OpenRouter fallback models, weighted routing.
- `all/auto`: broad fallback chain, sequential routing. `doubao-pro` is not currently part of `all/auto`.

## Incomplete Features

- **Alert enhancement**: The alerts panel (`/fallback/status` section 4) still needs a "mark all read" button, jump-to-deployment links, and alert rules UI.

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
- Fallback should happen immediately for upstream errors, stream interruptions, 429/rate-limit style failures, quota exhaustion, cooldown state, and hard-limit skips. Add debug logs around deployment attempts when diagnosing fallback behavior.
- For quota behavior, preserve the user's intended two-category model:
  - managed models with explicit daily limits switch at the soft threshold before full exhaustion.
  - upstream-limited/free models normally have no local quota and switch only when the upstream actually fails or is cooled down.
- Top failure model/channel in the runtime panel is currently derived from switch logs. It is approximate. Exact failure ranking would require a backend deployment-attempt event table or a dedicated health aggregation endpoint.

## Security Fix: Fallback Editor Key Masking (2026-06-21)

A security review was performed with the following findings and fixes:

### Audit Result: FAIL -> PASS

After fixes: SECURITY_REVIEW_STATUS: PASS

### Finding 1: OpenRouter API Key in Git History

- A real OpenRouter key was committed in a test report document (.md) and persisted in git history.
- Fix: Used git filter-repo --replace-text to clean the key from all 1433 commits (main + archive + tags).
- Action needed: Revoke the old key in OpenRouter dashboard, generate a new one, update data/fallback.json.

### Finding 2: Editor API Returned Full Channel Keys

- router/fallback_config.go:buildFallbackEditorChannel() returned channel.Key unmasked in JSON.
- Fix: Changed struct field: Key string json:"key" -> KeyMasked string json:"key_masked" + HasKey bool json:"has_key".
- maskSecretKey(): shows first 4 + **** + last 4, same length as original.
- upsertFallbackEditorChannel(): incoming key_masked empty or contains *** -> preserve existing key.
- Frontend (FallbackConfigPanel.js): channel.key -> channel.key_masked.

### Security Tests Added (router/fallback_test.go)

| Test | Assertion |
|------|-----------|
| TestMaskSecretKey_Empty | Empty -> empty |
| TestMaskSecretKey_ShortKey | <=8 -> ******** |
| TestMaskSecretKey_DoesNotLeakOriginal | No original, first/last 4 correct, has **** |
| TestMaskSecretKey_Length | Output length = input length |
| TestBuildFallbackEditorChannel_NoFullKey | No full key, key_masked with ****, has_key=true |
| TestBuildFallbackEditorChannel_NoKey | Empty key -> key_masked="" has_key=false |

### Git History Note

- History rewritten by git filter-repo. HEAD: 267e5b11 -> f200816.
- No remote configured; no force push needed.
