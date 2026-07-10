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
- usageRequestDelta: `<positive number>`
- usageSuccessDelta: `<positive number>`
- fallbackRequestsDelta: `<number>`
- pageReachable: `true` / `false`
- pageContainsOpenRouterAuto: `true` / `false`
- Remarks: `<text>`

`pageReachable` is the mandatory UI-route condition. Provider catalog and runtime rows prove OpenRouter configuration; `pageContainsOpenRouterAuto` is informational, and an authenticated browser check is optional when visible UI validation is required.

## 2. Required evidence from -OutputJson

```json
{
  "pass": true,
  "baseUrl": "http://localhost:3008",
  "model": "openrouter/auto",
  "deploymentId": "...",
  "usageRowsBefore": 1,
  "usageRowsAfter": 1,
  "runtimeRows": 1,
  "usageRequestCount": 7,
  "usageSuccessCount": 6,
  "usageRequestDelta": 2,
  "usageSuccessDelta": 2,
  "fallbackRequestsDelta": 2,
  "pageReachable": true,
  "pageContainsOpenRouterAuto": false
}
```

## 3. Failure triage (if failed)

- Verify both tokens match the active service user/admin permissions.
- Verify `fallback_*` provider/channel rows are restored consistently after migration/restore.
- Restart service and re-run once, capturing delta comparison.
- Confirm provider credential availability and status in upstream service.

## 4. Final submission copy-paste block

```text
Execution time: <YYYY-MM-DD HH:mm:ss>
Host: <machine-name>
Operator: <operator>
Branch/commit: main / <commit>

Result: PASS / FAIL
Non-stream: pass / fail
Stream: pass / fail
deploymentId: <value>
runtimeRows: <number>
usageRowsBefore: <number>
usageRowsAfter: <number>
usageRequestCount: <number>
usageSuccessCount: <number>
usageRequestDelta: <positive number>
usageSuccessDelta: <positive number>
fallbackRequestsDelta: <number>
pageReachable: true / false
pageContainsOpenRouterAuto: true / false
Remarks: <short notes>
```

Recommended attachment:
- raw `-OutputJson` output
- `curl.exe -I http://127.0.0.1:3008/`
- `curl.exe -I http://127.0.0.1:3008/fallback/free-pool`
