# FreeLLMAPI Native Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first FreeLLMAPI-native compatibility slice to cctapi by repairing malformed structured tool-call arguments before clients see responses.

**Architecture:** Keep cctapi's existing relay and fallback paths. Add pure helpers in `relay/model` for schema-gated argument repair, pass request tool schemas through request context, and apply repair in the OpenAI-compatible response handler before OpenAI, Responses, or Claude-format output is emitted.

**Tech Stack:** Go 1.22, Gin, cctapi relay/model package, cctapi OpenAI adaptor.

## Global Constraints

- Work on branch `feat/freellmapi-native-core`.
- Do not commit `one-api.db`, `.env`, `data/fallback.json`, `logs/`, or local backup files.
- Do not copy the FreeLLMAPI Node service into cctapi.
- Keep repair fail-open: malformed or unknown cases preserve the original response.
- Keep this slice non-streaming only; streaming tool-call repair is a later slice.
- Run `go test ./relay/model ./relay/adaptor/openai ./controller -count=1` before committing code.

---

### Task 1: Pure Tool Argument Repair Helpers

**Files:**
- Create: `relay/model/tool_args.go`
- Test: `relay/model/tool_args_test.go`

**Interfaces:**
- Consumes: existing `relay/model.Tool`.
- Produces: `RepairToolArguments(args string, paramSchema any) (string, bool)`.
- Produces: `RepairChatCompletionToolArgumentsJSON(body []byte, tools []Tool) ([]byte, bool, error)`.

- [ ] **Step 1: Write failing tests**

Add tests covering these exact cases:

```go
func TestRepairToolArgumentsUnwrapsWholeObjectString(t *testing.T) {
	got, changed := RepairToolArguments(`"{\"query\":\"hi\"}"`, map[string]any{
		"type": "object",
		"properties": map[string]any{"query": map[string]any{"type": "string"}},
	})
	if !changed || got != `{"query":"hi"}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairToolArgumentsRepairsNestedArrayBySchema(t *testing.T) {
	got, changed := RepairToolArguments(`{"steps":"[{\"step\":\"ship\"}]"}`, map[string]any{
		"type": "object",
		"properties": map[string]any{"steps": map[string]any{"type": "array"}},
	})
	if !changed || got != `{"steps":[{"step":"ship"}]}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairToolArgumentsPreservesStringSchema(t *testing.T) {
	got, changed := RepairToolArguments(`{"payload":"{\"keep\":\"string\"}"}`, map[string]any{
		"type": "object",
		"properties": map[string]any{"payload": map[string]any{"type": "string"}},
	})
	if changed || got != `{"payload":"{\"keep\":\"string\"}"}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairChatCompletionToolArgumentsJSONRepairsKnownToolOnly(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","object":"chat.completion","custom":"keep","choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"update_plan","arguments":"{\"steps\":\"[{\\\"step\\\":\\\"ship\\\"}]\"}"}},{"id":"call_2","type":"function","function":{"name":"unknown","arguments":"{\"steps\":\"[{\\\"step\\\":\\\"skip\\\"}]\"}"}}]}}]}`)
	tools := []Tool{{
		Type: "function",
		Function: Function{
			Name: "update_plan",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{"steps": map[string]any{"type": "array"}},
			},
		},
	}}
	repaired, changed, err := RepairChatCompletionToolArgumentsJSON(body, tools)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v body=%s", changed, err, repaired)
	}
	var decoded map[string]any
	if err := json.Unmarshal(repaired, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["custom"] != "keep" {
		t.Fatalf("custom field was dropped: %#v", decoded)
	}
	if !strings.Contains(string(repaired), `"arguments":"{\"steps\":[{\"step\":\"ship\"}]}"`) {
		t.Fatalf("known tool was not repaired: %s", repaired)
	}
	if !strings.Contains(string(repaired), `"name":"unknown"`) || !strings.Contains(string(repaired), `skip`) {
		t.Fatalf("unknown tool was not preserved: %s", repaired)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./relay/model -run "TestRepair" -count=1
```

Expected: fail because repair helpers do not exist.

- [ ] **Step 3: Implement helpers**

Implement `relay/model/tool_args.go` with:

```go
package model

import (
	"encoding/json"
	"strings"
)

func RepairToolArguments(args string, paramSchema any) (string, bool) {
	// Parse args, unwrap one whole-object double-encoded layer, then repair
	// object or array parameters only when the request schema requires that type.
}

func RepairChatCompletionToolArgumentsJSON(body []byte, tools []Tool) ([]byte, bool, error) {
	// Build tool schema map from tools, walk choices[].message.tool_calls[],
	// repair function.arguments for known tools, and marshal the body if changed.
}
```

