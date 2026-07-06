# FreeLLMAPI Route A

Route A adds cctapi-native compatibility for the core FreeLLMAPI workflow while keeping the existing fallback gateway as the only execution path.

## Supported

- `POST /v1/responses` for text-only Responses API clients.
- Streaming Responses SSE translated from chat-completions SSE.
- Explicit free-provider key replacement and `clear_keys`.
- Read-only free-provider usage rows at `GET /api/fallback/free-pool/usage`.
- Provider-aware health probes for keyless providers and request quirks.

## Not Supported In This Slice

- Assistants, Threads, files, fine-tuning, and vector stores.
- Image or file input through `/v1/responses`.
- Signed remote catalog sync.
- Encrypted free-provider key storage.

## Key Save Semantics

- Omitting `keys` preserves stored keys.
- Sending non-empty `keys` replaces stored keys after trimming blank and masked values.
- Sending `clear_keys: true` deletes stored keys.
- Sending `clear_keys: true` with replacement keys is rejected.
- Gateway GET responses and usage APIs never return raw provider keys.

## Usage API

`GET /api/fallback/free-pool/usage` accepts optional `provider`, `key_hash`, `model`, and `period` query parameters. The response contains provider, key hash, model name, period, prompt tokens, completion tokens, total tokens, request count, success count, and timestamps.
