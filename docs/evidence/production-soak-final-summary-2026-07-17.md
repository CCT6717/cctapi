# Production soak final summary (2026-07-17)

## Status

The credential-gated paced production soak is complete. This closes the current FreeLLMAPI fallback and observability delivery scope; the project is now in maintenance mode rather than active feature expansion.

## Verified production run

| Item | Result |
|---|---:|
| Duration | 40.04 minutes |
| Requests | 312 |
| Successes / failures | 312 / 0 |
| Success rate | 100% |
| Streaming requests | 144 |
| Tool-enabled requests | 42 |
| Responses API requests | 26 |
| Distinct providers observed | 3 (`kilo`, `openrouter`, `pollinations`) |
| Distinct real models observed | 13 |
| Average latency | 2749.4 ms |
| p50 / p95 latency | 2239 / 4659 ms |

The historical run offered tools on 42 successful requests, but used `tool_choice=auto`; it therefore proves tool-enabled compatibility, not that every request returned a required tool call. The hardened harness now uses a named required tool and validates the returned function name and JSON arguments for future runs.

## Rotation and fallback evidence

- Model-level rate-limit events were cumulative runtime observations: first `9`, last `52`, observed increase `43` during the captured snapshots.
- `skipped_quota` was already `25` at the first snapshot and remained `25`; this run did not add a new skipped-quota event.
- Overlapping observability snapshots contained 8 multi-provider chain appearances representing 3 unique request IDs.
- These values are runtime counter observations, not isolated per-run event-table deltas.

## Evidence integrity

Canonical sanitized evidence:

`docs/evidence/soak-2026-07-17/production-soak-consolidated.json`

The consolidated artifact is rebuilt only from five matched segment/latency-sidecar pairs. It validates request-count and latency-count invariants, records SHA-256 source manifests, deduplicates multi-provider request IDs, uses nearest-rank percentiles, and writes atomically. Empty or mismatched inputs fail without overwriting the canonical artifact.

Raw segment and latency files are intentionally excluded from Git and retained locally at:

`D:\ct\me\cctapi-soak-archive\2026-07-17-raw`

No API token, admin token, password, raw request body, or raw upstream response body is retained in the committed evidence.

## Delivered feature boundary

- Virtual-model fallback across free providers.
- Capability-aware Kilo model selection and model-level HTTP 429 rotation.
- Provider-level degradation, cooldown, recovery, and fallback.
- Streaming replay protection and required tool-call recovery logic.
- Exact structured attempt observability with sanitized runtime diagnostics.
- Admin runtime/free-pool visibility and responsive frontend coverage.
- Credential-gated production soak harness and integrity-checked evidence aggregator.

## Remaining boundaries

- Anonymous/free-provider capacity remains opportunistic and is not an SLA.
- The runtime/catalog design targets the current single-process SQLite deployment; multi-instance coordination remains deferred.
- The historical soak does not prove required tool-call return behavior; deterministic regression coverage does, and future live runs use the hardened required-tool contract.
- Long-duration paid-capacity soak, public deployment hardening, support operations, billing, and commercial SLA work are outside this release scope.

## Maintenance mode

No further product features are planned for now. Continue only security updates, dependency compatibility, provider API compatibility, and critical reliability fixes unless a new product or commercial goal is explicitly approved.
