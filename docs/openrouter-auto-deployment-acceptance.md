# OpenRouter/auto Deployment Acceptance Gate

Use this document as the deploy-time reusable checklist after environment restore or re-provisioning.

## Scope

- Reconcile DB/provider drift after moving to a different machine.
- Validate `openrouter/auto` end-to-end through free-pool.
- Verify both control-plane recovery and data-plane counters after `free-pool` sync.

## One command (repeatable)

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"

powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

## Mandatory pre-checks

- Service reachable: `curl.exe -I http://127.0.0.1:3008/`
- User/admin tokens are valid for the target environment.
- Upstream `openrouter` credentials are configured in `free_provider` settings.

## Acceptance requirements (must all pass)

- `openrouter` appears in `free_provider_catalog` and is enabled.
- `POST /api/fallback/config/reload` returns success.
- `POST /api/fallback/free-pool/sync` returns success.
- `GET /api/fallback/deployments/runtime-status` contains at least one deployment with prefix `free:openrouter-`.
- `POST /v1/chat/completions` with `model=openrouter/auto` returns non-stream success and `choices`.
- Stream request for `model=openrouter/auto` returns SSE text (`data:`).
- `/api/fallback/free-pool/usage?provider=openrouter` returns success, has provider row data, and shows positive request/success deltas.
- `/metrics` shows `fallback_requests_total` increased.
- `/fallback/free-pool` returns HTTP 2xx (`pageReachable=true`).

The provider catalog and `free:openrouter-` runtime row checks prove OpenRouter configuration. `pageContainsOpenRouterAuto` is informational because raw SPA shell HTML may not include authenticated route data; perform an authenticated browser check only when visible UI validation is required.

## Required evidence from `-OutputJson`

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
usageRequestDelta
usageSuccessDelta
fallbackRequestsDelta
pageReachable
pageContainsOpenRouterAuto
```

## Failure triage before release

1. If runtime rows exist but routing still fails:
   - Check that the restored DB rows for `fallback_provider_*` and related mappings are consistent.
2. If stream or non-stream fails:
   - Confirm both tokens and provider key availability on this host.
3. Re-run `free-pool` sync immediately once DB restore completes and rerun this checklist.

## Copy-paste final submission block

- Execution time: `YYYY-MM-DD HH:mm:ss`
- Branch/commit: `main / <commit>`
- Host: `<hostname>`
- Operator: `<operator>`
- Final result: `PASS` or `FAIL`
- `pass`: `true` / `false`
- `deploymentId`: `<value>`
- `runtimeRows`: `<number>`
- `usageRowsBefore`: `<number>`
- `usageRowsAfter`: `<number>`
- `usageRequestCount`: `<number>`
- `usageSuccessCount`: `<number>`
- `usageRequestDelta`: `<positive number>`
- `usageSuccessDelta`: `<positive number>`
- `fallbackRequestsDelta`: `<number>`
- `pageReachable`: `true` / `false`
- `pageContainsOpenRouterAuto`: `true` / `false`
- Remarks: `<short notes>`
