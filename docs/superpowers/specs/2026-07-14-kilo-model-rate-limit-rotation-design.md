# Kilo Model-Level Rate-Limit Rotation Design

## Goal

Keep a Kilo deployment usable when one catalog model returns HTTP 429. The
gateway should cool down only the affected model, retry another compatible Kilo
model inside the same deployment, and penalize or cool down the provider only
when no compatible model remains available.

## Scope

- Apply model-level rotation only to generated Kilo free-pool deployments.
- Build request-scoped model candidates from the last validated Kilo catalog.
- Preserve the configured deployment model as the first candidate when it is
  compatible and not cooling down.
- Filter every alternative by the request's stream, tools, JSON, vision, and
  context requirements before it can be attempted.
- On a pre-response 429, cool down the attempted model and continue with the
  next compatible model without cooling down the whole deployment.
- When all compatible models are unavailable or return 429, reuse the existing
  deployment cooldown and rate-limit score so automatic routing prefers another
  provider.
- Expose active model cooldowns and the most recent model-level result through
  the existing free-pool admin API and Chinese runtime UI.
- Let model and provider cooldowns expire automatically; a successful request
  clears the selected model's recent failure state.

## Non-Goals

- General model rotation for every provider in this slice.
- Creating one persistent deployment or channel per Kilo model.
- Retrying a streaming response after bytes have reached the client.
- Treating client errors, authentication failures, quota exhaustion, or 5xx
  responses as model-level rate limits.
- Persisting short model cooldowns across process restarts.
- Changing catalog refresh, channel model synchronization, or operator-selected
  provider order.

## Options Considered

### 1. One generated deployment per Kilo model

This would reuse the existing deployment fallback loop, but it multiplies
channels and configuration entries, makes catalog churn rewrite operator-visible
configuration, and weakens the provider boundary. It is rejected.

### 2. Mutate the deployment's global `RealModel` after each 429

This is small but unsafe under concurrent requests. One request could change the
model while another request is preparing a relay, and a transient failure would
become durable configuration. It is rejected.

### 3. Request-scoped model candidates with a model runtime registry

This is the selected approach. The catalog remains the source of model metadata,
the deployment remains the provider-level quota and health unit, and each relay
attempt carries its own model and resolved capabilities. A mutex-protected
runtime registry tracks only short-lived model cooldowns and counters.

## Architecture

### Model candidate planner

Add a fallback-domain planner that accepts a deployment and
`RequestCapabilities` and returns ordered request-scoped attempts.

For non-Kilo deployments it returns one attempt using the configured real model.
For a generated Kilo deployment it reads the immutable catalog snapshot, applies
the provider defaults plus each model's metadata, filters incompatible models,
and removes models with active cooldowns. The configured `RealModel` is promoted
to the front; remaining models keep deterministic catalog order.

The planner also returns counts for compatible, available, and cooling models.
This lets the controller distinguish "no model supports this request" from "all
compatible models are temporarily rate limited" without parsing text.

### Model runtime registry

Add a package-owned, mutex-protected registry keyed by deployment ID and model
ID. Each entry stores:

- cooldown deadline and reason
- last 429 time
- consecutive 429 count
- success and failure counters
- last successful time

Expired entries are treated as available and pruned during reads. A test-only
clock/reset hook keeps expiry and concurrency tests deterministic. The registry
is intentionally in memory because these cooldowns are short and a restart is
already a natural recovery boundary.

### Relay attempt boundary

The controller should operate on explicit attempt records rather than mutating
the shared deployment config. Each attempt contains the provider deployment plus
the request-scoped real model. Existing context keys, relay conversion, quota
accounting, concurrency slots, and usage records use that attempt model.

The deployment concurrency slot remains provider-wide. It is acquired and
released for each actual upstream attempt so a failed model does not hold the
slot while another provider is tried.

## Error And Recovery Semantics

Only a structured `ErrorCategoryRateLimit` response from Kilo is eligible for
model rotation. Rotation is allowed only when the response has not started.

1. Calculate the model cooldown from `Retry-After` when present; otherwise use
   the existing short rate-limit policy. Clamp it to the existing cooldown cap.
2. Record the current model cooldown and model-level failure.
3. If another compatible, non-cooling Kilo model remains, retry it and do not
   apply deployment cooldown, deployment failure accounting, channel-disable
   monitoring, or a fallback switch event yet.
4. If no compatible model remains, record one provider-level rate-limit failure,
   apply the existing deployment cooldown, clear provider sticky routing, and
   continue to the next deployment.
5. On success, record model success, clear its recent model error, retain normal
   provider success/sticky behavior, and leave unrelated model cooldowns intact.

If all compatible models were already cooling before the request, the Kilo
deployment is skipped as temporarily unavailable. The existing provider
cooldown normally covers this state; the planner's structured counts are the
fallback guard if model cooldown windows outlive it.

Non-429 errors keep the existing provider-level behavior and do not rotate Kilo
models. A streaming failure after output has begun remains terminal because the
gateway cannot safely replay partial output.

## Provider Priority And Isolation

The current deployment `RateLimitScore` remains the provider-level signal used
by `free_first` routing. It increases once only when all usable Kilo models are
exhausted for a request, not once per model attempt. Existing periodic decay
restores priority after quiet windows.

The current persistent deployment cooldown is the isolation mechanism. Its
duration uses `Retry-After` from the final exhausted model when available, or the
existing exponential rate-limit policy. No second provider circuit breaker is
introduced.

## API And UI

Extend the read-only runtime projection for a free provider with model runtime
summary data:

- active cooldown model count
- active model entries with model ID, cooldown deadline, reason, last 429,
  consecutive 429 count, and success/failure counters
- last model attempted and last model that succeeded

Never expose API keys or raw upstream bodies. The Chinese free-pool UI should
show a compact Kilo-only runtime summary and a details list for actively cooling
models. Existing provider health, catalog status, and manual recovery controls
remain unchanged.

## Concurrency And State Safety

- Catalog snapshots are read as existing deep copies.
- Model runtime state is protected by its own lock and never acquired while
  holding the fallback config write lock or a database transaction.
- Request attempts contain copied deployment values; no request mutates global
  `DeploymentConfig` or catalog snapshots.
- Provider-level accounting happens once per provider outcome, while model-level
  counters happen once per actual Kilo model attempt.
- Race tests must cover concurrent planning, cooldown writes, expiry, and
  snapshots.

## Testing

- Planner tests: configured model first, deterministic alternatives, capability
  filtering, active-cooldown filtering, and non-Kilo single-attempt behavior.
- Runtime tests: Retry-After duration, expiry/pruning, success recovery, snapshot
  copying, and concurrent access under the race detector.
- Controller tests: first Kilo model 429 then second succeeds; all compatible
  models 429 then provider fallback; non-429 skips model rotation; a written
  stream is never replayed; provider failure/score increments only once.
- API/UI tests: redacted runtime projection and Chinese active-cooldown display.
- Full Go tests, scoped race tests, frontend lint/test/build, Storybook,
  Playwright, rebuilt binary, and live localhost protocol smoke for Chat
  Completions, Responses, tools, and streaming.

## Acceptance Criteria

- A model-level Kilo 429 can succeed through another compatible Kilo model in
  the same client request.
- A tool/JSON/vision request never rotates to a model without the required
  capability.
- One limited Kilo model does not cool down or penalize the whole provider.
- Exhausting all compatible Kilo models cools and penalizes the provider once,
  then falls back to the next provider.
- Cooldowns expire without manual intervention and routing priority recovers
  through the existing score decay.
- Admin runtime data explains which Kilo models are cooling without exposing
  secrets.
