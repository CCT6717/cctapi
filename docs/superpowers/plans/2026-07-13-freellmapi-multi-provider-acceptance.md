# FreeLLMAPI Multi-Provider Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable Kilo and OVH beside OpenRouter and produce repeatable evidence that cctapi can route, fail over, stick, recover, and translate supported client protocols across real providers.

**Architecture:** Use the existing safe gateway configuration API to preserve provider secrets and create a runtime backup. Existing fallback sync owns generated channels and deployments; acceptance traffic uses temporary local tokens and observes safe runtime, usage, and log APIs. No production code changes are expected; any defect found must enter a separate RED-GREEN fix cycle before this plan continues.

**Tech Stack:** Go cctapi service on Windows, PowerShell 5.1-compatible acceptance commands, Gin admin APIs, ignored `data/fallback.json`, SQLite runtime state.

## Global Constraints

- Operate on the live checkout at `D:\ct\project` and service at `http://127.0.0.1:3008` because the target is ignored runtime configuration.
- Preserve the existing OpenRouter provider key count and manual Kimi Code channel `#22`.
- Never print, commit, or persist provider keys, local API tokens, admin passwords, cookies, or unsanitized response bodies.
- Keep Kimi Code outside the free pool.
- Remove temporary API tokens and acceptance-only manual cooldowns in `finally` cleanup paths.
- If a cctapi code defect is found, stop runtime acceptance, add a focused failing test, verify RED, implement the minimum fix, and verify GREEN before resuming.
- Do not claim a provider or protocol passes without a fresh real request and safe observable evidence.

---

### Task 1: Capture Baseline And Rollback State

**Files:**
- Read: `data/fallback.json`
- Read: `router/fallback_gateway.go`
- Create at completion: `docs/evidence/freellmapi-multi-provider-acceptance-2026-07-13.json`

**Interfaces:**
- Consumes: `GET /api/fallback/gateway/config`, `GET /api/fallback/deployments/runtime-status`, channel search APIs.
- Produces: an in-memory safe baseline with provider key counts, deployment IDs, channel `#22` fields, and repository HEAD.

- [ ] **Step 1: Verify the live service and clean repository**

Run:

```powershell
git status --porcelain
curl.exe -s -o NUL -w '%{http_code}' http://127.0.0.1:3008/
curl.exe -s -o NUL -w '%{http_code}' http://127.0.0.1:3008/fallback/free-pool
```

Expected: empty Git status and HTTP `200` for both URLs.

- [ ] **Step 2: Log in and capture safe projections**

Use a `WebRequestSession` and the credentials supplied through
`CCT_ADMIN_USERNAME` / `CCT_ADMIN_PASSWORD`. Read the gateway projection,
runtime status, and channel `#22`. Store only provider `key_count`, deployment
IDs, health state, channel type/base URL/models/status, and current HEAD.

- [ ] **Step 3: Record the current ignored configuration hash**

Run:

```powershell
Get-FileHash data\fallback.json -Algorithm SHA256
```

Expected: one SHA256 hash. Do not copy the configuration contents into evidence.

### Task 2: Enable Kilo And OVH Through The Safe Config API

**Files:**
- Runtime-only modify: `data/fallback.json` through `PUT /api/fallback/gateway/config`

**Interfaces:**
- Consumes: the full safe gateway projection and `gatewayV2ConfigInput`.
- Produces: enabled `kilo` and `ovh` provider entries with no keys and an API-created backup path.

- [ ] **Step 1: Build a lossless gateway payload**

Copy `enabled`, `virtual_models`, and `deployments` from the safe projection.
Convert each projected provider to an input containing `enabled`, `models`, and
`limits_override`; omit `keys` and leave `clear_keys=false` so existing secrets
are preserved. Add:

```json
{
  "kilo": {"enabled": true, "models": []},
  "ovh": {"enabled": true, "models": []}
}
```

- [ ] **Step 2: PUT the payload and verify ownership invariants**

Run the request against `/api/fallback/gateway/config`, require `success=true`,
and retain the returned `backup_path`. Re-read the safe projection and assert:

- OpenRouter `key_count` equals the baseline.
- Kilo and OVH are enabled with `key_count=0`.
- Channel `#22` is unchanged.

- [ ] **Step 3: Trigger free-pool sync**

POST `/api/fallback/free-pool/sync` and require `success=true`. Re-read gateway
configuration and assert at least one `free:kilo-*` and one `free:ovh-*`
deployment exist in pool `free`.

### Task 3: Probe Provider Health And Isolate External Failures

**Files:**
- No repository files modified.

**Interfaces:**
- Consumes: generated Kilo and OVH deployment IDs.
- Produces: provider health result, response time, safe error category, and cooldown state.

- [ ] **Step 1: Trigger each generated deployment health check**

