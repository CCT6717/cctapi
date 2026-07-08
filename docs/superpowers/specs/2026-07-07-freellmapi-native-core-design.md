# FreeLLMAPI Native Core Design

## Goal

Bring the useful FreeLLMAPI runtime behaviors into cctapi without embedding the
FreeLLMAPI Node service or replacing cctapi's relay/fallback core.

## Scope

This branch treats cctapi as the only serving process. FreeLLMAPI is used as a
reference implementation for compatibility behavior, provider quirks, routing
guards, and client-facing reliability fixes.

The first implementation slice focuses on tool-call compatibility because it is
small, testable, and directly affects Codex, Claude Code, and OpenAI-compatible
agents:

- Repair double-encoded tool-call arguments in non-stream chat responses.
- Reuse the same repair before Responses and Claude-format conversions see the
  response body.
- Keep repair schema-gated, so string parameters that merely look like JSON are
  not changed.
- Leave normal text, valid tool arguments, streaming passthrough, and upstream
  retry behavior unchanged in this slice.

## Out Of Scope For The First Slice

- Copying FreeLLMAPI's SQLite schema or dashboard into cctapi.
- Replacing cctapi fallback deployment ordering.
- Adding encrypted provider-key storage.
- Implementing a second proxy/router service.
- Reinterpreting ordinary assistant text as tool calls. Inline tool-call rescue
  is valuable, but it is riskier and should be a follow-up slice after the
  structured argument repair is stable.

## Architecture

Add pure Go helpers under `relay/model` for tool schema lookup and argument
repair. The helpers accept cctapi's existing `[]model.Tool` request shape and a
raw chat-completion JSON response body. They return either the original body or
a repaired body with only `choices[].message.tool_calls[].function.arguments`
changed.

`controller.RelayTextHelper` records the request tools in the Gin context before
dispatching to the adaptor. `relay/adaptor/openai.Handler` reads that context
value after parsing the upstream response, repairs the raw response body when
needed, and writes the repaired body to the client. This keeps controllers thin
and keeps the compatibility behavior in relay/model plus adaptor output
handling.

## Data Flow

1. Client sends OpenAI, Responses, or Anthropic-compatible request with tools.
2. cctapi converts the client request to the internal OpenAI chat request.
3. The chat request's tool schemas are stored in request context for this relay
   attempt.
4. Upstream returns an OpenAI-style chat completion.
5. Before the response is emitted or captured for another compatibility layer,
   tool-call `arguments` strings are repaired against the original request
   schema.
6. The repaired response continues through the existing OpenAI, Responses, or
   Claude output path.

## Error Handling

Repair must be fail-open. If the response body cannot be parsed, if a tool call
does not match a known schema, or if a nested string is not valid JSON of the
expected schema type, the original response body is preserved.

The repair function must never turn a valid upstream response into a relay
error. It should only return an error for JSON marshal/unmarshal failures that
already indicate the raw body is malformed in a way the existing handler would
also reject.

## Tests

Add focused tests in `relay/model`:

- Whole-argument double encoding is unwrapped.
- Schema-gated nested arrays and objects are repaired.
- String-typed parameters are preserved.
- Invalid JSON and unknown tools are preserved.
- A raw chat-completion response body is repaired without dropping unrelated
  response fields.

Add adaptor/controller coverage if needed to prove the repaired body is the one
written to clients.

## Later Slices

After the first slice is stable, continue with:

- Inline tool-call dialect rescue from FreeLLMAPI's `tool-call-rescue.ts`.
- Context handoff for model switches inside fallback sessions.
- More explicit model capability metadata for tools and vision.
- Clearer all-exhausted diagnostics surfaced through existing fallback runtime
  APIs.
