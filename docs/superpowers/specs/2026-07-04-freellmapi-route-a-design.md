# FreeLLMAPI Route A Design

## Context

cctapi already has a native free pool foundation: built-in free provider
metadata, automatic channel/deployment sync, dynamic model fetch, a safe
gateway catalog projection, provider quirks, fallback routing, health state,
and a persistent free-provider usage ledger.

FreeLLMAPI's upstream value proposition is broader: a single OpenAI-compatible
surface, `/v1/responses` compatibility for current OpenAI/Codex clients,
encrypted key storage, signed catalog sync, per-key quota learning, health
checks, and analytics. Route A is the first compatibility slice. It makes cctapi
directly usable by more FreeLLMAPI-style clients and makes the existing free
pool safer to operate, without introducing a second proxy service or replacing
the current fallback gateway.

## Goals

- Add a cctapi-native `/v1/responses` shim over the existing chat/fallback path.
- Keep `cct/free`, `cct/high`, and `cct/low` behavior unchanged.
- Harden free-provider save semantics so invalid provider config is rejected
  before sync time.
- Add explicit key replacement and key deletion semantics without ever returning
  raw keys through gateway responses.
- Surface existing free-provider ledger data through an admin API and UI.
- Improve free-provider health probing so keyless providers and provider quirks
  are handled consistently.

## Non-Goals

- Do not add a second FreeLLMAPI proxy process.
- Do not implement Assistants, Threads, files, fine-tuning, or vector stores.
- Do not build the encrypted key store in this slice. Existing plaintext
  `free_providers.keys` remains readable for compatibility until the secret
  store migration slice.
- Do not build signed remote catalog sync in this slice. The current registry
  and provider `/models` sync remain the source of provider metadata.
- Do not add image input support to `/v1/responses` in this slice. Image input
  continues to use `/v1/chat/completions`.

## Architecture

Route A keeps the existing One API relay as the execution path. `/v1/responses`
will translate client requests into the existing chat completion request model,
call the same `controller.Relay` path when possible, and translate the result
back into Responses-shaped JSON or SSE events. This preserves fallback routing,
quota checks, provider quirks, usage accounting, and billing behavior.

Free-provider configuration hardening remains in the gateway layer because it is
the boundary where operator intent is converted into `fallback.Config`. The
fallback package keeps provider validation helpers and runtime behavior. The UI
will call the same gateway endpoints, so backend validation is authoritative.

Free-provider usage analytics will reuse `fallback.FreeProviderUsageLedger`.
The first UI is read-only and scoped to provider, key hash, model, period,
request count, success count, and token totals. It does not expose raw keys.

Health probe changes stay in `fallback` to avoid import cycles. The design adds
small provider-neutral helpers for request URL/header construction and reuses
`common/freeproviderquirks` for quirk metadata. It does not import relay
adaptors into fallback.

## Components

### Responses API

- Add `relaymode.Responses`.
- Register `POST /v1/responses` in `router.SetRelayRouter`.
- Add request/response structs in a focused relay model file.
- Convert supported Responses inputs into `GeneralOpenAIRequest`:
  - string input becomes one user message;
  - message-style input preserves role/content;
  - `model`, `instructions`, `temperature`, `top_p`, `max_output_tokens`,
    `tools`, `tool_choice`, `parallel_tool_calls`, and `stream` map into the
    existing chat request where supported;
  - unsupported fields are ignored only when they are metadata or storage flags;
  - unsupported content types return a clear 422 error.
- Convert non-stream chat responses into a Responses object with:
  - `id`, `object`, `created_at`, `status`, `model`;
  - `output` containing one assistant message;
  - `usage` mapped from chat usage.
- Convert streaming chat chunks into Responses SSE events:
  - `response.created`;
  - `response.output_text.delta` for text deltas;
  - tool-call deltas when present;
  - `response.completed`;
  - `response.failed` on relay errors.

### Free Provider Save Validation

- Validate every incoming free provider name with `fallback.ValidateFreeProviderName`.
- Reject negative limits through the existing limit validator.
- Reject `enabled=true` for providers that require keys when the saved provider
  has no keys and the request supplies no replacement keys.
