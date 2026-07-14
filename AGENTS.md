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
main
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

The frontend build currently has zero ESLint warnings. A clean build is expected.

## Security Startup Requirements

- A new empty database no longer creates `root/123456`. Set `INITIAL_ROOT_PASSWORD` to a unique value of at least 12 characters before first startup. Existing databases and existing root credentials are unchanged.
- `SESSION_COOKIE_SECURE` accepts `true`, `false`, or `auto` and defaults to `auto`. Use `true` whenever public HTTPS terminates at a reverse proxy; also set `SERVER_ADDRESS` to the public HTTPS address and preserve the original `Host` header when possible.
- Browser session APIs are same-origin and protected by CSRF origin checks. OpenAI-compatible token routes retain credential-free CORS and must never advertise `Access-Control-Allow-Credentials`.
- Build and release with Go 1.26.5 or newer on the same patched release line. On this machine, CGO/SQLite builds must use `D:\ct\tools\w64devkit-1.23.0\bin`; the newer `D:\ct\tools\w64devkit` GCC 16 installation is incompatible with the current Go COFF parser.
- CI pins `govulncheck` v1.6.0. A clean security gate reports zero reachable vulnerabilities; imported but unreachable findings are informational unless call paths change.

## Current Handoff

Last verified handoff: 2026-07-14.

- Branch `main` is pushed to `origin/main`.
- Security hardening is merged through `5409921`: default root credentials are removed, session cookies and browser mutations are protected, token CORS is scoped, internal errors are sanitized, JWT/dependencies use patched releases, and CI runs pinned `govulncheck` v1.6.0.
- Latest smoke hardening commit: `36a6dc5 fix(smoke): wait for complete usage accounting`.
- Preview release tag for this handoff: `v0.1.0-freellmapi-preview`.
- Frontend build toolchain migrated from CRA to Vite 6.4.3 + Vitest 3.2.7 + Storybook Vite builder 10.5.0.
- react-scripts and Webpack dependency chain fully removed.
- All routed page components are loaded with `React.lazy` and `Suspense`; the production entry bundle is split into route chunks.
- Playwright smoke coverage contains 16 desktop/mobile checks for public pages, protected redirects, authenticated admin pages, and responsive scrolling.
- Web rate limiting skips immutable static assets so route chunks cannot be blocked with HTTP 429 during rapid navigation.
- Local preview route: `http://127.0.0.1:3008/fallback/free-pool`.
- Free-pool UI acceptance and backend FreeLLMAPI core integration are validated and currently in release-ready state.
- Added openrouter/auto production smoke playbook:
  - `docs/openrouter-auto-acceptance-checklist.md`
  - `docs/openrouter-auto-stability-submission.md`
  - `docs/openrouter-auto-deployment-acceptance.md`
  - `docs/openrouter-auto-final-submission-template.md`
  - `docs/openrouter-auto-stability-runbook.md`
  - `scripts/fallback-openrouter-auto-smoke.ps1`
