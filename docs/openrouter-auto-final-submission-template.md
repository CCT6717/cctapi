# OpenRouter/auto Stability Submission (Final)

Use this as the exact submission block after running:

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"

powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

Reference:
- `docs/openrouter-auto-acceptance-checklist.md`
- `docs/openrouter-auto-stability-submission.md`

## 1. Check Result (required)

- Execution time: `YYYY-MM-DD HH:mm:ss`
- Host: `<hostname>`
- Operator: `<operator>`
- Result: `PASS` or `FAIL`
- Non-stream: `pass` / `fail`
- Stream: `pass` / `fail`
- deploymentId: `<value>`
- runtimeRows: `<number>`
- usageRowsBefore: `<number>`
- usageRowsAfter: `<number>`
- usageRequestCount: `<number>`
- usageSuccessCount: `<number>`
- fallbackRequestsDelta: `<number>`
- pageContainsOpenRouterAuto: `true` / `false`
- Remarks: `<text>`

## 2. Required evidence from -OutputJson

```json
{
  "pass": true,
  "baseUrl": "http://localhost:3008",
  "model": "openrouter/auto",
  "deploymentId": "...",
  "usageRowsBefore": 0,
  "usageRowsAfter": 0,
  "runtimeRows": 0,
  "usageRequestCount": 0,
  "usageSuccessCount": 0,
  "fallbackRequestsDelta": 0,
  "pageContainsOpenRouterAuto": true
}
```

## 3. Failure triage (if failed)

- Verify both tokens match the active service user/admin permissions.
- Verify `fallback_*` provider/channel rows are restored consistently after migration/restore.
- Restart service and re-run once, capturing delta comparison.
- Confirm provider credential availability and status in upstream service.

