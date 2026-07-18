import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("aggregate-soak-evidence.py")


class AggregateSoakEvidenceTest(unittest.TestCase):
    def run_aggregator(self, evidence_dir: Path, output: Path):
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--evidence-dir",
                str(evidence_dir),
                "--output",
                str(output),
            ],
            cwd=evidence_dir.parents[2],
            capture_output=True,
            text=True,
            check=False,
        )

    def write_segment(self, evidence_dir: Path, stamp: str, *, snapshots=None):
        segment = {
            "generated_at": f"2026-07-17T{stamp[-4:-2]}:{stamp[-2:]}:30Z",
            "kind": "production-capacity-soak",
            "credential_source": "real-production-credentials",
            "base_url": "http://localhost:3008",
            "model": "openrouter/auto",
            "duration_minutes": 10,
            "interval_sec": 5,
            "summary": {
                "total_requests": 2,
                "success_count": 2,
                "failure_count": 0,
                "stream_requests": 1,
                "tool_requests": 1,
                "responses_requests": 1,
                "latency_ms_avg": 15.0,
                "latency_ms_p50": 20,
                "latency_ms_p95": 20,
                "distinct_providers": ["kilo", "pollinations"],
                "distinct_real_models": ["model-a", "model-b"],
            },
            "observability_snapshots": snapshots or [],
            "request_errors": [],
        }
        path = evidence_dir / f"production-soak-{stamp}.json"
        path.write_text(json.dumps(segment), encoding="utf-8")

    def write_sidecar(self, evidence_dir: Path, records):
        (evidence_dir / "latency-log.jsonl").write_text(
            "".join(json.dumps(record) + "\n" for record in records),
            encoding="utf-8",
        )

    def test_empty_input_fails_without_overwriting_existing_output(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence_dir = root / "docs" / "evidence" / "soak-2026-07-17"
            evidence_dir.mkdir(parents=True)
            output = evidence_dir / "production-soak-consolidated.json"
            output.write_text('{"sentinel": true}', encoding="utf-8")

            result = self.run_aggregator(evidence_dir, output)

            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(json.loads(output.read_text(encoding="utf-8")), {"sentinel": True})

    def test_rejects_sidecar_that_does_not_match_segment_counts(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence_dir = root / "docs" / "evidence" / "soak-2026-07-17"
            evidence_dir.mkdir(parents=True)
            output = evidence_dir / "production-soak-consolidated.json"
            self.write_segment(evidence_dir, "2026-07-17-1000")
            self.write_sidecar(
                evidence_dir,
                [{
                    "stamp": "2026-07-17-1000",
                    "total_requests": 3,
                    "success_count": 2,
                    "failure_count": 1,
                    "duration_minutes": 10,
                    "interval_sec": 5,
                    "latencies_ms": [10, 20, 30],
                }],
            )

            result = self.run_aggregator(evidence_dir, output)

            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists())

    def test_aggregates_bound_segments_and_deduplicates_request_chains(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence_dir = root / "docs" / "evidence" / "soak-2026-07-17"
            evidence_dir.mkdir(parents=True)
            output = evidence_dir / "production-soak-consolidated.json"
            first_snapshot = {
                "at": "2026-07-17T10:00:20Z",
                "iteration": 1,
                "failure_event_count": 2,
                "skip_event_count": 0,
                "outcomes": [{"outcome": "model_rate_limited", "count": 2}],
                "recent_chains": [{
                    "request_id": "req-cross",
                    "steps": [{"provider": "kilo", "real_model": "model-a"}, {"provider": "pollinations", "real_model": "model-b"}],
                }],
            }
            second_snapshot = {
                "at": "2026-07-17T10:10:20Z",
                "iteration": 1,
                "failure_event_count": 4,
                "skip_event_count": 0,
                "outcomes": [{"outcome": "model_rate_limited", "count": 4}],
                "recent_chains": [{
                    "request_id": "req-cross",
                    "steps": [{"provider": "kilo", "real_model": "model-a"}, {"provider": "pollinations", "real_model": "model-b"}],
                }],
            }
            self.write_segment(evidence_dir, "2026-07-17-1000", snapshots=[first_snapshot])
            self.write_segment(evidence_dir, "2026-07-17-1010", snapshots=[second_snapshot])
            second_path = evidence_dir / "production-soak-2026-07-17-1010.json"
            second_segment = json.loads(second_path.read_text(encoding="utf-8"))
            second_segment["summary"].update({
                "latency_ms_avg": 35.0,
                "latency_ms_p50": 40,
                "latency_ms_p95": 40,
            })
            second_path.write_text(json.dumps(second_segment), encoding="utf-8")
            self.write_sidecar(
                evidence_dir,
                [
                    {"stamp": "2026-07-17-1000", "duration_minutes": 10, "interval_sec": 5, "total_requests": 2, "success_count": 2, "failure_count": 0, "latencies_ms": [10, 20]},
                    {"stamp": "2026-07-17-1010", "duration_minutes": 10, "interval_sec": 5, "total_requests": 2, "success_count": 2, "failure_count": 0, "latencies_ms": [30, 40]},
                ],
            )

            result = self.run_aggregator(evidence_dir, output)

            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["summary"]["total_requests"], 4)
            self.assertEqual(report["summary"]["stream_requests"], 2)
            self.assertEqual(report["summary"]["tool_enabled_requests"], 2)
            self.assertEqual(report["summary"]["responses_requests"], 2)
            self.assertEqual(report["summary"]["latency_ms_p50"], 20)
            self.assertEqual(report["summary"]["latency_ms_p95"], 40)
            signals = report["rotation_fallback_signals"]
            self.assertEqual(signals["unique_multi_provider_request_count"], 1)
            self.assertEqual(signals["multi_provider_chain_appearances"], 2)
            observation = signals["cumulative_outcome_observations"]["model_rate_limited"]
            self.assertEqual(observation["first_observed"], 2)
            self.assertEqual(observation["last_observed"], 4)
            self.assertEqual(observation["observed_increase"], 2)
            self.assertEqual(len(report["source_manifest"]["segments"]), 2)

    def test_rejects_latency_summary_that_does_not_match_sidecar(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence_dir = root / "docs" / "evidence" / "soak-2026-07-17"
            evidence_dir.mkdir(parents=True)
            output = evidence_dir / "production-soak-consolidated.json"
            self.write_segment(evidence_dir, "2026-07-17-1000")
            segment_path = evidence_dir / "production-soak-2026-07-17-1000.json"
            segment = json.loads(segment_path.read_text(encoding="utf-8"))
            segment["summary"]["latency_ms_p95"] = 999
            segment_path.write_text(json.dumps(segment), encoding="utf-8")
            self.write_sidecar(evidence_dir, [{
                "stamp": "2026-07-17-1000", "duration_minutes": 10, "interval_sec": 5,
                "total_requests": 2, "success_count": 2, "failure_count": 0, "latencies_ms": [10, 20],
            }])

            result = self.run_aggregator(evidence_dir, output)

            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists())

    def test_aggregates_future_required_tool_count(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence_dir = root / "docs" / "evidence" / "soak-2026-07-17"
            evidence_dir.mkdir(parents=True)
            output = evidence_dir / "production-soak-consolidated.json"
            self.write_segment(evidence_dir, "2026-07-17-1000")
            segment_path = evidence_dir / "production-soak-2026-07-17-1000.json"
            segment = json.loads(segment_path.read_text(encoding="utf-8"))
            segment["summary"].pop("tool_requests")
            segment["summary"]["required_tool_requests"] = 1
            segment_path.write_text(json.dumps(segment), encoding="utf-8")
            self.write_sidecar(evidence_dir, [{
                "stamp": "2026-07-17-1000", "duration_minutes": 10, "interval_sec": 5,
                "total_requests": 2, "success_count": 2, "failure_count": 0,
                "required_tool_requests": 1, "latencies_ms": [10, 20],
            }])

            result = self.run_aggregator(evidence_dir, output)

            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["summary"]["required_tool_requests"], 1)
            self.assertEqual(report["summary"]["tool_enabled_requests"], 0)


if __name__ == "__main__":
    unittest.main()