- OpenRouter stability work is complete at the application boundary. The smoke script now waits until both non-stream and stream requests appear in the usage ledger, while enforcing a deadline before every poll.
- The 2026-07-13 paced soak sent 50 requests at 5.2-second intervals with no retries: 50/50 succeeded, average latency was 1737.01 ms, p95 was 3727.4 ms, and 14 free response models were observed. Final usage and fallback deltas were all +50.
- The separate burst probe succeeded for 14 requests before OpenRouter returned `free-models-per-min`; the gateway correctly applied a 60-second cooldown. This is an upstream free-tier rate limit, not a supported production traffic profile.
- OpenRouter's daily free-model quota was exhausted after acceptance. A final live rerun is currently blocked by `free-models-per-day`; adding 10 OpenRouter credits unlocks the larger daily allowance. Do not diagnose this external quota response as a local regression.
- Acceptance evidence: `docs/evidence/openrouter-auto-soak-2026-07-13.json`.
- Historical and temporary smoke tokens were removed; no acceptance token remains in the local database.
- Current runtime binary has been rebuilt with the latest `web/build/default` assets and restarted on port `3008`.
- Vite migration screenshots are verified under `screenshots/vite-migration/`: 7 pages at desktop (`1425x891`) and mobile (`360x640`) viewports, with distinct hashes and confirmed page content.
- Final channel/settings responsive acceptance screenshots are under `screenshots/final-validation/`.
- Post-review `openrouter/auto` smoke passed with non-stream, stream, usage, and fallback metric checks before the daily quota was exhausted; `pageContainsOpenRouterAuto` remains informational.
- Multi-provider FreeLLMAPI acceptance is archived at `docs/evidence/freellmapi-multi-provider-acceptance-2026-07-13.json`.
- The live free pool now has enabled OpenRouter, Kilo, OVH, and Pollinations deployments. OpenRouter retains one stored key; the other three providers are keyless.
- Forced provider routing passed from Kilo to Pollinations with real non-stream and stream responses, runtime-observed sticky reuse, and complete cleanup.
- Real protocol coverage passed: Chat Completions, Responses, and Anthropic Messages in non-stream and stream modes through Kilo; a structured tool call with nested array arguments passed through Pollinations.
- Health checks no longer mark unexpected 4xx responses healthy. The stale OVH `Llama-3.1-8B-Instruct` default was removed, and Pollinations tool-call capability is represented in deployment metadata.
- OpenRouter remained blocked by its external daily free quota during this acceptance. OVH used a current model after sync but public requests remained intermittently rate limited. These are recorded external gaps, not local acceptance passes.
- Manual Kimi Code channel `#22` remains enabled and outside the generated free pool. No `verify-mp` or `verify-proto` token remains.
- Dynamic provider catalog refresh is complete. Validated Kilo and OVH metadata is persisted atomically, restored on startup, exposed through the admin API/UI, and used for tools/JSON/vision-aware routing.
- Catalog refresh outcomes are tied to the active config generation. Superseded successes and failures are skipped without changing the last good snapshot, and queued superseded work does not call upstream providers.
- OpenRouter keeps the stable `openrouter/free` alias by default while preserving an explicitly selected concrete deployment model. Explicit provider-level `models` still takes precedence.
- Catalog HTTP timeout is 15 seconds. The 2026-07-14 live sync passed for Kilo (11 models) and OVH (14 models): 2 providers succeeded, 0 failed, 0 skipped.
- Current local runtime is the rebuilt `one-api.exe` on port `3008`; post-deploy Playwright acceptance passed 16/16 desktop/mobile checks.
- Catalog refresh evidence: `docs/evidence/provider-catalog-refresh-2026-07-14.json`.

Final verification from the Vite migration batch:

```powershell
cd D:\ct\project\web\default
npm run lint
npm test
npm run build
$env:STORYBOOK_DISABLE_TELEMETRY='1'; npm run build-storybook

cd D:\ct\project
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./... -count=1
& 'D:\ct\tools\go\bin\go.exe' build -o one-api.exe .
curl.exe -I http://127.0.0.1:3008/
```

Expected result from the latest pass:

- Frontend tests: 9 suites passed, 44 tests passed (Vitest 3.2.7).
- Browser tests: 16 Playwright checks passed across desktop Chromium and Mobile Chrome.
- Frontend production build: passed (Vite 6.4.3), ESLint 0 errors and 0 warnings.
- Storybook build: passed with asset-size warnings only.
- Local server on port 3008: HTTP 200.

Latest verification on `main`:

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./... -count=1
& 'D:\ct\tools\go\bin\go.exe' build ./...
```

Expected result from the latest pass:

- Full Go tests: all packages passed, 0 failed.
- Go build: passed.
- `git diff --check`: no actual formatting errors, only Git line-ending warnings.

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./router -count=1
& 'D:\ct\tools\go\bin\go.exe' test ./... -count=1
& 'D:\ct\tools\go\bin\go.exe' build ./...
```

Expected result from the latest pass:

- `./fallback` and `./router` tests: passed.
- Full Go tests: passed.
- Go build: passed.
- `git diff --check`: no actual formatting errors, only Git line-ending warnings.

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
npm test -- src/components/fallback-gateway/freePoolUtils.test.js src/components/fallback-gateway/freeProviderDisplay.test.js src/components/fallback-gateway/FreeModelPool.test.jsx src/pages/Fallback/index.test.js src/pages/Fallback/Fallback.test.jsx
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

Completed backend alignment after UI acceptance:

