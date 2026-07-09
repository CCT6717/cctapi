# OpenRouter/auto Stability Acceptance Submission

Purpose:
Validate runtime recovery of `openrouter/auto` after deployment (database/channel configuration re-sync consistency).

Execution environment:

- Service URL: `http://localhost:3008`
- Target model: `openrouter/auto`
- Tool: `scripts/fallback-openrouter-auto-smoke.ps1`

One-command run:

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"

powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

Pass criteria (required):

1. `openrouter` exists in `free_provider_catalog` and is enabled.
2. `config/reload` and `free-pool/sync` return success (`HTTP 2xx` and `success=true`).
3. At least one runtime row exists for `free:openrouter-*`.
4. Non-stream `POST /v1/chat/completions` with `model=openrouter/auto` returns `choices`.
5. Streaming request returns SSE chunks with `data:`.
6. `/api/fallback/free-pool/usage?provider=openrouter` returns success and updated counters.
7. `/metrics` shows increased `fallback_requests_total`.
8. `/fallback/free-pool` page shows OpenRouter auto path available.

Submission note template:

- Time: `<YYYY-MM-DD HH:mm>`
- Host: `<machine-name>`
- Operator: `<operator>`
- Result:
  - Non-stream: `pass/fail`
  - Stream: `pass/fail`
  - usage rows: `<count>`
  - request_count: `<number>`
  - success_count: `<number>`
  - fallback_requests_total delta: `<delta>`
  - deploymentId: `<value>`
  - runtimeRows: `<count>`
- Conclusion: `PASS` / `FAIL`

Recommended follow-up:

- If openrouter routing succeeds but usage row count is unchanged, verify provider row replacement logic in `/api/fallback/free-pool/usage`.
- If openrouter/auto fails, verify `fallback_*` DB rows (provider/provider_key mapping) match the deployed machine.
- Run this script once after each environment restore before release.
