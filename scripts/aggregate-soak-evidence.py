#!/usr/bin/env python3
"""Aggregate cctapi production soak segments into one sanitized report.

Reads per-segment evidence JSONs + the latency sidecar produced by
scripts/fallback-production-soak.ps1 and emits a single consolidated,
sanitized production-capacity report. No tokens, passwords, raw request
bodies, or raw upstream response bodies are read or written.
"""
import json
import glob
import os
import statistics

EVIDENCE_DIR = "docs/evidence/soak-2026-07-17"
OUT = os.path.join(EVIDENCE_DIR, "production-soak-consolidated.json")

files = sorted(f for f in glob.glob(os.path.join(EVIDENCE_DIR, "production-soak-2026-*.json")))

# ---- latency sidecar (true overall percentiles) ----
latencies = []
seg_latency = []
sidecar = os.path.join(EVIDENCE_DIR, "latency-log.jsonl")
if os.path.exists(sidecar):
    with open(sidecar, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            rec = json.loads(line)
            seg_latency.append({
                "stamp": rec.get("stamp"),
                "total_requests": rec.get("total_requests"),
                "success_count": rec.get("success_count"),
                "latencies_ms": rec.get("latencies_ms", []),
            })
            latencies.extend(rec.get("latencies_ms", []))

# ---- aggregate request + provider/model stats from evidence files ----
total_requests = success = failure = 0
providers = set()
models = set()
snapshot_timeline = []
cumulative_outcomes = {}
max_cumulative = {}  # take the highest observed cumulative counts

for f in files:
    d = json.load(open(f, encoding="utf-8"))
    s = d["summary"]
    total_requests += s["total_requests"]
    success += s["success_count"]
    failure += s["failure_count"]
    providers.update(s["distinct_providers"])
    models.update(s["distinct_real_models"])
    for snap in d.get("observability_snapshots", []):
        # cumulative outcome counts => keep the max seen per outcome key
        for oc in snap.get("outcomes", []):
            key = oc.get("outcome")
            cnt = oc.get("count", 0)
            max_cumulative[key] = max(max_cumulative.get(key, 0), cnt)
        # per-snapshot multi-provider chain count (overlapping windows, indicative)
        mpc = 0
        snap_providers = set()
        snap_models = set()
        for c in snap.get("recent_chains", []):
            ps = set(st.get("provider") for st in c.get("steps", []) if st.get("provider"))
            if len(ps) > 1:
                mpc += 1
            snap_providers.update(ps)
            snap_models.update(st.get("real_model") for st in c.get("steps", []) if st.get("real_model"))
        snapshot_timeline.append({
            "at": snap.get("at"),
            "iteration": snap.get("iteration"),
            "failure_event_count": snap.get("failure_event_count"),
            "skip_event_count": snap.get("skip_event_count"),
            "outcomes": snap.get("outcomes"),
            "snapshot_providers": sorted(snap_providers),
            "snapshot_real_models": sorted(snap_models),
            "multi_provider_chains": mpc,
        })

# ---- latency percentiles (true, from raw sidecar) ----
latencies.sort()
def pct(p):
    if not latencies:
        return 0
    idx = min(len(latencies) - 1, int(len(latencies) * p))
    return latencies[idx]
avg = round(statistics.mean(latencies), 1) if latencies else 0

rate = (success / total_requests) if total_requests else 0
overall_mpc = sum(t["multi_provider_chains"] for t in snapshot_timeline)
snapshots_with_mpc = sum(1 for t in snapshot_timeline if t["multi_provider_chains"] > 0)

report = {
    "generated_at": __import__("datetime").datetime.utcnow().isoformat() + "Z",
    "kind": "production-capacity-soak-consolidated",
    "credential_source": "real-production-credentials",
    "base_url": "http://localhost:3008",
    "model": "openrouter/auto",
    "segments": len(files),
    "window_minutes": round(sum(
        json.load(open(f, encoding="utf-8"))["duration_minutes"] for f in files), 2),
    "summary": {
        "total_requests": total_requests,
        "success_count": success,
        "failure_count": failure,
        "success_rate": round(rate, 4),
        "distinct_providers": sorted(providers),
        "distinct_real_models": sorted(models),
        "distinct_real_model_count": len(models),
        "latency_ms_avg": avg,
        "latency_ms_p50": pct(0.50),
        "latency_ms_p95": pct(0.95),
        "latency_ms_p99": pct(0.99),
        "latency_ms_max": latencies[-1] if latencies else 0,
    },
    "rotation_fallback_signals": {
        "cumulative_outcomes": max_cumulative,
        "note": ("outcomes are cumulative over the persistent 1-hour attempt store; "
                 "intermediate model_rate_limited / skipped_quota events are rotated "
                 "transparently and are NOT user-facing failures."),
        "total_snapshots_observed": len(snapshot_timeline),
        "snapshots_with_cross_provider_fallback": snapshots_with_mpc,
        "multi_provider_chains_observed_total": overall_mpc,
    },
    "per_segment_latency": [],
    "snapshot_timeline": snapshot_timeline,
}

def pct_seg(arr, p):
    if not arr:
        return 0
    a = sorted(arr)
    return a[min(len(a) - 1, int(len(a) * p))]

report["per_segment_latency"] = [
    {"stamp": r["stamp"], "requests": r["total_requests"], "success": r["success_count"],
     "p50": pct_seg(r["latencies_ms"], 0.50), "p95": pct_seg(r["latencies_ms"], 0.95)}
    for r in seg_latency
]

with open(OUT, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2, ensure_ascii=False)

print("WROTE", OUT)
print("segments:", report["segments"], "window_minutes:", report["window_minutes"])
print("total_requests:", total_requests, "success:", success, "rate:", report["summary"]["success_rate"])
print("providers:", report["summary"]["distinct_providers"])
print("distinct_real_models:", report["summary"]["distinct_real_model_count"])
print("latency p50/p95/p99:", report["summary"]["latency_ms_p50"], report["summary"]["latency_ms_p95"], report["summary"]["latency_ms_p99"])
print("cumulative_outcomes:", max_cumulative)
print("snapshots_with_cross_provider_fallback:", snapshots_with_mpc, "of", len(snapshot_timeline))
