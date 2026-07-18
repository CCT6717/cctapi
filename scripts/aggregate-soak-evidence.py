#!/usr/bin/env python3
"""Validate and consolidate production soak evidence segments."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import re
import tempfile
from pathlib import Path
from typing import Any


SEGMENT_RE = re.compile(r"^production-soak-(\d{4}-\d{2}-\d{2}-\d{4})\.json$")
DEFAULT_EVIDENCE_DIR = Path("docs/evidence/soak-2026-07-17")


def fail(message: str) -> None:
    raise ValueError(message)


def load_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8-sig"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"invalid JSON in {path.name}: {exc}")
    if not isinstance(value, dict):
        fail(f"expected an object in {path.name}")
    return value


def nonnegative_int(value: Any, label: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        fail(f"{label} must be a non-negative integer")
    return value


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def nearest_rank(values: list[int], percentile: float) -> int:
    if not values:
        return 0
    ordered = sorted(values)
    return ordered[max(0, math.ceil(percentile * len(ordered)) - 1)]


def legacy_segment_percentile(values: list[int], percentile: float) -> int:
    """Match the historical harness percentile used in source segment summaries."""
    ordered = sorted(values)
    return ordered[min(len(ordered) - 1, math.floor(percentile * len(ordered)))]


def parse_sidecars(path: Path) -> dict[str, dict[str, Any]]:
    if not path.is_file():
        fail(f"missing latency sidecar: {path.name}")
    entries: dict[str, dict[str, Any]] = {}
    for line_number, line in enumerate(path.read_text(encoding="utf-8-sig").splitlines(), 1):
        if not line.strip():
            continue
        try:
            entry = json.loads(line)
        except json.JSONDecodeError as exc:
            fail(f"invalid JSON in {path.name} line {line_number}: {exc}")
        stamp = entry.get("stamp")
        if not isinstance(stamp, str) or not stamp:
            fail(f"missing stamp in {path.name} line {line_number}")
        if stamp in entries:
            fail(f"duplicate latency sidecar stamp: {stamp}")
        entries[stamp] = entry
    return entries


def outcome_counts(snapshot: dict[str, Any]) -> dict[str, int]:
    result: dict[str, int] = {}
    for item in snapshot.get("outcomes") or []:
        if not isinstance(item, dict) or not isinstance(item.get("outcome"), str):
            continue
        result[item["outcome"]] = nonnegative_int(
            item.get("count"), f"outcome {item['outcome']} count"
        )
    return result


def atomic_write_json(output: Path, report: dict[str, Any]) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary: str | None = None
    try:
        with tempfile.NamedTemporaryFile(
            "w", encoding="utf-8", newline="\n", dir=output.parent, delete=False
        ) as handle:
            temporary = handle.name
            json.dump(report, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, output)
    finally:
        if temporary and os.path.exists(temporary):
            os.unlink(temporary)


def consolidate(evidence_dir: Path) -> dict[str, Any]:
    segment_paths: list[tuple[str, Path]] = []
    for path in evidence_dir.glob("production-soak-*.json"):
        match = SEGMENT_RE.match(path.name)
        if match:
            segment_paths.append((match.group(1), path))
    segment_paths.sort()
    if not segment_paths:
        fail(f"no production soak segments found in {evidence_dir}")

    stamps = [stamp for stamp, _ in segment_paths]
    if len(stamps) != len(set(stamps)):
        fail("duplicate production soak segment stamps")

    sidecar_path = evidence_dir / "latency-log.jsonl"
    sidecars = parse_sidecars(sidecar_path)
    if set(sidecars) != set(stamps):
        missing = sorted(set(stamps) - set(sidecars))
        extra = sorted(set(sidecars) - set(stamps))
        fail(f"segment/sidecar stamp mismatch: missing={missing}, extra={extra}")

    segments: list[tuple[str, Path, dict[str, Any], dict[str, Any]]] = []
    identity: tuple[Any, Any, Any] | None = None
    total_requests = success_count = failure_count = 0
    stream_requests = tool_enabled_requests = required_tool_requests = responses_requests = 0
    duration_minutes = 0.0
    latencies: list[int] = []
    providers: set[str] = set()
    models: set[str] = set()
    snapshots: list[dict[str, Any]] = []
    manifest: list[dict[str, Any]] = []
    common_interval: int | None = None

    for stamp, path in segment_paths:
        segment = load_json(path)
        summary = segment.get("summary")
        if not isinstance(summary, dict):
            fail(f"missing summary in {path.name}")
        sidecar = sidecars[stamp]

        total = nonnegative_int(summary.get("total_requests"), f"{path.name} total_requests")
        success = nonnegative_int(summary.get("success_count"), f"{path.name} success_count")
        failure = nonnegative_int(summary.get("failure_count"), f"{path.name} failure_count")
        if total != success + failure:
            fail(f"request count invariant failed in {path.name}")
        for field, expected in (
            ("total_requests", total),
            ("success_count", success),
            ("failure_count", failure),
        ):
            actual = nonnegative_int(sidecar.get(field), f"sidecar {stamp} {field}")
            if actual != expected:
                fail(f"sidecar count mismatch for {stamp}: {field}")

        raw_latencies = sidecar.get("latencies_ms")
        if not isinstance(raw_latencies, list) or len(raw_latencies) != total:
            fail(f"latency count mismatch for {stamp}")
        segment_latencies = [
            nonnegative_int(value, f"sidecar {stamp} latency") for value in raw_latencies
        ]
        duration = segment.get("duration_minutes")
        if isinstance(duration, bool) or not isinstance(duration, (int, float)) or duration < 0:
            fail(f"invalid duration_minutes in {path.name}")
        sidecar_duration = sidecar.get("duration_minutes")
        if isinstance(sidecar_duration, bool) or not isinstance(sidecar_duration, (int, float)):
            fail(f"invalid sidecar duration_minutes for {stamp}")
        if abs(float(sidecar_duration) - float(duration)) > 0.001:
            fail(f"sidecar duration mismatch for {stamp}")
        interval = nonnegative_int(segment.get("interval_sec"), f"{path.name} interval_sec")
        if interval == 0:
            fail(f"interval_sec must be positive in {path.name}")
        sidecar_interval = nonnegative_int(sidecar.get("interval_sec"), f"sidecar {stamp} interval_sec")
        if sidecar_interval != interval:
            fail(f"sidecar interval mismatch for {stamp}")
        if common_interval is None:
            common_interval = interval
        elif common_interval != interval:
            fail(f"interval_sec mismatch in {path.name}")

        expected_latency = {
            "latency_ms_avg": round(sum(segment_latencies) / len(segment_latencies), 1) if segment_latencies else 0,
            "latency_ms_p50": legacy_segment_percentile(segment_latencies, 0.50) if segment_latencies else 0,
            "latency_ms_p95": legacy_segment_percentile(segment_latencies, 0.95) if segment_latencies else 0,
        }
        for field, expected in expected_latency.items():
            actual = summary.get(field)
            if isinstance(actual, bool) or not isinstance(actual, (int, float)) or abs(float(actual) - float(expected)) > 0.1:
                fail(f"segment latency summary mismatch for {stamp}: {field}")

        current_identity = (
            segment.get("base_url"),
            segment.get("model"),
            segment.get("credential_source"),
        )
        if any(not isinstance(value, str) or not value for value in current_identity):
            fail(f"missing soak identity in {path.name}")
        if identity is None:
            identity = current_identity
        elif current_identity != identity:
            fail(f"soak identity mismatch in {path.name}")

        total_requests += total
        success_count += success
        failure_count += failure
        segment_stream = nonnegative_int(
            summary.get("stream_requests", 0), f"{path.name} stream_requests"
        )
        segment_tool_enabled = nonnegative_int(
            summary.get("tool_enabled_requests", summary.get("tool_requests", 0)),
            f"{path.name} tool_enabled_requests",
        )
        segment_required_tools = nonnegative_int(
            summary.get("required_tool_requests", 0), f"{path.name} required_tool_requests"
        )
        segment_responses = nonnegative_int(
            summary.get("responses_requests", 0), f"{path.name} responses_requests"
        )
        for label, value in (
            ("stream_requests", segment_stream),
            ("tool_enabled_requests", segment_tool_enabled),
            ("required_tool_requests", segment_required_tools),
            ("responses_requests", segment_responses),
        ):
            if value > total:
                fail(f"{label} exceeds total_requests in {path.name}")
            if label in sidecar:
                sidecar_value = nonnegative_int(sidecar.get(label), f"sidecar {stamp} {label}")
                if sidecar_value != value:
                    fail(f"sidecar count mismatch for {stamp}: {label}")
        stream_requests += segment_stream
        tool_enabled_requests += segment_tool_enabled
        required_tool_requests += segment_required_tools
        responses_requests += segment_responses
        duration_minutes += float(duration)
        latencies.extend(segment_latencies)
        providers.update(value for value in summary.get("distinct_providers", []) if isinstance(value, str))
        models.update(value for value in summary.get("distinct_real_models", []) if isinstance(value, str))
        segment_snapshots = segment.get("observability_snapshots") or []
        if not isinstance(segment_snapshots, list):
            fail(f"invalid observability_snapshots in {path.name}")
        snapshots.extend(value for value in segment_snapshots if isinstance(value, dict))
        segments.append((stamp, path, segment, sidecar))
        manifest.append(
            {
                "stamp": stamp,
                "segment_file": path.name,
                "segment_sha256": sha256(path),
                "sidecar_file": sidecar_path.name,
                "sidecar_entry_sha256": hashlib.sha256(
                    json.dumps(sidecar, sort_keys=True, separators=(",", ":")).encode("utf-8")
                ).hexdigest(),
            }
        )

    outcome_series: dict[str, list[int]] = {}
    multi_provider_appearances = 0
    multi_provider_request_ids: set[str] = set()
    timeline: list[dict[str, Any]] = []
    for snapshot in snapshots:
        counts = outcome_counts(snapshot)
        for name, count in counts.items():
            outcome_series.setdefault(name, []).append(count)
        for chain in snapshot.get("recent_chains") or []:
            if not isinstance(chain, dict):
                continue
            chain_providers = {
                step.get("provider")
                for step in chain.get("steps") or []
                if isinstance(step, dict) and isinstance(step.get("provider"), str) and step.get("provider")
            }
            if len(chain_providers) > 1:
                multi_provider_appearances += 1
                request_id = chain.get("request_id")
                if isinstance(request_id, str) and request_id:
                    multi_provider_request_ids.add(request_id)
        timeline.append(
            {
                "at": snapshot.get("at"),
                "iteration": snapshot.get("iteration"),
                "outcomes": counts,
            }
        )

    observations = {
        name: {
            "first_observed": values[0],
            "last_observed": values[-1],
            "peak_observed": max(values),
            "observed_increase": max(0, values[-1] - values[0]),
        }
        for name, values in sorted(outcome_series.items())
        if values
    }
    assert identity is not None
    generated_values = [
        segment.get("generated_at") for _, _, segment, _ in segments if isinstance(segment.get("generated_at"), str)
    ]
    generated_at = max(generated_values) if generated_values else segments[-1][0]
    success_rate = success_count / total_requests if total_requests else 0.0

    return {
        "generated_at": generated_at,
        "kind": "production-capacity-soak-consolidated",
        "credential_source": identity[2],
        "base_url": identity[0],
        "model": identity[1],
        "duration_minutes": round(duration_minutes, 2),
        "interval_sec": common_interval,
        "summary": {
            "total_requests": total_requests,
            "success_count": success_count,
            "failure_count": failure_count,
            "success_rate": round(success_rate, 4),
            "stream_requests": stream_requests,
            "tool_enabled_requests": tool_enabled_requests,
            "required_tool_requests": required_tool_requests,
            "responses_requests": responses_requests,
            "distinct_providers": sorted(providers),
            "distinct_real_models": sorted(models),
            "latency_ms_avg": round(sum(latencies) / len(latencies), 1) if latencies else 0,
            "latency_ms_p50": nearest_rank(latencies, 0.50),
            "latency_ms_p95": nearest_rank(latencies, 0.95),
        },
        "rotation_fallback_signals": {
            "unique_multi_provider_request_count": len(multi_provider_request_ids),
            "multi_provider_chain_appearances": multi_provider_appearances,
            "cumulative_outcome_observations": observations,
        },
        "observability_timeline": timeline,
        "source_manifest": {
            "segments": manifest,
            "latency_sidecar": {
                "file": sidecar_path.name,
                "sha256": sha256(sidecar_path),
            },
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence-dir", type=Path, default=DEFAULT_EVIDENCE_DIR)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    output = args.output or args.evidence_dir / "production-soak-consolidated.json"
    try:
        report = consolidate(args.evidence_dir)
        atomic_write_json(output, report)
    except (OSError, ValueError) as exc:
        parser.exit(1, f"error: {exc}\n")
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
