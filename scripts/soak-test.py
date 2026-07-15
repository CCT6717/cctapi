#!/usr/bin/env python3
"""
Paced soak test for cctapi fallback routing.

Usage:
    python scripts/soak-test.py --provider kilo --count 100 --delay 5.5
    python scripts/soak-test.py --provider openrouter --count 100 --delay 5.5
    python scripts/soak-test.py --provider ovh --model <configured-virtual-model> --count 100

Features:
- Mixed request types: chat, stream, responses, tools
- Records every attempt with latency, status, model, error category
- No client retry (to observe gateway behavior)
- Resumable via --resume-from
- Generates sanitized JSON evidence
"""

import argparse
import json
import math
import random
import sqlite3
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

BASE_URL = "http://127.0.0.1:3008"
DB_PATH = Path("one-api.db")

PROMPTS = [
    "Say exactly OK.",
    "Count from 1 to 3.",
    "What is 2+2?",
    "Reply with a single word: hello.",
    "List three colors.",
]

DEFAULT_MODELS = {
    "kilo": "kilo-auto/free",
    "openrouter": "openrouter/auto",
}
TEXT_MAX_TOKENS = 256


def resolve_model(provider, explicit_model):
    if explicit_model:
        return explicit_model
    if provider in DEFAULT_MODELS:
        return DEFAULT_MODELS[provider]
    raise ValueError(
        f"Provider {provider!r} has no validated default virtual model; pass --model explicitly"
    )


def percentile(values, fraction):
    if not values:
        return 0
    ordered = sorted(values)
    index = max(0, math.ceil(len(ordered) * fraction) - 1)
    return ordered[index]


def request_type_for_index(type_pool, index):
    if not type_pool:
        raise ValueError("At least one request type is required")
    return type_pool[index % len(type_pool)]


def parse_stream_response(body):
    text = body.decode("utf-8", errors="replace")
    frame_count = 0
    done = False
    content_present = False
    real_model = None

    for line in text.splitlines():
        if not line.startswith("data: "):
            continue
        payload = line[6:].strip()
        if payload == "[DONE]":
            done = True
            continue
        try:
            frame = json.loads(payload)
        except (TypeError, ValueError):
            continue
        frame_count += 1
        if frame.get("model"):
            real_model = frame["model"]
        for choice in frame.get("choices", []):
            delta = choice.get("delta", {})
            if delta.get("content"):
                content_present = True

    return {
        "protocol_valid": frame_count > 0 and done and content_present,
        "stream_frame_count": frame_count,
        "stream_done": done,
        "content_present": content_present,
        "real_model": real_model,
    }


def parse_json_response(req_type, data):
    result = {
        "protocol_valid": False,
        "content_present": False,
        "tool_call_count": 0,
        "tool_arguments_valid_json": False,
        "response_id_present": False,
        "response_output_count": 0,
        "real_model": data.get("model"),
    }

    if req_type == "responses":
        output = data.get("output") or []
        result["response_id_present"] = bool(data.get("id"))
        result["response_output_count"] = len(output)
        result["content_present"] = bool(output)
        result["protocol_valid"] = result["response_id_present"] and bool(output)
        return result

    choices = data.get("choices") or []
    if req_type == "tools":
        tool_calls = []
        if choices:
            tool_calls = choices[0].get("message", {}).get("tool_calls") or []
        arguments_valid = bool(tool_calls)
        for call in tool_calls:
            try:
                json.loads(call.get("function", {}).get("arguments", ""))
            except (TypeError, ValueError):
                arguments_valid = False
                break
        result["tool_call_count"] = len(tool_calls)
        result["tool_arguments_valid_json"] = arguments_valid
        result["content_present"] = bool(tool_calls)
        result["protocol_valid"] = bool(tool_calls) and arguments_valid
        return result

    content = ""
    if choices:
        content = choices[0].get("message", {}).get("content") or ""
    result["content_present"] = bool(str(content).strip())
    result["protocol_valid"] = bool(choices) and result["content_present"]
    return result