POST `/api/fallback/deployments/{id}/health-check` for every Kilo and OVH
deployment. Record `success`, health enum, response time, HTTP/error category,
and whether a cooldown was applied. Do not store full upstream bodies.

- [ ] **Step 2: Classify usable providers**

A provider is usable only when a fresh check returns healthy or a real gateway
request succeeds. A provider that returns auth, rate limit, timeout, malformed
response, or 5xx stays configured but cooled/isolated with the surfaced reason.

- [ ] **Step 3: Confirm at least two providers are candidates**

Require at least two usable providers for forced failover. If fewer than two
are usable, preserve the configuration and record an external acceptance gap;
do not weaken validation or fabricate a pass.

### Task 4: Run Forced Fallback, Sticky, And Recovery Acceptance

**Files:**
- No repository files modified.

**Interfaces:**
- Consumes: temporary token scoped to `openrouter/auto`, runtime status, switch logs.
- Produces: baseline provider, fallback provider, sticky provider, and cleanup receipt.

- [ ] **Step 1: Create a temporary model-scoped token**

POST `/api/token/` with name `verify-freellmapi-multiprovider`, unlimited quota,
expiry `-1`, and models `openrouter/auto`. Keep the returned key only in memory.

- [ ] **Step 2: Send baseline chat traffic**

POST one non-stream and one stream request to `/v1/chat/completions`. Require
HTTP 200, a choice/content event, terminal stream marker, and usage visibility.
Read runtime/switch logs to identify the successful deployment and provider.

- [ ] **Step 3: Force provider-level fallback**

POST a 300-second manual cooldown for the selected deployment. Send another
non-stream and stream request. Require a successful deployment whose provider
differs from the baseline provider, then verify it is sticky for
`openrouter/auto`.

- [ ] **Step 4: Verify sticky reuse and recovery**

Send a third request and require the sticky deployment to remain selected.
Recover all acceptance-cooled deployments and assert no manual cooldown remains.

- [ ] **Step 5: Delete the temporary token**

Delete the token in `finally`, then search by exact prefix and require zero
remaining tokens.

### Task 5: Run The Real Protocol Compatibility Matrix

**Files:**
- No production file modified unless a defect enters the TDD policy.

**Interfaces:**
- Consumes: temporary token scoped to `openrouter/auto` and currently usable provider routes.
- Produces: six protocol results plus one structured tool-call result.

- [ ] **Step 1: Chat completions**

Run non-stream and stream `/v1/chat/completions` requests. Require valid JSON for
non-stream and useful SSE plus a terminal event for stream.

- [ ] **Step 2: Responses compatibility**

Run non-stream and stream `/v1/responses` requests using string input. Require a
valid Responses object for non-stream and ordered useful SSE events ending in a
completed event for stream.

- [ ] **Step 3: Anthropic Messages compatibility**

Run non-stream and stream `/v1/messages` requests with `anthropic-version` and
`x-api-key`. Require a valid Anthropic message object or useful Anthropic SSE.
Record an explicit unsupported/provider-specific gap if the selected route
cannot satisfy the protocol.

- [ ] **Step 4: Structured tool call**

Send one tool schema whose object argument contains a nested array. Accept only
a structured tool call with JSON arguments matching the schema; ordinary text
that describes a call is not a pass.

- [ ] **Step 5: Cleanup**

Delete the temporary protocol token and require zero matching tokens.

### Task 6: Verify, Archive Evidence, And Integrate

**Files:**
- Create: `docs/evidence/freellmapi-multi-provider-acceptance-2026-07-13.json`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: sanitized results from Tasks 1-5.
- Produces: durable acceptance evidence and current handoff guidance.

- [ ] **Step 1: Write sanitized evidence**

Record commit, configuration hashes before/after, API-created backup path,
provider/deployment counts, health categories, routing transitions, protocol
matrix outcomes, cleanup counts, and external gaps. Exclude all credentials,
cookies, full prompts, and unsanitized upstream bodies.

- [ ] **Step 2: Update AGENTS.md**

Document enabled providers, real forced-fallback outcome, protocol coverage,
known external failures, rollback location, and the exact next task. Do not
claim unsupported paths pass.

- [ ] **Step 3: Run repository verification**

Run:

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go1.22.12\bin\go.exe' test ./fallback ./controller ./router ./relay/model -count=1
git diff --check
git status --short
curl.exe -s -o NUL -w '%{http_code}' http://127.0.0.1:3008/fallback/free-pool
```

Expected: tests pass, no whitespace errors, only intended docs are modified,
and the page returns HTTP 200.

- [ ] **Step 4: Review and commit**

Review evidence for secrets, stage only the plan/evidence/AGENTS files and any
TDD-backed defect fix, then commit with a message describing the actual result.

- [ ] **Step 5: Push and verify CI**

Push `main`, wait for the matching GitHub Actions run to finish, and require a
successful conclusion before declaring the slice complete.
