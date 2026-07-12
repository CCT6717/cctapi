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
6. `/api/fallback/free-pool/usage?provider=openrouter` returns success and both request/success counters have positive deltas.
7. `/metrics` shows increased `fallback_requests_total`.
8. `/fallback/free-pool` returns HTTP 2xx (`pageReachable=true`).

The provider catalog and runtime deployment checks prove OpenRouter configuration. `pageContainsOpenRouterAuto` remains informational because raw SPA shell HTML may omit authenticated route data; use an authenticated browser check only when visible UI validation is required.

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
  - usageRequestDelta: `<positive number>`
  - usageSuccessDelta: `<positive number>`
  - fallback_requests_total delta: `<positive number>`
  - pageReachable: `true/false`
  - pageContainsOpenRouterAuto (informational): `true/false`
  - deploymentId: `<value>`
  - runtimeRows: `<count>`
- Conclusion: `PASS` / `FAIL`

Latest verified run:

- Time: `2026-07-12 22:45 +08:00`
- Host: local Windows acceptance host
- Branch/commit: `main` / `fd1006a`
- Result:
  - Non-stream: `pass`
  - Stream: `pass`
  - resolved model: `nvidia/nemotron-3-ultra-550b-a55b-20260604:free`
  - usage rows before/after: `1 / 1`
  - request_count: `1`
  - success_count: `1`
  - usageRequestDelta: `1`
  - usageSuccessDelta: `1`
  - fallback_requests_total delta: `2`
  - pageReachable: `true`
  - pageContainsOpenRouterAuto (informational): `false`
  - deploymentId: `free:openrouter-1f9cf7de`
  - runtimeRows: `1`
- Conclusion: `PASS`
- Evidence: [`evidence/openrouter-auto-2026-07-12.json`](evidence/openrouter-auto-2026-07-12.json)
- Artifact hashes:
  - `one-api.exe` SHA256: `FA010662AB4C825F2D97A6CBA236BB82A5F21E350FF453E5385A6F26C926526C`
  - `main.3b53fcb6.js` SHA256: `53B74539EBAB5D9C8CFF5C7C73F7BC4A96421B5208AF6415FBBF3793B9FB9E9D`

Post-hardening verification:

- Time: `2026-07-13 00:23 +08:00`
- Branch/commit: `main` / `59ba393`
- Changes under verification: Axios `1.18.1`, committed npm lockfile, reproducible `npm ci`, rebuilt embedded frontend.
- Result: `PASS`
- Evidence: [`evidence/openrouter-auto-2026-07-13.json`](evidence/openrouter-auto-2026-07-13.json)
- Artifact hashes:
  - `one-api.exe` SHA256: `98BAEF8F4694CABB822E163D691CD52EB1E8C17AD3E91FB11D385196D87C9B2E`
  - `main.7b346cd1.js` SHA256: `5958ACBB1DCA3EA4885B123E721AD97EAAF4298100EF2BEB7E875AA0B5F25ABF`

Post-Vite-review verification:

- Time: `2026-07-13 02:14 +08:00`
- Branch/commit: `main` / `c5a0ed2`
- Changes under verification: Vite config hardening, Storybook 10.5.0, strict ESLint, corrected responsive screenshots, Chat translation fix, and rebuilt embedded frontend.
- Result: `PASS`
- Evidence: [`evidence/openrouter-auto-2026-07-13-vite-review.json`](evidence/openrouter-auto-2026-07-13-vite-review.json)
- Artifact hashes:
  - `one-api.exe` SHA256: `01716AB9AB42FDED3B1243F461FF40886E0FF45ACC632B8364ABDC6F77D7BFC4`
  - `index-BZLCYnQY.js` SHA256: `55B50150EBEA6B88ADE4CD6F9F2F3235E558DF31A2104D20F01E7280D36AB916`

Recommended follow-up:

- If openrouter routing succeeds but usage row count is unchanged, verify provider row replacement logic in `/api/fallback/free-pool/usage`.
- If openrouter/auto fails, verify `fallback_*` DB rows (provider/provider_key mapping) match the deployed machine.
- Run this script once after each environment restore before release.
