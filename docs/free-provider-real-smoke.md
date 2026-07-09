# Free Provider Real Smoke Checks

Use this checklist to validate the native FreeLLMAPI-style provider pool in cctapi. Keep provider keys in local environment variables or local `data/fallback.json`; never paste keys into docs, screenshots, shell transcripts, or issue comments.

## Metadata-Only Check

This mode requires an admin token but does not send a chat request.

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"
$env:CCT_EXPECTED_PROVIDER = "aihorde"
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1 -FreeProviderCatalogOnly
```

Pass criteria:

- The selected provider appears in `free_provider_catalog`.
- The output shows `enabled`, `key_count`, `keyless`, `requires_key`, `model_fetch_mode`, capabilities, quirks, and effective limits.
- If the provider is enabled and synced, runtime rows exist with deployment IDs like `free:<provider>-<hash>`.
- The output contains provider/deployment IDs only, never raw provider keys.

## Real Request Check

This mode sends a request to `cct/free`. Add `-SkipStream` for providers that intentionally disable upstream streaming, such as AIHorde.

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"
$env:CCT_API_MODEL = "cct/free"
$env:CCT_EXPECTED_PROVIDER = "aihorde"
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1 -SkipStream
```

Pass criteria:

- Catalog inspection succeeds before the chat request.
- Runtime rows matching `free:<provider>-` exist after reload/sync.
- Non-stream chat succeeds.
- Stream succeeds unless `-SkipStream` is used.
- `/metrics` includes fallback counters and shows a request delta.

## Provider Acceptance Matrix

| Provider | Key mode | Fetch mode | Stream test | Notes |
| --- | --- | --- | --- | --- |
| openrouter | key required | OpenRouter free catalog | yes | Also verify credit sync when using real OpenRouter keys. |
| groq | key required | static | yes | Static model list should remain available if upstream listing is unavailable. |
| google | key required | static/native model list | yes | Verify Gemini channel behavior separately when using non-OpenAI-compatible paths. |
| nvidia | key required | OpenAI-compatible models | yes | Runtime quirk forces `parallel_tool_calls=false`. |
| routeway | key required | OpenAI-compatible models | yes | Runtime quirk adds a default User-Agent. |
| aihorde | keyless allowed | OpenAI-compatible models | skip or non-stream only | Runtime quirk disables stream and drops unsupported fields. |
| pollinations | keyless | static | yes | Good first keyless smoke candidate. |
| ovh | keyless | static | yes | Low default limits; keep request volume small. |

### OpenRouter/auto Smoke Check

Run this command to validate `openrouter/auto` end-to-end:
```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"
powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

- `openrouter` catalog check
- `config/reload` + `free-pool/sync`
- `free:openrouter-*` runtime rows
- `openrouter/auto` non-stream + stream chat
- `/metrics` delta and `/free-pool/usage` records

Final acceptance evidence and checklist: see [`docs/openrouter-auto-acceptance-checklist.md`](/D:/ct/project/docs/openrouter-auto-acceptance-checklist.md) and [`docs/openrouter-auto-stability-submission.md`](/D:/ct/project/docs/openrouter-auto-stability-submission.md).

## Safe Operating Rules

- Use `free_provider_catalog` to confirm whether a provider is keyless or key-required before enabling it.
- Run config reload and free-pool sync before expecting runtime rows.
- Verify runtime rows by provider prefix, for example `free:nvidia-` or `free:aihorde-`.
- Keep all real provider keys out of committed files and backup artifacts.
