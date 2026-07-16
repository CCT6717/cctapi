# Persistent Provider Rate-Limit Degradation Design

Date: 2026-07-16

## Goal

Lower the automatic-routing priority of a provider that repeatedly returns a
confirmed provider-level HTTP 429 after its cooldown has expired. The provider
must remain available as a last-resort candidate and must recover priority
automatically after successful or quiet recovery windows.

This feature extends the existing deployment runtime state. It does not change
Kilo model-level rotation: an intermediate Kilo model 429 remains isolated to
that model, and a provider episode is recorded only after all compatible Kilo
models are exhausted and the relay reaches provider-level accounting.

## Current Behavior And Gap

`DeploymentRuntimeState.RateLimitScore` is a short-lived provider signal. A
confirmed provider-level HTTP 429 increments it once, and the health loop
periodically decays it. `free_first` routing consumes this signal, but the other
automatic strategies do not. The current state also cannot distinguish one
cooldown episode from a provider that fails again after cooldown recovery.

The new state machine complements, rather than replaces, `RateLimitScore`:

- `RateLimitScore` remains the immediate, short-term pressure signal.
- provider degradation records repeated failures across distinct cooldown
  episodes and applies a stronger, strategy-independent ordering penalty.

## Considered Approaches

### 1. Explicit in-memory provider degradation state (selected)

Extend the deployment runtime boundary with episode, level, and recovery state.
All automatic strategies consume the resulting level. This matches the current
single-process runtime model, is race-testable, and avoids a database migration.

### 2. Reinterpret `RateLimitScore` as the complete state machine

This is smaller, but it cannot reliably distinguish repeated records within one
cooldown from failures after separate cooldowns. It also provides insufficient
diagnostics for operators. Rejected.

### 3. Persist degradation in SQLite

This would survive restarts and help a future multi-instance deployment, but it
adds migration and ownership complexity that the current single-process runtime
does not need. Rejected for this slice.

## Runtime State

Add the following provider-level fields to `DeploymentRuntimeState` and its safe
snapshot:

- `RateLimitEpisodeCount`: distinct confirmed provider-level 429 episodes in
  the active observation window.
- `RateLimitDegradationLevel`: current level from 0 through 3.
- `LastProviderRateLimitedAt`: most recent accepted episode time.
- `RateLimitEpisodeCooldownUntil`: cooldown deadline associated with the most
  recent episode, used to reject duplicate records from the same window.
- `NextDegradationRecoveryAt`: next time-based level reduction.
- `ConsecutiveRecoverySuccesses`: successful provider relays since the most
  recent accepted episode.

The state is process memory shared by concurrent requests and is cleared on
process restart, consistent with the existing deployment runtime state.

The externally visible reason is the fixed safe string `repeated rate limits`.
Raw upstream response bodies, caller-provided error text, keys, and tokens are
never stored in the degradation snapshot.

## Episode And Degradation Rules

An accepted episode must satisfy every condition below:

1. The upstream status is HTTP 429.
2. Error classification is rate limit.
3. The relay has reached provider-level accounting.
4. For Kilo, no compatible model attempt remains.

The state transition rules are:

1. The first accepted episode starts a 15-minute observation window. It records
   episode count 1 and leaves degradation at level 0.
2. An event whose timestamp is not after the recorded cooldown deadline is a
   duplicate from the same cooldown episode and does not change episode count or
   level.
3. A second accepted episode after cooldown expiry and within 15 minutes enters
   level 1. Each later distinct episode within the observation window raises the
   level once, capped at level 3.
4. An accepted episode more than 15 minutes after the previous episode starts a
   new observation window at episode count 1 and level 0.
5. A new accepted episode resets consecutive recovery successes and schedules
   the next time recovery for 10 minutes later.

Kilo model-level 429 events handled by `MarkFreeProviderModelRateLimited` never
enter this provider state machine. Local concurrency skips, quota pre-check
skips, text-shaped non-429 errors, and model capability false positives also do
not enter it.

## Recovery Rules

Recovery is gradual and does not require an operator:

- A successful provider relay while level 0 clears a first-episode observation.
- Each successful provider relay while degraded lowers the level by one and
  increments consecutive recovery successes. L3 therefore requires at most
  three consecutive successes to clear.
- Any accepted provider-level 429 resets the consecutive-success count.
- While degraded, each 10-minute period without a new accepted episode lowers
  the level by one. The periodic runtime maintenance loop performs this decay.
- Reaching level 0 clears episode count, timestamps, recovery counters, and the
  safe reason.
- Manual deployment recovery clears the complete provider degradation state in
  addition to the existing cooldown, quota, sticky, model-runtime, and error
  state.

All transitions use the existing runtime-state mutex. Time-aware internal
helpers accept an explicit timestamp so unit tests do not sleep; production
wrappers use `time.Now()`.

## Routing Integration

Provider degradation changes ordering, not eligibility. Apply the following
penalty after the existing strategy score is calculated:

| Level | Score penalty |
| --- | ---: |
| 0 | 0 |
| 1 | 25 |
| 2 | 50 |
| 3 | 75 |

The penalty applies consistently to `quality_first`, `cost_first`, and
`free_first`, and to the dynamic scores returned by the admin score endpoint.
The existing `RateLimitScore` contribution remains in `free_first` as the
short-term pressure signal.

The feature does not hard-skip a degraded provider. If healthier providers are
unavailable, the degraded provider can still receive traffic and demonstrate
recovery. Explicit fixed routing and preferred-start semantics remain unchanged;
the penalty applies only when candidates are automatically ranked.

## Relay And Health Boundaries

The controller remains orchestration-only:

- after the written-response guard and model-attempt decision, confirmed HTTP
  429 provider exhaustion calls the fallback state API once;
- intermediate Kilo model rotation returns before that call;
- successful provider settlement uses the existing `RecordSuccess` path to
  advance recovery;
- cooldown calculation remains owned by the existing cooldown module.

The periodic health loop may run time-based decay, but a standalone health probe
429 does not create a degradation episode in this slice. Kilo health probes are
not model-rotation-aware, so counting them could incorrectly turn one model's
429 into a provider penalty.

## Runtime API And Admin UI

The existing deployment runtime endpoint adds a nested
`provider_rate_limit_degradation` object:

```json
{
  "active": true,
  "level": 2,
  "episode_count": 3,
  "reason": "repeated rate limits",
  "last_rate_limited_at": "2026-07-16T10:00:00Z",
  "next_recovery_at": "2026-07-16T10:10:00Z",
  "consecutive_recovery_successes": 0
}
```

Inactive deployments return the same object with `active: false`, level and
counts set to zero, and optional timestamps omitted.

The existing runtime panel displays a compact Chinese diagnostic without adding
a new page or dashboard shortcut:

```text
持续限流降权 L2
跨冷却限流 3 次
预计恢复：2026/7/16 18:10:00
```

Provider brand names and technical abbreviations remain unchanged. The UI never
renders raw upstream bodies or secrets.

## Testing Strategy

### Runtime state tests

- first episode observes without degradation;
- second distinct post-cooldown episode enters L1;
- duplicate records inside one cooldown do not increment;
- a gap over 15 minutes starts a new observation window;
- repeated episodes cap at L3;
- success lowers one level and L3 clears after three consecutive successes;
- level-0 success clears the first-episode observation;
- each quiet 10-minute window lowers one level;
- manual reset clears all fields;
- snapshots contain only the fixed reason;
- concurrent record, success, snapshot, decay, and reset operations pass race
  detection.

### Routing and relay tests

- every automatic strategy orders a healthy peer before an otherwise-equivalent
  degraded provider;
- fixed/preferred-start semantics remain unchanged;
- a Kilo intermediate model 429 does not record a provider episode;
- all compatible Kilo models exhausted record exactly one provider episode;
- a non-429 rate-limit-shaped error does not record an episode;
- a successful fallback attempt advances only the successful provider's
  recovery state.

### API and UI tests

- runtime rows include level, episode count, safe reason, and recovery time;
- serialized responses do not contain keys, bearer values, or raw bodies;
- the Chinese compact diagnostic renders active degradation details;
- inactive degradation does not add visual noise;
- existing mobile table scrolling remains usable.

### Verification and production evidence

- focused Go tests for fallback, controller, and router;
- scoped race tests for shared-state packages;
- full Go tests, vet, and build;
- frontend ESLint, Vitest, Vite, Storybook, and Playwright;
- rebuild and restart the local server on port 3008;
- paced soak through an automatic virtual model verifies successful traffic does
  not create or retain degradation on a normal provider;
- deterministic repeated-429 coverage proves cross-cooldown escalation and
  provider fallback without depending on an upstream quota event;
- sanitized evidence records request counts, selected providers, degradation
  transitions, recovery, and final level without credentials or raw bodies.

## Acceptance Criteria

- One provider-level 429 does not degrade the provider.
- A second confirmed provider-level 429 after cooldown expiry and within 15
  minutes enters L1 exactly once.
- Further cross-cooldown episodes rise to L3 without duplicate increments.
- All automatic routing strategies prefer an equivalent healthy provider while
  degradation is active.
- A degraded provider remains eligible as a last resort.
- Successes and quiet windows restore priority automatically and manual recovery
  clears state immediately.
- Kilo intermediate model 429 rotation remains isolated from provider state.
- The admin runtime API and Chinese UI show the safe reason, episode count,
  level, and next recovery time.
- Focused, full, race, frontend, browser, build, and paced-soak gates pass with
  sanitized evidence.

## Non-Goals

- Persisting degradation across process restarts.
- Coordinating degradation state across multiple application instances.
- Changing provider configuration or mutating catalog snapshots.
- Hard-disabling channels or deployments based only on degradation level.
- Treating health-probe 429 responses as degradation episodes.