- Allow keyless providers to be enabled with no keys.
- Preserve existing keys when the request omits `keys`.
- Replace keys when the request supplies a non-empty `keys` array.
- Add `clear_keys: true` to explicitly delete stored provider keys.
- Reject requests that set both `clear_keys: true` and non-empty `keys`.
- Do not expose raw keys in GET responses, backups, logs, docs, or tests.

### Usage API and UI

- Add admin API `GET /api/fallback/free-pool/usage`.
- Query parameters:
  - `provider` optional;
  - `key_hash` optional;
  - `model` optional;
  - `period` optional, defaulting to the current free-provider period.
- Response rows include provider, key hash, model name, period, prompt tokens,
  completion tokens, total tokens, request count, success count, and updated time.
- Add a compact read-only usage table to the Free Pool page.
- The UI must display key hashes only, never raw keys.

### Health Probe Improvements

- Build free-provider probe URLs from the configured channel base URL and
  channel type with the same `/v1` suffix rules already used for
  OpenAI-compatible channels.
- For keyless providers, omit the Authorization header.
- Apply `DefaultUserAgent` quirks from `common/freeproviderquirks`.
- Apply `DisableStream`, `MaxOutputTokens`, and `DropStop` to the probe body
  where relevant.
- Keep the fallback package independent from relay adaptors.
- Preserve existing health states: `healthy`, `rate_limited`, `invalid`,
  `error`, and `unknown`.

## Data Flow

1. A client calls `/v1/responses`.
2. The router authenticates the token and distributes as usual.
3. The Responses shim converts the request to the chat request shape.
4. Existing fallback planning selects a deployment and channel.
5. Existing relay/adaptor code sends the provider request.
6. Existing success and usage hooks record deployment usage and free-provider
   ledger rows.
7. The shim converts the chat result or stream back to Responses shape.

For free-provider config saves:

1. The UI submits `free_providers`.
2. Gateway validation rejects unknown providers, invalid limits, ambiguous key
   operations, and enabled requires-key providers with no available key.
3. Valid config merges into the live fallback config.
4. Free pool sync creates, updates, disables, or removes auto resources.

## Error Handling

- `/v1/responses` unsupported content returns 422 with a clear message.
- `/v1/responses` relay errors preserve the existing error status where possible.
- Streaming errors emit `response.failed` before the stream closes when headers
  have already been sent.
- Gateway save validation errors return 400 and do not mutate config.
- Usage API returns an empty list for no rows and 500 only for database errors.
- Health probe network errors mark `error` and short cooldown, as today.
- 401/403 health responses mark `invalid`.
- 429 health responses mark `rate_limited`.

## Testing

Use TDD for each behavior change.

Backend tests:

- `router` test: `POST /v1/responses` is registered and reaches the relay path.
- Responses translation tests for string input, message input, instructions,
  max output tokens, tools, non-stream output, and unsupported image input.
- Streaming translation tests for text delta, tool-call delta, completion event,
  and failure event.
- Gateway validation tests for unknown providers, requires-key enabled without
  keys, keyless enabled without keys, key replacement, key preservation, and
  `clear_keys`.
- Usage API tests for aggregation filtering and no raw keys.
- Health probe tests for keyless auth omission, user-agent quirk, max token
  clamp, and OpenAI-compatible `/v1` suffix behavior.

Frontend tests:

- Free provider rows can show usage data with key hash only.
- Key clearing requires the explicit clear action.
- Provider model and key controls preserve existing keys unless the operator
  replaces or clears them.

Verification commands:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
go test ./fallback ./router ./controller ./relay/... -count=1
go test ./... -count=1
go build -o one-api.exe
```

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false
npm run build
```

## Rollout

- Implement in small commits on `cleanup/structure-boundaries`.
- Keep all new behavior behind existing routes and admin auth; no new public
  unauthenticated endpoints.
- Preserve old free-provider config files.
- Add docs and smoke coverage after the backend and UI paths pass.
- Push the branch after full verification.

## First Implementation Plan Shape

1. Responses non-stream request/response translation.
2. Responses streaming translation.
3. Free-provider validation and explicit key clearing.
4. Free-provider usage API and UI table.
5. Health probe hardening.
6. Docs, smoke checks, full verification, and final review.
