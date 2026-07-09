# OpenRouter/auto Acceptance Checklist

Use this checklist to validate `openrouter/auto` through free-pool sync and request flow.

- Service URL: `http://localhost:3008`
- Target model: `openrouter/auto`

## Run Command

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"

powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

Requirements:

- `CCT_ADMIN_TOKEN`: admin token
- `CCT_API_TOKEN`: user token
- Valid `openrouter` credentials available to running service
- Service reachable at `http://localhost:3008/`

## Must-Pass Checklist

- [ ] `free_provider_catalog` has `openrouter` and it is enabled.
- [ ] `POST /api/fallback/config/reload` returns success.
- [ ] `POST /api/fallback/free-pool/sync` returns success.
- [ ] `GET /api/fallback/deployments/runtime-status` has at least one `free:openrouter-` deployment row.
- [ ] `POST /v1/chat/completions` with `model=openrouter/auto` returns HTTP 200 and has `choices`.
- [ ] Streaming request for `model=openrouter/auto` returns SSE content (`data:`).
- [ ] `/api/fallback/free-pool/usage?provider=openrouter` returns success and updates usage counters.
- [ ] `/metrics` shows increased `fallback_requests_total`.
- [ ] `/fallback/free-pool` page shows OpenRouter auto routing enabled.

## Optional Follow-up Checks

- Runtime row for `free:openrouter-*` stays `enabled=true` after key rotations.
- Usage payload includes key hash metadata (`key_hash`) for deeper diagnosis.
- Restart service and rerun the script once to confirm idempotence.

## Result Template

Sample output when accepted:

```text
Run time: <ISO timestamp>
Model non-stream: pass/fail
Model stream: pass/fail
runtime rows: N
usage rows: yes (request_count: X)
fallback_requests_total delta: +Y
Evidence: passed / failed
```

When running with `-OutputJson`, expected keys include:

```text
pass
baseUrl
model
deploymentId
usageRowsBefore
usageRowsAfter
runtimeRows
usageRequestCount
usageSuccessCount
fallbackRequestsDelta
pageContainsOpenRouterAuto
```