- Real free-provider health checks and surfaced error reasons. Manual health checks now write runtime errors for channel, auth, rate-limit, timeout, and 5xx cases; provider JSON/text error bodies are parsed into runtime diagnostics and cooldown reasons.
- Model sync/status refresh that updates admin-visible state. Admin `/free-pool/sync` now runs resource sync plus dynamic model and credit refresh; dynamic model refresh failures are surfaced through runtime diagnostics.
- Retry, cooldown, and circuit-breaker behavior partially aligned:
  - 401/403 `ModelAccess` failures now go through long cooldown + sticky invalidation in relay routing.
- Sticky routing and automatic route selection for free providers.
- Tool-call rescue compatibility. Structured tool-call argument repair is implemented in the relay/model boundary for OpenAI-compatible chat completions.
- Admin UI/API display of real runtime failures rather than local-only sync errors. Runtime status now exposes persistent state, cooldown, exhausted, sticky route, and free-pool sync/health-check errors for the admin UI.

Completed dynamic catalog work:

- Kilo, OVH, OpenAI-compatible, and OpenRouter catalog sources use validated response parsing with an 8 MiB response limit and a 15-second request timeout.
- Successful snapshots update channel abilities and persistent catalog state in one transaction. Failed refreshes preserve the last successful model inventory and capabilities.
- Per-model tools, JSON, vision, streaming, and context metadata is applied at the routing boundary without moving compatibility logic into controllers.
- Admin sync reports aggregate provider/key outcomes without exposing keys, and partial failures or skips are never reported as full success.


Completed Kilo model-level rate-limit rotation:

- In-memory model runtime registry with per-model cooldowns, success/failure counters, consecutive 429 tracking, and deep-copy snapshots.
- Capability-aware Kilo model attempt planner that filters alternatives by stream, tools, JSON, vision, and context requirements before each attempt.
- Request-scoped model rotation in the relay fallback loop: a Kilo 429 cools only the affected model, retries the next compatible model within the same deployment, and penalizes the provider only when all compatible models are exhausted.
- Non-429 Kilo errors skip remaining models and fall through to existing provider-level behavior without duplicate penalties.
- Written-stream guard prevents replay after response bytes have reached the client.
- Manual deployment recovery (`ResetDeploymentState`) clears both persistent provider cooldowns and in-memory model cooldowns.
- Runtime API exposes `model_runtime` snapshots through the existing `/api/fallback/deployments/runtime-status` endpoint.
- Free-pool admin UI renders compact Chinese diagnostics for actively cooling Kilo models (model ID, consecutive 429 count, cooldown deadline).
- All changes covered by planner tests, runtime registry tests (including race and concurrency), controller rotation tests, router projection tests, and frontend Vitest tests.
- Full Go build, vet, `./...` tests, and scoped race tests pass. Frontend lint, Vitest, Vite build, and Storybook build pass.

Remaining production work:

- The 2026-07-14 anonymous-provider paced soak is recorded in `docs/evidence/provider-paced-soak-2026-07-14.json`:
  - Kilo completed 4/6 requests, then hit an upstream shared OpenRouter free-model rate limit and recovered to `healthy` after cooldown.
  - OVH completed 1/3 requests, honored upstream `Retry-After`, and remained `rate_limited` on the recovery probe.
  - Treat anonymous Kilo/OVH capacity as opportunistic only. Before sustained production traffic, repeat the soak with intended credentials or paid quota.
- The catalog store is designed for the current single-process SQLite deployment. A future multi-instance deployment should add database-level compare-and-swap or leader ownership for catalog publication.

- The 2026-07-14 anonymous-provider paced soak is recorded in `docs/evidence/provider-paced-soak-2026-07-14.json`:
  - Kilo completed 4/6 requests, then hit an upstream shared OpenRouter free-model rate limit and recovered to `healthy` after cooldown.
  - OVH completed 1/3 requests, honored upstream `Retry-After`, and remained `rate_limited` on the recovery probe.
  - Treat anonymous Kilo/OVH capacity as opportunistic only. Before sustained production traffic, repeat the soak with intended credentials or paid quota.
- The next resilience slice should rotate Kilo catalog models on model-specific 429 responses and lower the automatic-routing score of providers that remain rate-limited across cooldown windows.
- The catalog store is designed for the current single-process SQLite deployment. A future multi-instance deployment should add database-level compare-and-swap or leader ownership for catalog publication.

