import importlib.util
import sqlite3
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("soak-test.py")
SPEC = importlib.util.spec_from_file_location("soak_test_script", SCRIPT_PATH)
soak = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(soak)


class SoakScriptTests(unittest.TestCase):
    def test_text_requests_allow_visible_output_budget(self):
        self.assertGreaterEqual(soak.TEXT_MAX_TOKENS, 128)

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
            {"id": "resp_1", "model": "upstream/model", "output": [{"type": "message"}]},
        )
        self.assertTrue(responses["protocol_valid"])
        self.assertTrue(responses["response_id_present"])
        self.assertEqual(responses["response_output_count"], 1)

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
            conn.execute(
                """
                INSERT INTO attempt_events (
                    created_at, request_id, virtual_model, provider,
                    real_model, outcome, status_code
                ) VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "2026-07-15 06:44:42.2219578+00:00",
                    "request-full-id",
                    "openrouter/auto",
                    "kilo",
                    "cohere/north-mini-code:free",
                    "model_rate_limited",
                    429,
                ),
            )
            conn.commit()
            conn.close()

            result = soak.check_attempt_events(
                "openrouter/auto",
                "2026-07-15T06:40:00+00:00",
                "2026-07-15T06:50:00+00:00",
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


if __name__ == "__main__":
    unittest.main()
