import json
import subprocess
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


SCRIPT = Path(__file__).with_name("fallback-production-soak.ps1")


class MockGatewayHandler(BaseHTTPRequestHandler):
    def log_message(self, _format, *_args):
        pass

    def send_json(self, value):
        payload = json.dumps(value).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self):
        self.send_json({
            "success": True,
            "data": {
                "failure_event_count": 0,
                "skip_event_count": 0,
                "top_providers": [],
                "top_models": [],
                "error_categories": [],
                "outcomes": [],
                "recent_chains": [],
            },
        })

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length) or b"{}")
        if body.get("stream"):
            payload = b'data: {"choices":[{"delta":{"content":"ok"}}]}\n\ndata: [DONE]\n\n'
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
        elif body.get("tool_choice"):
            self.send_json({
                "choices": [{
                    "message": {
                        "tool_calls": [{
                            "type": "function",
                            "function": {
                                "name": "get_time",
                                "arguments": '"TOP-SECRET-UPSTREAM"',
                            },
                        }],
                    },
                }],
            })
        else:
            self.send_json({"choices": [{"message": {"content": "ok"}}]})


class ProductionSoakHarnessContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = SCRIPT.read_text(encoding="utf-8")

    def run_script(self, *arguments):
        return subprocess.run(
            [
                "powershell.exe",
                "-NoProfile",
                "-ExecutionPolicy",
                "Bypass",
                "-File",
                str(SCRIPT),
                "-ApiToken",
                "test-api-token",
                "-AdminToken",
                "test-admin-token",
                *arguments,
            ],
            capture_output=True,
            text=True,
            check=False,
        )

    def test_rejects_zero_interval_before_network_access(self):
        result = self.run_script("-IntervalSec", "0")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("IntervalSec must be greater than 0", result.stdout + result.stderr)

    def test_rejects_base_url_credentials_before_network_access(self):
        result = self.run_script("-BaseUrl", "http://user:secret@127.0.0.1:1/?token=hidden")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("BaseUrl must not contain credentials", result.stdout + result.stderr)

    def test_invalid_required_tool_arguments_count_failure_and_stay_sanitized(self):
        server = ThreadingHTTPServer(("127.0.0.1", 0), MockGatewayHandler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory() as temp:
                result = self.run_script(
                    "-BaseUrl", f"http://127.0.0.1:{server.server_port}",
                    "-DurationMin", "1",
                    "-IntervalSec", "1",
                    "-SnapshotEvery", "100",
                    "-MaxRequests", "7",
                    "-MinSuccessRate", "0.95",
                    "-OutputDir", temp,
                    "-IncludeTools",
                    "-OutputJson",
                )

                self.assertNotEqual(result.returncode, 0)
                evidence_files = [
                    path for path in Path(temp).glob("production-soak-*.json")
                    if path.name != "production-soak-partial.json"
                ]
                self.assertEqual(len(evidence_files), 1)
                evidence_text = evidence_files[0].read_text(encoding="utf-8")
                evidence = json.loads(evidence_text)
                self.assertEqual(evidence["summary"]["total_requests"], 7)
                self.assertEqual(evidence["summary"]["success_count"], 6)
                self.assertEqual(evidence["summary"]["failure_count"], 1)
                self.assertEqual(evidence["summary"]["required_tool_requests"], 0)
                self.assertEqual(evidence["request_errors"][0]["category"], "protocol_error")
                self.assertNotIn("TOP-SECRET-UPSTREAM", evidence_text)
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)

    def test_failure_and_required_tool_contracts_are_explicit(self):
        self.assertNotIn('$body["tool_choice"] = "auto"', self.source)
        self.assertIn("required_tool_requests", self.source)
        self.assertIn("message.tool_calls", self.source)
        self.assertGreaterEqual(self.source.count("$stats.failureCount++"), 3)
        self.assertNotIn("error = $msg", self.source)


if __name__ == "__main__":
    unittest.main()