## Completed Architecture Phase 1 (2026-07-14)

Status markers (`CURRENT` / `TARGET` / `DEFERRED` / `UNVERIFIED`) have been added to all delivery documents under `delivery/`:

| Document | Markers Added | CURRENT | TARGET | DEFERRED | UNVERIFIED |
|----------|--------------|---------|--------|----------|------------|
| `INDEX.md` | 8 | 8 | 0 | 0 | 0 |
| `高层架构设计.md` | 125 | 104 | 5 | 16 | 0 |
| `系统设计.md` | 225 | 161 | 61 | 3 | 0 |
| `部署设计.md` | 318 | 33 | 268 | 17 | 0 |
| `安全设计.md` | 403 | 202 | 201 | 0 | 0 |

Commit: `93f119a` — `docs: add CURRENT/TARGET/DEFERRED/UNVERIFIED status markers to delivery docs`

Key reclassifications:
- Single-instance SQLite, Kilo model-level 429 rotation, existing fallback panels → `CURRENT`
- WAF, cloud VPC, KMS/Vault, SSM, CI/CD Stage 7~8, Prometheus/Grafana surplus check → `TARGET`
- Redis, multi-instance, multi-region, elastic scaling → `DEFERRED`
- RPO ≤ 24h / RTO ≤ 30min, log retention ≥ 180 days, cloud vendor selection → `TARGET` (not yet acceptance-tested)
- Fixed public egress IP → `UNVERIFIED` (depends on upstream whitelist requirements)

These markers distinguish what is already running from what is planned but not yet implemented, preventing future sessions from mistaking architecture targets for deployed capabilities.

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

- Frontend install, ESLint, Vitest, Vite production build, and Storybook build.
- Full Go tests and Go binary build.
- Repository whitespace/conflict checks.
- Playwright desktop/mobile E2E against a CI-built frontend and Go server.
- Go vet plus scoped race tests for fallback, controller, middleware, common, and router packages. CI run `29308699282` passed for commit `4170fa8`.

## PR 检查清单

合并前必须通过以下检查：

### 并发 / 数据一致性
- 对共享可变状态（map、计数器、缓存）必须使用 `sync.Mutex` / `RWMutex` 或 `atomic`。
- 对数据库行的数值累加（如 token 用量、请求计数）优先使用原子 `UPDATE ... SET col = col + ?`，避免在应用层读-改-写后 `Save`。
- 多步 Redis 操作（限流、窗口判断 + 写入）应合并为 Lua 脚本，消除 TOCTOU。
- 修改 `fallback/state.go` 后，本地运行 `go test -race ./fallback -count=5` 必须全部通过。

### 错误处理契约
- 禁止在生产代码中使用 `fmt.Println` 输出错误；统一使用 `common.SysLog` / `common.SysError` 或返回结构化 JSON 错误。
- HTTP 错误响应必须返回结构化 JSON（`{success:false, message: "..."}`），尤其 429 / 500 / 503 等网关错误。
- 不要吞掉 upstream 错误信息；如需脱敏，保留错误类别和可追踪 ID。

### 测试与 CI
- 新增并发或状态相关代码必须附带回归测试。
- CI 中的 `go test -race` 不得跳过；若新增包需要纳入 race 检查，同步更新 `ci.yml` 的 scoped 包列表。
- 前端改动需通过 `npm run lint`、`npm test`、`npm run build` 和 Playwright E2E。

### 分支与提交
- 使用 conventional commits 格式（`fix:`、`feat:`、`docs:`、`test:`、`refactor:`）。
- 功能分支合并后立即删除远端分支。
- 禁止直接向 `main` 提交未经 review 的代码；所有改动通过 PR / 独立功能分支合并。

### 结构关注点
- 控制器只负责编排（请求捕获、转换、调用 relay、返回响应），不重复实现路由、计费、重试、熔断逻辑。
- 协议翻译和兼容性代码放在 `relay/model` 边界，不渗入 controller。
- 不要在 dashboard / overview 添加独立的 connectivity-test 面板；扩展现有虚拟模型配置模块。

### Free Pool 文案规范
- 用户界面保持中文。
- 保留提供商品牌名和技术缩写（RPM、RPD、TPM、TPD、JSON、API key、token）。


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