The implementation must preserve the original body when:

- `tools` is empty.
- A tool call is missing `function.name`.
- A tool call name is unknown.
- `function.arguments` is not a string.
- Nested JSON is invalid or does not match the schema type.

- [ ] **Step 4: Run tests to verify pass**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./relay/model -run "TestRepair" -count=1
```

Expected: pass.

### Task 2: Relay Integration

**Files:**
- Modify: `common/ctxkey/key.go`
- Modify: `relay/controller/text.go`
- Modify: `relay/adaptor/openai/main.go`
- Test: `relay/adaptor/openai/adaptor_test.go`

**Interfaces:**
- Consumes: `ctxkey.RequestTools`.
- Consumes: `model.RepairChatCompletionToolArgumentsJSON`.
- Produces: repaired non-stream OpenAI chat response bodies for clients and capture writers.

- [ ] **Step 1: Add failing integration test**

Add a test to `relay/adaptor/openai/adaptor_test.go`:

```go
func TestHandlerRepairsToolArgumentsFromRequestToolsContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ctxkey.RequestTools, []model.Tool{{
		Type: "function",
		Function: model.Function{
			Name: "update_plan",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{"steps": map[string]any{"type": "array"}},
			},
		},
	}})
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"update_plan","arguments":"{\"steps\":\"[{\\\"step\\\":\\\"ship\\\"}]\"}"}}]}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)),
	}
	err, usage := Handler(c, resp, 1, "model")
	if err != nil {
		t.Fatalf("Handler returned error: %+v", err)
	}
	if usage == nil || usage.TotalTokens != 2 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if !strings.Contains(w.Body.String(), `"arguments":"{\"steps\":[{\"step\":\"ship\"}]}"`) {
		t.Fatalf("response body was not repaired: %s", w.Body.String())
	}
}
```

Add imports `io` and `strings` if missing.

- [ ] **Step 2: Run integration test to verify failure**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./relay/adaptor/openai -run TestHandlerRepairsToolArgumentsFromRequestToolsContext -count=1
```

Expected: fail because context key and handler repair are not wired.

- [ ] **Step 3: Wire request tools into context**

Add `RequestTools = "request_tools"` to `common/ctxkey/key.go`.

In `relay/controller/text.go`, after `textRequest` is validated and before dispatch:

```go
if len(textRequest.Tools) > 0 {
	c.Set(ctxkey.RequestTools, append([]model.Tool(nil), textRequest.Tools...))
} else if c.Keys != nil {
	delete(c.Keys, ctxkey.RequestTools)
}
```

- [ ] **Step 4: Apply repair in OpenAI handler**

In `relay/adaptor/openai/main.go`, import `github.com/songquanpeng/one-api/common/ctxkey`.

After unmarshalling `textResponse` and before resetting/writing `resp.Body`, add:

```go
if requestToolsValue, ok := c.Get(ctxkey.RequestTools); ok {
	if requestTools, ok := requestToolsValue.([]model.Tool); ok && len(requestTools) > 0 {
		if repairedBody, changed, repairErr := model.RepairChatCompletionToolArgumentsJSON(responseBody, requestTools); repairErr != nil {
			logger.SysError("error repairing tool call arguments: " + repairErr.Error())
		} else if changed {
			responseBody = repairedBody
		}
	}
}
```

Repair errors are logged and ignored, preserving fail-open behavior.

- [ ] **Step 5: Run integration tests**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./relay/model ./relay/adaptor/openai ./controller -count=1
```

Expected: pass.

### Task 3: Verification And Commit

**Files:**
- Commit code/test changes only.

**Interfaces:**
- Produces: one code commit after tests pass.

- [ ] **Step 1: Run targeted verification**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./relay/model ./relay/adaptor/openai ./controller -count=1
```

Expected: pass.

- [ ] **Step 2: Run broader backend verification**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 3: Check Git status**

Run:

```powershell
git status --short
git check-ignore -v one-api.db data/fallback.json logs/
```

Expected: no local database, config, or logs in tracked changes.

- [ ] **Step 4: Commit**

Run:

```powershell
git add common/ctxkey/key.go relay/controller/text.go relay/adaptor/openai/main.go relay/adaptor/openai/adaptor_test.go relay/model/tool_args.go relay/model/tool_args_test.go
git commit -m "fix: repair structured tool call arguments"
```

Expected: commit succeeds.
