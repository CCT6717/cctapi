import copy
import importlib.util
import io
import json
import sqlite3
import tempfile
import unittest
from unittest import mock
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("soak-test.py")
SPEC = importlib.util.spec_from_file_location("soak_test_script", SCRIPT_PATH)
soak = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(soak)


class SoakScriptTests(unittest.TestCase):
    def test_text_requests_allow_visible_output_budget(self):
        self.assertGreaterEqual(soak.TEXT_MAX_TOKENS, 128)
        self.assertGreaterEqual(soak.RESPONSES_MAX_OUTPUT_TOKENS, 512)

    def test_resolve_model_uses_only_valid_defaults(self):
        self.assertEqual(soak.resolve_model("kilo", ""), "kilo-auto/free")
        self.assertEqual(soak.resolve_model("openrouter", ""), "openrouter/auto")
        self.assertEqual(soak.resolve_model("ovh", "explicit/model"), "explicit/model")
        with self.assertRaises(ValueError):
            soak.resolve_model("ovh", "")
        with self.assertRaises(ValueError):
            soak.resolve_model("pollinations", "")

    def test_http_200_requires_valid_protocol_structure(self):
        self.assertEqual(
            soak.classify_outcome({"status": 200, "protocol_valid": True}),
            "success",
        )
        self.assertEqual(
            soak.classify_outcome({"status": 200, "protocol_valid": False}),
            "protocol_error",
        )

    def test_stream_requires_frames_content_and_done_marker(self):
        valid_body = (
            b'data: {"model":"upstream/model","choices":[{"delta":{"content":"ok"}}]}\n\n'
            b"data: [DONE]\n\n"
        )
        parsed = soak.parse_stream_response(valid_body)
        self.assertTrue(parsed["protocol_valid"])
        self.assertEqual(parsed["stream_frame_count"], 1)
        self.assertTrue(parsed["stream_done"])
        self.assertTrue(parsed["content_present"])
        self.assertEqual(parsed["real_model"], "upstream/model")

        truncated = soak.parse_stream_response(valid_body.split(b"data: [DONE]")[0])
        self.assertFalse(truncated["protocol_valid"])

    def test_json_protocol_parsers_validate_responses_and_tools(self):
        responses = soak.parse_json_response(
            "responses",
            {
                "id": "resp_1",
                "model": "upstream/model",
                "output": [
                    {
                        "type": "message",
                        "content": [{"type": "output_text", "text": "ok"}],
                    }
                ],
            },
        )
        self.assertTrue(responses["protocol_valid"])
        self.assertTrue(responses["content_present"])
        self.assertTrue(responses["response_id_present"])
        self.assertEqual(responses["response_output_count"], 1)

        for malformed_output in (
            "not-a-list",
            [{"type": "message"}],
            [{"type": "message", "content": [{"type": "output_text", "text": ""}]}],
        ):
            malformed = soak.parse_json_response(
                "responses",
                {"id": "resp_bad", "output": malformed_output},
            )
            self.assertFalse(malformed["protocol_valid"])

        tools = soak.parse_json_response(
            "tools",
            {
                "model": "upstream/model",
                "choices": [
                    {
                        "message": {
                            "tool_calls": [
                                {"function": {"name": "get_weather", "arguments": '{"city":"Tokyo"}'}}
                            ]
                        }
                    }
                ],
            },
        )
        self.assertTrue(tools["protocol_valid"])
        self.assertEqual(tools["tool_call_count"], 1)
        self.assertTrue(tools["tool_arguments_valid_json"])

        invalid_tools = soak.parse_json_response(
            "tools",
            {
                "choices": [
                    {"message": {"tool_calls": [{"function": {"arguments": "not-json"}}]}}
                ]
            },
        )
        self.assertFalse(invalid_tools["protocol_valid"])

    def test_attempt_events_query_uses_virtual_model_and_sqlite_datetime(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            db_path = Path(temp_dir) / "attempts.db"
            conn = sqlite3.connect(db_path)
            conn.execute(
                """
                CREATE TABLE attempt_events (
                    id INTEGER PRIMARY KEY,
                    created_at TEXT,
                    request_id TEXT,
                    virtual_model TEXT,
                    provider TEXT,
                    deployment_id TEXT,
                    real_model TEXT,
                    outcome TEXT,
                    status_code INTEGER,
                    error_category TEXT,
                    duration_ms INTEGER,
                    stream_written INTEGER,
                    plan_index INTEGER,
                    upstream_attempt_index INTEGER
                )
                """
            )
            conn.executemany(
                """
                INSERT INTO attempt_events (
                    created_at, request_id, virtual_model, provider,
                    real_model, outcome, status_code
                ) VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                [
                    (
                        "2026-07-15 06:44:42.100+00:00",
                        "request-before-window",
                        "openrouter/auto",
                        "kilo",
                        "before/model:free",
                        "model_rate_limited",
                        429,
                    ),
                    (
                        "2026-07-15 06:44:42.300+00:00",
                        "request-full-id",
                        "openrouter/auto",
                        "kilo",
                        "cohere/north-mini-code:free",
                        "model_rate_limited",
                        429,
                    ),
                    (
                        "2026-07-15 06:44:42.500+00:00",
                        "request-after-window",
                        "openrouter/auto",
                        "kilo",
                        "after/model:free",
                        "model_rate_limited",
                        429,
                    ),
                ],
            )
            conn.commit()
            conn.close()

            result = soak.check_attempt_events(
                "openrouter/auto",
                "2026-07-15T06:44:42.200+00:00",
                "2026-07-15T06:44:42.400+00:00",
                db_path=db_path,
            )

        self.assertEqual(result["count"], 1)
        self.assertEqual(result["distinct_requests"], 1)
        self.assertEqual(result["breakdown"]["kilo"]["model_rate_limited"], 1)
        self.assertEqual(result["request_paths"]["request-full-id"][0]["provider"], "kilo")

    def test_percentile_uses_nearest_rank(self):
        self.assertEqual(soak.percentile(list(range(1, 21)), 0.95), 19)
        self.assertEqual(soak.percentile([7], 0.95), 7)
        self.assertEqual(soak.percentile([], 0.95), 0)

    def test_request_types_rotate_deterministically(self):
        request_types = ["chat", "stream", "responses", "tools"]
        observed = [soak.request_type_for_index(request_types, index) for index in range(6)]
        self.assertEqual(
            observed,
            ["chat", "stream", "responses", "tools", "chat", "stream"],
        )
        with self.assertRaises(ValueError):
            soak.request_type_for_index([], 0)

    def test_default_request_types_cover_all_protocols_and_force_tool_choice(self):
        self.assertEqual(
            soak.default_request_types(),
            ["chat", "stream", "responses", "tools"],
        )
        self.assertEqual(
            soak.REQUIRED_TOOL_CHOICE,
            {"type": "function", "function": {"name": "get_weather"}},
        )

    def test_checkpoint_identity_must_match_current_run(self):
        identity = soak.build_run_identity(
            "openrouter",
            "openrouter/auto",
            ["chat", "stream", "responses", "tools"],
            5.2,
            120,
        )
        soak.validate_checkpoint_identity({"run_identity": identity}, identity)
        self.assertEqual(identity["request_contract"], soak.request_contract())

        with self.assertRaises(ValueError):
            soak.validate_checkpoint_identity({}, identity)

        changed = dict(identity)
        changed["virtual_model"] = "kilo-auto/free"
        with self.assertRaises(ValueError):
            soak.validate_checkpoint_identity({"run_identity": changed}, identity)

        old_contract = copy.deepcopy(identity)
        old_contract["request_contract"]["responses_max_output_tokens"] = 20
        with self.assertRaises(ValueError):
            soak.validate_checkpoint_identity({"run_identity": old_contract}, identity)

    def test_runtime_reasons_and_request_ids_are_sanitized(self):
        self.assertEqual(
            soak.sanitize_runtime_reason('Post "https://upstream.example/v1": EOF'),
            "upstream unavailable",
        )
        self.assertEqual(soak.sanitize_runtime_reason("429 rate limited"), "rate limited")
        self.assertEqual(soak.sanitize_runtime_reason("unknown detail"), "runtime error")
        self.assertEqual(
            soak.extract_request_id({"X-Oneapi-Request-Id": "request-123"}),
            "request-123",
        )
        self.assertEqual(
            soak.extract_request_id({"x-request-id": "request-legacy"}),
            "request-legacy",
        )

    def test_http_error_preserves_gateway_request_id(self):
        error = soak.urllib.error.HTTPError(
            "http://127.0.0.1:3008/v1/chat/completions",
            429,
            "rate limited",
            {"Retry-After": "60", "X-Oneapi-Request-Id": "request-429"},
            io.BytesIO(b"{}"),
        )
        with mock.patch.object(soak.urllib.request, "urlopen", side_effect=error):
            record = soak.make_request(
                "test-token",
                "openrouter/auto",
                "chat",
                timeout=1,
            )

        self.assertEqual(record["status"], 429)
        self.assertEqual(record["request_id"], "request-429")
        self.assertEqual(record["retry_after"], "60")

    def test_runtime_degradation_snapshots_allowlist_and_clamp_safe_fields(self):
        snapshots = soak.parse_runtime_degradation_snapshots(
            {
                "success": True,
                "data": [
                    {
                        "deployment_id": "kilo/free-1",
                        "success_count": "7",
                        "access_token": "secret",
                        "raw_body": {"secret": "secret"},
                        "provider_rate_limit_degradation": {
                            "active": True,
                            "level": 99,
                            "episode_count": -3,
                            "consecutive_recovery_successes": True,
                            "reason": "repeated rate limits",
                            "last_rate_limited_at": "2026-07-16T01:02:03",
                            "next_recovery_at": "2026-07-16T01:02:03Z",
                        },
                    },
                    "not-a-row",
                    {"deployment_id": "invalid deployment id"},
                ],
            }
        )

        self.assertEqual(
            snapshots,
            [
                {
                    "deployment_id": "kilo/free-1",
                    "success_count": 7,
                    "active": True,
                    "level": 3,
                    "episode_count": 0,
                    "consecutive_recovery_successes": 0,
                    "reason": "repeated rate limits",
                    "next_recovery_at": "2026-07-16T01:02:03+00:00",
                }
            ],
        )
        self.assertNotIn("access_token", json.dumps(snapshots))
        self.assertNotIn("raw_body", json.dumps(snapshots))

    def test_runtime_degradation_snapshots_reject_invalid_top_level_shapes(self):
        with self.assertRaisesRegex(RuntimeError, "invalid data shape"):
            soak.parse_runtime_degradation_snapshots({"success": True, "data": {}})

    def test_successful_deployments_come_from_success_count_delta(self):
        pre_snapshots = [
            {"deployment_id": "kilo/free-1", "success_count": 4},
            {"deployment_id": "kilo/free-2", "success_count": 9},
        ]
        post_snapshots = [
            {
                "deployment_id": "kilo/free-1",
                "success_count": 5,
                "active": False,
                "level": 0,
                "episode_count": 0,
                "consecutive_recovery_successes": 0,
            },
            {
                "deployment_id": "kilo/free-2",
                "success_count": 9,
                "active": True,
                "level": 3,
                "episode_count": 2,
                "last_rate_limited_at": "2026-07-16T01:02:03+00:00",
            },
        ]

        result = soak.summarize_successful_deployment_degradation(
            pre_snapshots,
            post_snapshots,
        )

        self.assertEqual(result["deployment_ids"], ["kilo/free-1"])
        self.assertEqual(result["missing_deployment_ids"], [])
        self.assertTrue(result["all_level_zero"])
        self.assertTrue(result["no_retained_observations"])

    def test_degradation_validation_fails_closed_without_success_delta_snapshots(self):
        result = soak.summarize_successful_deployment_degradation([], [])

        self.assertEqual(result["deployment_ids"], [])
        self.assertFalse(result["all_level_zero"])
        self.assertFalse(result["no_retained_observations"])

    def test_degradation_validation_fails_closed_for_missing_snapshot(self):
        result = soak.summarize_successful_deployment_degradation(
            [{"deployment_id": "kilo/free-1", "success_count": 4}],
            [
                {
                    "deployment_id": "kilo/free-2",
                    "success_count": 5,
                    "active": False,
                    "level": 0,
                    "episode_count": 0,
                    "consecutive_recovery_successes": 0,
                }
            ],
        )

        self.assertEqual(
            result["missing_deployment_ids"],
            ["kilo/free-1", "kilo/free-2"],
        )
        self.assertFalse(result["all_level_zero"])
        self.assertFalse(result["no_retained_observations"])


if __name__ == "__main__":
    unittest.main()