def safe_error_reason(status):
    if status == 401:
        return "authentication failed"
    if status == 403:
        return "model access denied"
    if status == 429:
        return "rate limited"
    if status == -1:
        return "network failure"
    if status is not None and status >= 500:
        return "upstream unavailable"
    if status is not None and status >= 400:
        return "request rejected"
    return None


def get_token():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("SELECT key FROM tokens WHERE status = 1 LIMIT 1")
    row = cursor.fetchone()
    conn.close()
    if not row:
        raise RuntimeError("No active token found in database")
    return row[0]


def make_request(token, model, req_type, timeout=30):
    start = time.perf_counter()
    req_id = None
    status = None
    real_model = None
    retry_after = None
    validation = {
        "protocol_valid": False,
        "content_present": False,
        "stream_frame_count": 0,
        "stream_done": False,
        "tool_call_count": 0,
        "tool_arguments_valid_json": False,
        "response_id_present": False,
        "response_output_count": 0,
    }

    if req_type == "chat":
        payload = {
            "model": model,
            "messages": [{"role": "user", "content": random.choice(PROMPTS)}],
            "max_tokens": TEXT_MAX_TOKENS,
        }
        req = urllib.request.Request(
            f"{BASE_URL}/v1/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {token}"},
        )
    elif req_type == "stream":
        payload = {
            "model": model,
            "messages": [{"role": "user", "content": random.choice(PROMPTS)}],
            "max_tokens": TEXT_MAX_TOKENS,
            "stream": True,
        }
        req = urllib.request.Request(
            f"{BASE_URL}/v1/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {token}"},
        )
    elif req_type == "tools":
        payload = {
            "model": model,
            "messages": [{"role": "user", "content": "What is the weather in Tokyo?"}],
            "tools": [
                {
                    "type": "function",
                    "function": {
                        "name": "get_weather",
                        "description": "Get weather for a city",
                        "parameters": {
                            "type": "object",
                            "properties": {"city": {"type": "string"}},
                            "required": ["city"],
                        },
                    },
                }
            ],
            "max_tokens": 50,
        }
        req = urllib.request.Request(
            f"{BASE_URL}/v1/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {token}"},
        )
    elif req_type == "responses":
        payload = {
            "model": model,
            "input": random.choice(PROMPTS),
            "max_output_tokens": 20,
        }
        req = urllib.request.Request(
            f"{BASE_URL}/v1/responses",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "Authorization": f"Bearer {token}"},
        )
    else:
        raise ValueError(f"Unknown req_type: {req_type}")

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            status = resp.status
            body = resp.read()
            headers = dict(resp.headers)
            retry_after = headers.get("Retry-After")

            if req_type == "stream":
                validation.update(parse_stream_response(body))
                real_model = validation.pop("real_model", None)
            else:
                try:
                    data = json.loads(body)
                    validation.update(parse_json_response(req_type, data))
                    real_model = validation.pop("real_model", None)
                except (TypeError, ValueError):
                    validation["protocol_valid"] = False

            # Try to extract request ID from headers
            req_id = headers.get("X-Request-Id", headers.get("x-request-id", None))

    except urllib.error.HTTPError as e:
        status = e.code
        if hasattr(e, "read"):
            e.read()
        retry_after = e.headers.get("Retry-After") if e.headers else None
    except Exception:
        status = -1

    latency_ms = round((time.perf_counter() - start) * 1000, 2)

    record = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "type": req_type,
        "virtual_model": model,
        "real_model": real_model,
        "status": status,
        "latency_ms": latency_ms,
        "safe_error_reason": safe_error_reason(status),
        "retry_after": retry_after,
        "request_id": req_id,
    }
    record.update(validation)
    return record


def classify_outcome(record):
    status = record["status"]
    if status == 200:
        return "success" if record.get("protocol_valid") else "protocol_error"
    if status == 429:
        return "rate_limited"
    if status == 401:
        return "auth_error"
    if status == 500:
        return "server_error"
    if status == -1:
        return "network_error"
    if 400 <= status < 500:
        return "client_error"
    if status >= 500:
        return "server_error"
    return "unknown"


