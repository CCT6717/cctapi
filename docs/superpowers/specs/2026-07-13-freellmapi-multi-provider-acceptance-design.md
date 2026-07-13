# FreeLLMAPI Multi-Provider Acceptance Design

## Goal

Prove that cctapi's FreeLLMAPI-inspired runtime works across multiple real
providers, not only one OpenRouter deployment, while preserving the current
`openrouter/auto` public model and all existing manual channels.

## Scope

This acceptance slice enables the built-in keyless `kilo` and `ovh` providers
alongside the existing OpenRouter provider. All generated deployments remain in
the `free` pool and continue to be reached through `openrouter/auto`.

The slice validates:

- provider sync and generated channel/deployment ownership;
- real health probes and surfaced provider errors;
- cross-provider fallback after a deployment is cooled down;
- sticky selection after a successful fallback and sticky recovery afterward;
- non-stream and stream chat completions;
- the current Responses and Anthropic Messages compatibility paths;
- cleanup of temporary local API tokens and acceptance-only state.

Kimi Code channel `#22` remains a standalone manual channel. It is not a free
provider and must not be added to the `free` pool in this slice.

## Approaches Considered

### A. Kilo plus OVH keyless providers

Enable both built-in keyless providers. This is the selected approach because
it proves provider-level fallback without requiring another credential and
gives the router at least three independent provider choices.

### B. Kilo only

This changes less configuration, but it provides weaker evidence: one added
provider cannot distinguish deployment switching from robust provider-level
fallback as clearly.

### C. Wait for another paid-provider credential

This may be closer to a future production mix, but it blocks current acceptance
and adds credential-management work that is unnecessary for the routing proof.

## Architecture

No new serving process or routing layer is introduced. Existing ownership
boundaries remain unchanged:

- `fallback` owns provider metadata, generated resources, routing state,
  cooldowns, health, quota checks, and sticky state.
- `controller/relay.go` orchestrates attempts through the existing fallback
  plan.
- relay adaptors own protocol conversion and upstream request/response handling.
- fallback admin APIs expose safe projected configuration and runtime status.
- `data/fallback.json` remains ignored runtime configuration and never enters a
  commit.

Configuration changes go through `PUT /api/fallback/gateway/config` so the
existing merge and backup behavior preserves the OpenRouter key. The request
adds only `kilo` and `ovh` with `enabled: true` and no keys. The API-created
backup path is recorded before acceptance traffic begins.

## Data Flow

1. Read the safe gateway projection and record the current virtual models,
   deployments, provider key counts, and runtime state.
2. Add keyless Kilo and OVH provider inputs through the gateway config API.
3. Trigger free-pool sync and dynamic/static model refresh.
4. Confirm generated channels and deployments exist in the `free` pool without
   changing the OpenRouter channel key or Kimi channel `#22`.
5. Run direct deployment health checks and classify each provider as usable or
   isolated with a surfaced reason.
6. Send gateway traffic through `openrouter/auto`, record the chosen real model
   and deployment, then cool that deployment and repeat the request.
7. Confirm a different provider succeeds, becomes sticky, and remains selected
   until recovery changes the available order.
8. Exercise chat, Responses, and Messages compatibility using temporary,
   model-scoped local tokens that are deleted in a `finally` cleanup path.

## Failure Isolation And Rollback

- An upstream 401/403, 429, 5xx, timeout, malformed response, or unsupported
  protocol is evidence about that provider/path, not permission to weaken
  validation globally.
- A failing provider is cooled down or disabled; healthy existing providers
  remain enabled.
- If configuration sync corrupts ownership, removes the OpenRouter deployment,
  changes its key count, or affects Kimi channel `#22`, restore the API-created
  backup immediately and stop acceptance.
- Manual cooldowns created by the test are cleared during cleanup.
- Temporary API tokens are always deleted, even when a request fails.
- Real keys, cookies, passwords, response bodies containing sensitive data, and
  external host details are excluded from committed evidence.

## Test Strategy

### Configuration acceptance

- Safe projection shows OpenRouter key count unchanged.
- Kilo and OVH are enabled with zero keys.
- Sync creates at least one generated deployment for each usable provider.
- All generated resources stay in pool `free`.

### Routing acceptance

- A baseline `openrouter/auto` request succeeds through one provider.
- Cooling the selected deployment forces a successful request through a
  different provider.
- The new provider becomes sticky for `openrouter/auto`.
- Recovering the cooled deployment leaves all deployments available and no
  acceptance-only cooldown behind.

### Protocol matrix

- `/v1/chat/completions`: non-stream and stream.
- `/v1/responses`: non-stream and stream.
- `/v1/messages`: non-stream and stream when supported by the selected route.
- One structured tool call through chat or Responses verifies schema-gated
  tool-argument repair without relying on ordinary text rescue.

### Automated regression policy

Runtime-only configuration changes need no production-code test. If acceptance
reveals a cctapi defect, first add a focused failing Go or Vitest test, verify
the expected RED state, implement the smallest fix, and run the focused and
full suites before committing.

## Success Criteria

The slice is accepted when:

- OpenRouter remains configured with its original key count;
- Kilo and OVH configuration is synchronized without affecting Kimi channel
  `#22`;
- at least two distinct providers complete real gateway requests;
- forced fallback and sticky behavior are observed through runtime APIs/logs;
- chat stream and non-stream pass;
- Responses and Messages outcomes are recorded accurately, with unsupported
  cases kept as explicit gaps rather than hidden;
- temporary tokens and manual cooldowns are removed;
- `git diff --check`, relevant tests, and the final repository status are clean;
- sanitized acceptance evidence is written under `docs/evidence/`.

## Out Of Scope

- Moving Kimi Code into a virtual model or the free pool.
- Adding paid-provider credentials.
- Renaming the existing `openrouter/auto` public model.
- Building persistent analytics tables or a new dashboard.
- Implementing the full OpenAI Responses object lifecycle.
- Broad relay or fallback refactoring unrelated to failures found by this
  acceptance run.