def check_deployment_state(provider):
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    today = datetime.now().strftime("%Y-%m-%d")
    
    # Find deployment states for provider (uses request_count / success_count / error_count)
    results = {}
    cursor.execute(
        "SELECT deployment_id, request_count, success_count, error_count, cooldown_until, exhausted_until, last_error_code FROM deployment_states WHERE date = ? AND deployment_id LIKE ?",
        (today, f"%{provider}%",)
    )
    for row in cursor.fetchall():
        results[row[0]] = {
            "request_count": row[1],
            "success_count": row[2],
            "error_count": row[3],
            "cooldown_until": row[4],
            "exhausted_until": row[5],
            "last_error_code": row[6],
        }
    
    # Check deployment_cooldown_states (not model_cooldowns)
    cursor.execute(
        "SELECT deployment_id, reason, cooldown_until FROM deployment_cooldown_states WHERE cooldown_until > datetime('now') AND deployment_id LIKE ?",
        (f"%{provider}%",)
    )
    cooldowns = []
    for row in cursor.fetchall():
        cooldowns.append({
            "deployment_id": row[0],
            "reason": row[1],
            "cooldown_until": row[2],
        })
    
    conn.close()
    return results, cooldowns


def check_attempt_events(virtual_model, since_iso, until_iso, db_path=DB_PATH):
    """Verify attempt_events consistency after soak."""
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    cursor.execute(
        """
        SELECT COUNT(*) FROM attempt_events
        WHERE virtual_model = ?
          AND datetime(created_at) >= datetime(?)
          AND datetime(created_at) <= datetime(?)
        """,
        (virtual_model, since_iso, until_iso),
    )
    count = cursor.fetchone()[0]

    cursor.execute(
        """
        SELECT COUNT(DISTINCT request_id) FROM attempt_events
        WHERE virtual_model = ?
          AND datetime(created_at) >= datetime(?)
          AND datetime(created_at) <= datetime(?)
        """,
        (virtual_model, since_iso, until_iso),
    )
    distinct_requests = cursor.fetchone()[0]

    cursor.execute(
        """
        SELECT provider, outcome, COUNT(*) FROM attempt_events
        WHERE virtual_model = ?
          AND datetime(created_at) >= datetime(?)
          AND datetime(created_at) <= datetime(?)
        GROUP BY provider, outcome
        ORDER BY provider, outcome
        """,
        (virtual_model, since_iso, until_iso),
    )
    breakdown = {}
    for provider, outcome, outcome_count in cursor.fetchall():
        breakdown.setdefault(provider, {})[outcome] = outcome_count

    cursor.execute(
        """
        SELECT request_id, provider, real_model, outcome, status_code,
               plan_index, upstream_attempt_index
        FROM attempt_events
        WHERE virtual_model = ?
          AND datetime(created_at) >= datetime(?)
          AND datetime(created_at) <= datetime(?)
        ORDER BY created_at, id
        """,
        (virtual_model, since_iso, until_iso),
    )
    request_paths = {}
    for row in cursor.fetchall():
        request_paths.setdefault(row[0], []).append(
            {
                "provider": row[1],
                "real_model": row[2],
                "outcome": row[3],
                "status_code": row[4],
                "plan_index": row[5],
                "upstream_attempt_index": row[6],
            }
        )

    conn.close()
    return {
        "count": count,
        "distinct_requests": distinct_requests,
        "breakdown": breakdown,
        "request_paths": request_paths,
    }


def run_soak(args):
    token = get_token()
    model = resolve_model(args.provider, args.model)
    
    # Determine request type mix
    if args.types:
        type_pool = args.types.split(",")
    else:
        type_pool = ["chat", "chat", "chat", "stream", "tools"]
        if args.provider == "kilo":
            type_pool.append("responses")
    
    records = []
    state_checks = []
    checkpoint_owned = False
    checkpoint_written = False
    soak_start_iso = datetime.now(timezone.utc).isoformat()

    # Load resume point
    resume_from = 0
    if args.resume_from and args.resume_file.exists():
        with open(args.resume_file, "r", encoding="utf-8") as f:
            prev_data = json.load(f)
        records = prev_data.get("records", [])
        state_checks = prev_data.get("state_checks", [])
        resume_from = len(records)
        soak_start_iso = prev_data.get("soak_start_iso", soak_start_iso)
        checkpoint_owned = True
        print(f"Resuming from request {resume_from + 1}")
    
    # Pre-soak state snapshot
    pre_states, pre_cooldowns = check_deployment_state(args.provider)
    
    print(f"Starting soak: {args.count} requests to {model}")
    print(f"Delay: {args.delay}s | Types: {type_pool}")
    print(f"Pre-soak state: {len(pre_states)} deployment states, {len(pre_cooldowns)} deployment cooldowns")
    
    for i in range(resume_from, args.count):
        req_type = request_type_for_index(type_pool, i)
        print(f"[{i+1}/{args.count}] {req_type} ...", end=" ", flush=True)
        
        record = make_request(token, model, req_type, timeout=args.timeout)
        record["outcome"] = classify_outcome(record)
        records.append(record)

        outcome = record["outcome"]
        print(f"{record['status']} {outcome} {record['latency_ms']}ms", end="")
        if record.get("real_model"):
            print(f" [{record['real_model']}]", end="")
        if record.get("retry_after"):
            print(f" retry_after={record['retry_after']}", end="")
        print()
        
        # Periodic state check every 10 requests
        if (i + 1) % 10 == 0:
            states, cooldowns = check_deployment_state(args.provider)
            state_checks.append({
                "after_request": i + 1,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "deployment_states": states,
                "deployment_cooldowns": cooldowns,
            })
            print(f"  -> State check: {len(states)} states, {len(cooldowns)} deployment cooldowns")
        
        # Save checkpoint every 20 requests
        if (i + 1) % 20 == 0 and args.resume_file:
            # Convert args to serializable dict (Path objects -> str)
            serializable_args = {k: str(v) if isinstance(v, Path) else v for k, v in vars(args).items()}
            with open(args.resume_file, "w", encoding="utf-8") as f:
                json.dump({
                    "records": records,
                    "state_checks": state_checks,
                    "args": serializable_args,
                    "checkpoint_at": i + 1,
                    "soak_start_iso": soak_start_iso,
                    "virtual_model": model,
                }, f, indent=2, ensure_ascii=False)
            checkpoint_written = True
            print(f"  -> Checkpoint saved")
        
        # Delay before next request (except last)
        if i < args.count - 1:
            actual_delay = args.delay + random.uniform(-0.5, 0.5)
            time.sleep(max(0.1, actual_delay))
    
    # Post-soak state
    soak_end_iso = datetime.now(timezone.utc).isoformat()
    post_states, post_cooldowns = check_deployment_state(args.provider)

    # Analysis
    success_count = sum(1 for r in records if r["outcome"] == "success")
    http_200_count = sum(1 for r in records if r["status"] == 200)
    protocol_error_count = sum(1 for r in records if r["outcome"] == "protocol_error")
    rate_limit_count = sum(1 for r in records if r["outcome"] == "rate_limited")
    error_count = len(records) - success_count - rate_limit_count - protocol_error_count
    latencies = [r["latency_ms"] for r in records if r["outcome"] == "success"]
    avg_latency = round(sum(latencies) / len(latencies), 2) if latencies else 0
    p95_latency = round(percentile(latencies, 0.95)) if latencies else 0

    real_models = {}
    for r in records:
        if r.get("real_model"):
            real_models[r["real_model"]] = real_models.get(r["real_model"], 0) + 1
    
    summary = {
        "provider": args.provider,
        "virtual_model": model,
        "total_requests": len(records),
        "success": success_count,
        "http_200": http_200_count,
        "protocol_errors": protocol_error_count,
        "rate_limited": rate_limit_count,
        "other_errors": error_count,
        "success_rate": round(success_count / len(records) * 100, 2) if records else 0,
        "avg_latency_ms": avg_latency,
        "p95_latency_ms": p95_latency,
        "real_models_used": real_models,
        "pre_deployment_states": pre_states,
        "post_deployment_states": post_states,
        "pre_deployment_cooldowns": pre_cooldowns,
        "post_deployment_cooldowns": post_cooldowns,
        "validation_contract": {
            "chat": "HTTP 200, JSON choices, and non-empty assistant content",
            "stream": "HTTP 200, parsed SSE frames, content, and [DONE]",
            "responses": "HTTP 200, response id, and non-empty output",
            "tools": "HTTP 200, tool_calls, and valid JSON arguments",
        },
    }

    # Verify attempt_events consistency
    attempt_events = check_attempt_events(model, soak_start_iso, soak_end_iso)
    summary["attempt_events_verification"] = attempt_events
    
    report = {
        "meta": {
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "version": "soak-v1",
            "sensitive": False,
            "soak_start": soak_start_iso,
            "soak_end": soak_end_iso,
            "protocol_validation": "strict",
        },
        "summary": summary,
        "records": records,
        "state_checks": state_checks,
    }
    
    # Save report
    report_path = Path(args.output)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2, ensure_ascii=False)
    
    # Also update resume file if exists
    if (checkpoint_owned or checkpoint_written) and args.resume_file.exists():
        args.resume_file.unlink()
    
    print(f"\n{'='*50}")
    print(f"Soak complete for {args.provider}")
    print(f"Success: {success_count}/{len(records)} ({summary['success_rate']}%)")
    print(f"Rate limited: {rate_limit_count}")
    print(f"Protocol errors: {protocol_error_count}")
    print(f"Other errors: {error_count}")
    print(f"Avg latency: {avg_latency}ms")
    print(f"P95 latency: {p95_latency}ms")
    print(f"Real models used: {len(real_models)}")
    for m, c in sorted(real_models.items(), key=lambda x: -x[1]):
        print(f"  {m}: {c}")
    print(f"Report saved: {report_path}")
    print(f"{'='*50}")
    
    return (
        summary["success_rate"] >= 99.0
        and rate_limit_count == 0
        and protocol_error_count == 0
        and error_count == 0
    )


def main():
    parser = argparse.ArgumentParser(description="Paced soak test for cctapi")
    parser.add_argument("--provider", required=True, choices=["kilo", "ovh", "pollinations", "openrouter"])
    parser.add_argument(
        "--model",
        default="",
        help="Explicit virtual model; required when the provider has no validated default",
    )
    parser.add_argument("--count", type=int, default=100, help="Total requests (default 100)")
    parser.add_argument("--delay", type=float, default=5.5, help="Base delay between requests in seconds (default 5.5)")
    parser.add_argument("--types", default="", help="Comma-separated request types: chat,stream,tools,responses")
    parser.add_argument("--timeout", type=int, default=30, help="Request timeout in seconds")
    parser.add_argument("--output", default="docs/evidence/soak-{provider}-{timestamp}.json", help="Output report path")
    parser.add_argument("--resume-file", default=".soak-checkpoint.json", help="Checkpoint file for resuming")
    parser.add_argument("--resume-from", action="store_true", help="Resume from checkpoint if exists")
    
    args = parser.parse_args()
    
    # Format output path
    if "{provider}" in args.output:
        args.output = args.output.replace("{provider}", args.provider)
    if "{timestamp}" in args.output:
        args.output = args.output.replace("{timestamp}", datetime.now().strftime("%Y-%m-%d"))
    
    args.resume_file = Path(args.resume_file)
    
    # Change to project root if in scripts/
    script_dir = Path(__file__).parent.resolve()
    project_root = script_dir.parent
    if (project_root / "one-api.db").exists():
        import os
        os.chdir(project_root)
    
    try:
        ok = run_soak(args)
    except ValueError as exc:
        parser.error(str(exc))
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
