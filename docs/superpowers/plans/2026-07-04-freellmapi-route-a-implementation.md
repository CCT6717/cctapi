# FreeLLMAPI Route A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first native FreeLLMAPI compatibility slice to cctapi: `/v1/responses`, safer free-provider saves, usage visibility, and better free-pool health probes.

**Architecture:** Keep cctapi's existing One API relay and fallback planner as the execution path. Add pure translation helpers in `relay/model`, add route-level buffering shims in the top-level `controller` package, keep gateway validation in `router`, keep provider/runtime logic in `fallback`, and limit the UI work to `web/default`.

**Tech Stack:** Go 1.22, Gin, GORM, SQLite-backed tests, existing One API relay/adaptor system, React 18 and Semantic UI under `web/default`.

## Global Constraints

- Work on branch `cleanup/structure-boundaries`.
- Keep `cct/free`, `cct/high`, and `cct/low` behavior unchanged.
- Do not add a second FreeLLMAPI proxy process.
- Do not implement Assistants, Threads, files, fine-tuning, or vector stores.
- Do not build the encrypted key store in this slice; existing plaintext `free_providers.keys` remains readable for compatibility.
- Do not build signed remote catalog sync in this slice.
- Do not add image input support to `/v1/responses`; image input continues to use `/v1/chat/completions`.
- Never return raw free-provider keys through gateway responses, usage responses, logs, docs, or tests.
- Use TDD for behavior changes: write a failing test, run it red, implement the minimal code, run it green, then refactor.
- Use the Go toolchain with `D:\ct\tools\go1.22.12\bin` and `D:\ct\tools\w64devkit-1.23.0\bin` first in `PATH`; set `CGO_ENABLED=1`.

---

## File Structure

- Create: `relay/model/responses.go`
  - Owns Responses API request structs, output structs, error type, chat request conversion, non-stream response conversion, and SSE conversion helpers.
- Create: `relay/model/responses_test.go`
  - Pure tests for input conversion, unsupported content rejection, chat response conversion, and stream event conversion.
- Modify: `relay/relaymode/define.go`
  - Adds `Responses` relay mode for route discovery and future metrics.
- Modify: `relay/relaymode/helper.go`
  - Maps `/v1/responses` to `relaymode.Responses`.
- Create: `relay/relaymode/helper_test.go`
  - Locks `/v1/responses` path detection.
- Create: `controller/responses.go`
  - Adds `RelayResponses(c *gin.Context)`, local response-capture writer, path/body rewrite into chat completions, and response conversion back to Responses format.
- Create: `controller/responses_test.go`
  - Tests capture writer behavior and direct conversion through the controller helper without touching provider network.
- Modify: `middleware/utils.go`
  - Treats `/v1/responses` as a model-checked endpoint so token model allowlists still apply.
- Modify: `router/relay.go`
  - Registers `POST /v1/responses`.
- Create: `router/relay_test.go`
  - Verifies the route is present.
- Modify: `router/fallback_gateway_types.go`
  - Adds explicit `clear_keys` input semantics.
- Modify: `router/fallback_gateway_projection.go`
  - Adds provider-name validation, key operation merging, and key requirements.
- Modify: `router/fallback_gateway.go`
  - Calls the gateway free-provider validation before saving both full and manual gateway config payloads.
- Modify: `router/fallback_gateway_test.go`
  - Adds save validation, key replacement, key preservation, key clearing, and raw-key safety tests.
- Modify: `fallback/free_provider_ledger.go`
  - Adds list/query support over existing `FreeProviderUsageLedger`.
- Modify: `fallback/free_provider_ledger_test.go`
  - Adds list/filter/no-key-exposure tests.
- Create: `router/fallback_usage.go`
  - Adds `getFreePoolUsage(c *gin.Context)`.
- Create: `router/fallback_usage_test.go`
  - Tests API filtering and empty rows.
- Modify: `router/fallback.go`
  - Registers `GET /api/fallback/free-pool/usage`.
- Modify: `fallback/health.go`
  - Adds provider-aware health probe request helpers and keyless/auth/header/body quirks.
- Create: `fallback/health_test.go`
  - Tests URL suffix, keyless auth omission, user-agent quirk, max-token quirk, and health status mapping.
- Modify: `web/default/src/components/fallback-gateway/gatewayConfigApi.js`
  - Adds `getFreePoolUsage`.
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.js`
  - Adds usage-row indexing helpers and key-clear payload helpers.
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.test.js`
  - Tests usage indexing and key-clear payload semantics.
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
  - Passes key-clear handlers to provider rows.
- Modify: `web/default/src/components/fallback-gateway/FreeProviderRow.js`
  - Adds explicit clear-key control and usage summary cells.
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.js`
  - Loads usage rows and renders a compact read-only usage table.
- Modify: `docs/freellmapi-route-a.md`
  - Adds operator-facing notes for `/v1/responses`, key save semantics, usage API, and health probe behavior.
- Modify: `scripts/fallback-smoke.ps1`
  - Adds parser-safe smoke examples for `/v1/responses` and usage endpoint checks.

---

## Task 1: Responses Request and Response Translation

**Files:**
- Create: `relay/model/responses.go`
- Create: `relay/model/responses_test.go`

**Interfaces:**
- Consumes: `GeneralOpenAIRequest`, `Message`, `Tool`, `Usage`.
- Produces: `type ResponsesRequest struct`.
- Produces: `func (r ResponsesRequest) ToChatRequest() (*GeneralOpenAIRequest, error)`.
- Produces: `func ChatCompletionToResponses(body []byte, fallbackModel string) (*ResponsesObject, int, error)`.
- Produces: `func UnsupportedResponsesInputError(message string) error`.

- [ ] **Step 1: Write failing request conversion tests**

Add to `relay/model/responses_test.go`:

```go
package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestResponsesRequestToChatRequestConvertsStringInput(t *testing.T) {
	temp := 0.2
	maxOutput := 64
	req := ResponsesRequest{
		Model:           "cct/free",
		Instructions:    "answer shortly",
		Input:           "ping",
		Temperature:     &temp,
		MaxOutputTokens: &maxOutput,
		Stream:          true,
	}

	chat, err := req.ToChatRequest()
	if err != nil {
		t.Fatalf("ToChatRequest returned error: %v", err)
	}
	if chat.Model != "cct/free" || !chat.Stream {
		t.Fatalf("unexpected model/stream: model=%q stream=%v", chat.Model, chat.Stream)
	}
	if chat.MaxTokens != 64 {
		t.Fatalf("expected max_tokens 64, got %d", chat.MaxTokens)
	}
	if chat.Temperature == nil || *chat.Temperature != 0.2 {
		t.Fatalf("expected temperature 0.2, got %#v", chat.Temperature)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("expected system and user messages, got %#v", chat.Messages)
	}
	if chat.Messages[0].Role != "system" || chat.Messages[0].Content != "answer shortly" {
		t.Fatalf("unexpected system message: %#v", chat.Messages[0])
	}
	if chat.Messages[1].Role != "user" || chat.Messages[1].Content != "ping" {
		t.Fatalf("unexpected user message: %#v", chat.Messages[1])
	}
}

func TestResponsesRequestToChatRequestConvertsMessageInput(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"hello"}]},
			{"role":"assistant","content":[{"type":"output_text","text":"hi"}]}
		]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	chat, err := req.ToChatRequest()
	if err != nil {
		t.Fatalf("ToChatRequest returned error: %v", err)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("expected two messages, got %#v", chat.Messages)
	}
	if chat.Messages[0].Role != "user" || chat.Messages[0].StringContent() != "hello" {
		t.Fatalf("unexpected first message: %#v", chat.Messages[0])
	}
	if chat.Messages[1].Role != "assistant" || chat.Messages[1].StringContent() != "hi" {
		t.Fatalf("unexpected second message: %#v", chat.Messages[1])
	}
}

func TestResponsesRequestToChatRequestRejectsImageInput(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/a.png"}]}]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	var unsupported *ResponsesUnsupportedInputError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected ResponsesUnsupportedInputError, got %T %v", err, err)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
go test ./relay/model -run "TestResponsesRequestToChatRequest" -count=1
```

Expected: FAIL because `ResponsesRequest` and `ResponsesUnsupportedInputError` do not exist.

- [ ] **Step 3: Implement request conversion**

Create `relay/model/responses.go` with this starting implementation:

```go
package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ResponsesUnsupportedInputError struct {
	Message string
}

func (e *ResponsesUnsupportedInputError) Error() string {
	return e.Message
}

func UnsupportedResponsesInputError(message string) error {
	return &ResponsesUnsupportedInputError{Message: message}
}

type ResponsesRequest struct {
	Model             string   `json:"model"`
	Input             any      `json:"input"`
	Instructions      string   `json:"instructions,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	MaxOutputTokens   *int     `json:"max_output_tokens,omitempty"`
	Tools             []Tool   `json:"tools,omitempty"`
	ToolChoice        any      `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool    `json:"parallel_tool_calls,omitempty"`
	Stream            bool     `json:"stream,omitempty"`
	Store             *bool    `json:"store,omitempty"`
	Metadata          any      `json:"metadata,omitempty"`
	User              string   `json:"user,omitempty"`
}

func (r ResponsesRequest) ToChatRequest() (*GeneralOpenAIRequest, error) {
	messages, err := responsesInputToMessages(r.Input)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.Instructions) != "" {
		messages = append([]Message{{
			Role:    "system",
			Content: strings.TrimSpace(r.Instructions),
		}}, messages...)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("field input is required")
	}
	req := &GeneralOpenAIRequest{
		Model:            r.Model,
		Messages:         messages,
		Temperature:      r.Temperature,
		TopP:             r.TopP,
		Tools:            append([]Tool{}, r.Tools...),
		ToolChoice:       r.ToolChoice,
		ParallelTooCalls: r.ParallelToolCalls,
		Stream:           r.Stream,
		Store:            r.Store,
		Metadata:         r.Metadata,
		User:             r.User,
	}
	if r.MaxOutputTokens != nil {
		req.MaxTokens = *r.MaxOutputTokens
	}
	return req, nil
}

func responsesInputToMessages(input any) ([]Message, error) {
	switch value := input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("field input is required")
		}
		return []Message{{Role: "user", Content: value}}, nil
	case []any:
		out := make([]Message, 0, len(value))
		for _, item := range value {
			msg, err := responseInputItemToMessage(item)
			if err != nil {
				return nil, err
			}
			out = append(out, msg)
		}
		return out, nil
	default:
		return nil, UnsupportedResponsesInputError("responses input must be a string or an array of messages")
	}
}

func responseInputItemToMessage(item any) (Message, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return Message{}, UnsupportedResponsesInputError("responses input items must be objects")
	}
	role, _ := obj["role"].(string)
	role = strings.TrimSpace(role)
	if role == "" {
		role = "user"
	}
	content, ok := obj["content"]
	if !ok {
		if text, ok := obj["text"].(string); ok {
			return Message{Role: role, Content: text}, nil
		}
		return Message{}, UnsupportedResponsesInputError("responses input message content is required")
	}
	converted, err := responseContentToChatContent(content)
	if err != nil {
		return Message{}, err
	}
	return Message{Role: role, Content: converted}, nil
}

func responseContentToChatContent(content any) (any, error) {
	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		parts := make([]any, 0, len(value))
		for _, rawPart := range value {
			part, ok := rawPart.(map[string]any)
			if !ok {
				return nil, UnsupportedResponsesInputError("responses content parts must be objects")
			}
			partType, _ := part["type"].(string)
			switch partType {
			case "input_text", "output_text", ContentTypeText:
				text, _ := part["text"].(string)
				parts = append(parts, map[string]any{"type": ContentTypeText, "text": text})
			case "input_image", "image_url", "input_file", "file":
				return nil, UnsupportedResponsesInputError("responses image and file input is not supported; use /v1/chat/completions")
			default:
				return nil, UnsupportedResponsesInputError(fmt.Sprintf("responses content type %q is not supported", partType))
			}
		}
		return parts, nil
	default:
		return nil, UnsupportedResponsesInputError("responses message content must be a string or content array")
	}
}
```

- [ ] **Step 4: Run request conversion tests to verify GREEN**

Run:

```powershell
go test ./relay/model -run "TestResponsesRequestToChatRequest" -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing non-stream response conversion test**

Add to `relay/model/responses_test.go`:

```go
func TestChatCompletionToResponsesMapsAssistantTextAndUsage(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-test",
		"object":"chat.completion",
		"created":1710000000,
		"model":"llama-free",
		"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if resp.ID == "" || resp.Object != "response" || resp.Status != "completed" {
		t.Fatalf("unexpected response envelope: %#v", resp)
	}
	if resp.Model != "llama-free" {
		t.Fatalf("expected upstream model, got %q", resp.Model)
	}
	if len(resp.Output) != 1 || resp.Output[0].Role != "assistant" {
		t.Fatalf("unexpected output: %#v", resp.Output)
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Text != "pong" {
		t.Fatalf("unexpected content: %#v", resp.Output[0].Content)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 || resp.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}
```

- [ ] **Step 6: Run non-stream response test to verify RED**

Run:

```powershell
go test ./relay/model -run TestChatCompletionToResponsesMapsAssistantTextAndUsage -count=1
```

Expected: FAIL because response structs and `ChatCompletionToResponses` do not exist.

- [ ] **Step 7: Implement non-stream response conversion**

Append to `relay/model/responses.go`:

```go
type ResponsesObject struct {
	ID        string                    `json:"id"`
	Object    string                    `json:"object"`
	CreatedAt int64                     `json:"created_at"`
	Status    string                    `json:"status"`
	Model     string                    `json:"model,omitempty"`
	Output    []ResponsesOutputItem     `json:"output"`
	Usage     *ResponsesUsage           `json:"usage,omitempty"`
	Error     *ResponsesErrorObject      `json:"error,omitempty"`
}

type ResponsesOutputItem struct {
	ID      string                   `json:"id"`
	Type    string                   `json:"type"`
	Role    string                   `json:"role,omitempty"`
	Content []ResponsesOutputContent `json:"content,omitempty"`
}

type ResponsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type ResponsesErrorObject struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

func ChatCompletionToResponses(body []byte, fallbackModel string) (*ResponsesObject, int, error) {
	var errPayload struct {
		Error Error `json:"error"`
	}
	if err := json.Unmarshal(body, &errPayload); err == nil && errPayload.Error.Message != "" {
		return &ResponsesObject{
			ID:        responsesID(),
			Object:    "response",
			CreatedAt: time.Now().Unix(),
			Status:    "failed",
			Model:     fallbackModel,
			Error: &ResponsesErrorObject{
				Message: errPayload.Error.Message,
				Type:    errPayload.Error.Type,
				Code:    errPayload.Error.Code,
			},
		}, 500, nil
	}

	var chat struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		return nil, 0, err
	}
	id := chat.ID
	if id == "" {
		id = responsesID()
	}
	created := chat.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	modelName := chat.Model
	if modelName == "" {
		modelName = fallbackModel
	}
	output := make([]ResponsesOutputItem, 0, len(chat.Choices))
	for index, choice := range chat.Choices {
		text := choice.Message.StringContent()
		output = append(output, ResponsesOutputItem{
			ID:   fmt.Sprintf("msg_%d", index),
			Type: "message",
			Role: "assistant",
			Content: []ResponsesOutputContent{{
				Type: "output_text",
				Text: text,
			}},
		})
	}
	return &ResponsesObject{
		ID:        id,
		Object:    "response",
		CreatedAt: created,
		Status:    "completed",
		Model:     modelName,
		Output:    output,
		Usage: &ResponsesUsage{
			InputTokens:  chat.Usage.PromptTokens,
			OutputTokens: chat.Usage.CompletionTokens,
			TotalTokens:  chat.Usage.TotalTokens,
		},
	}, 200, nil
}

func responsesID() string {
	return fmt.Sprintf("resp_%d", time.Now().UnixNano())
}
```

- [ ] **Step 8: Run relay model tests**

Run:

```powershell
go test ./relay/model -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

Run:

```powershell
git add relay/model/responses.go relay/model/responses_test.go
git commit -m "feat: add responses translation model"
```

Expected: commit succeeds.

---

## Task 2: Responses Route Shim and Non-Stream Relay Integration

**Files:**
- Modify: `relay/relaymode/define.go`
- Modify: `relay/relaymode/helper.go`
- Create: `relay/relaymode/helper_test.go`
- Create: `controller/responses.go`
- Create: `controller/responses_test.go`
- Modify: `middleware/utils.go`
- Modify: `router/relay.go`
- Create: `router/relay_test.go`

**Interfaces:**
- Consumes: `ResponsesRequest.ToChatRequest()`.
- Consumes: `ChatCompletionToResponses(body []byte, fallbackModel string)`.
- Produces: `controller.RelayResponses(c *gin.Context)`.
- Produces: `POST /v1/responses` using the existing token auth, distributor, fallback, billing, and retry path.

- [ ] **Step 1: Write failing relaymode and router tests**

Create `relay/relaymode/helper_test.go`:

```go
package relaymode

import "testing"

func TestGetByPathResponses(t *testing.T) {
	if got := GetByPath("/v1/responses"); got != Responses {
		t.Fatalf("GetByPath(/v1/responses) = %d, want %d", got, Responses)
	}
}
```

Create `router/relay_test.go`:

```go
package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetRelayRouterRegistersResponsesRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == "POST" && route.Path == "/v1/responses" {
			return
		}
	}
	t.Fatalf("POST /v1/responses route was not registered")
}
```

- [ ] **Step 2: Run route tests to verify RED**

Run:

```powershell
go test ./relay/relaymode ./router -run "TestGetByPathResponses|TestSetRelayRouterRegistersResponsesRoute" -count=1
```

Expected: FAIL because `Responses` and the route do not exist.

- [ ] **Step 3: Add relay mode and route registration**

Modify `relay/relaymode/define.go`:

```go
const (
	Unknown = iota
	ChatCompletions
	Completions
	Embeddings
	Moderations
	ImagesGenerations
	Edits
	AudioSpeech
	AudioTranscription
	AudioTranslation
	Responses
	Proxy
)
```

Modify `relay/relaymode/helper.go`:

```go
if strings.HasPrefix(path, "/v1/chat/completions") {
	relayMode = ChatCompletions
} else if strings.HasPrefix(path, "/v1/responses") {
	relayMode = Responses
} else if strings.HasPrefix(path, "/v1/messages") {
	relayMode = ChatCompletions
}
```

Modify `router/relay.go` inside the `/v1` group:

```go
relayV1Router.POST("/responses", controller.RelayResponses)
```

Modify `middleware/utils.go`:

```go
if strings.HasPrefix(c.Request.URL.Path, "/v1/responses") {
	return true
}
```

- [ ] **Step 4: Run route tests to verify GREEN**

Run:

```powershell
go test ./relay/relaymode ./router -run "TestGetByPathResponses|TestSetRelayRouterRegistersResponsesRoute" -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing controller shim tests**

Create `controller/responses_test.go`:

```go
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponsesCaptureWriterCapturesStatusHeadersAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	capture := newResponsesCaptureWriter(rec)

	capture.Header().Set("X-Test", "yes")
	capture.WriteHeader(http.StatusCreated)
	if _, err := capture.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	if capture.Status() != http.StatusCreated {
		t.Fatalf("expected captured status 201, got %d", capture.Status())
	}
	if capture.BodyString() != `{"ok":true}` {
		t.Fatalf("unexpected captured body: %q", capture.BodyString())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("capture writer must not write to real recorder before conversion")
	}
}

func TestRewriteResponsesContextForChatRelay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	restore := rewriteResponsesContextForChatRelay(c, []byte(`{"model":"cct/free","messages":[{"role":"user","content":"ping"}]}`), "cct/free")
	defer restore()

	if c.Request.URL.Path != "/v1/chat/completions" {
		t.Fatalf("expected chat completions path, got %s", c.Request.URL.Path)
	}
	if got := c.GetString("request_model"); got != "cct/free" {
		t.Fatalf("expected request model cct/free, got %q", got)
	}
	restore()
	if c.Request.URL.Path != "/v1/responses" {
		t.Fatalf("expected path restored, got %s", c.Request.URL.Path)
	}
}
```

- [ ] **Step 6: Run controller tests to verify RED**

Run:

```powershell
go test ./controller -run "TestResponsesCaptureWriter|TestRewriteResponsesContextForChatRelay" -count=1
```

Expected: FAIL because the capture and rewrite helpers do not exist.

- [ ] **Step 7: Implement non-stream controller shim**

Create `controller/responses.go`:

```go
package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/ctxkey"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

type responsesCaptureWriter struct {
	gin.ResponseWriter
	body       bytes.Buffer
	statusCode int
	wroteCode  bool
}

func newResponsesCaptureWriter(real gin.ResponseWriter) *responsesCaptureWriter {
	return &responsesCaptureWriter{ResponseWriter: real, statusCode: http.StatusOK}
}

func (w *responsesCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteCode = true
}

func (w *responsesCaptureWriter) Write(data []byte) (int, error) {
	if !w.wroteCode {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(data)
}

func (w *responsesCaptureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responsesCaptureWriter) Flush() {}

func (w *responsesCaptureWriter) Status() int {
	return w.statusCode
}

func (w *responsesCaptureWriter) BodyBytes() []byte {
	return w.body.Bytes()
}

func (w *responsesCaptureWriter) BodyString() string {
	return w.body.String()
}

func rewriteResponsesContextForChatRelay(c *gin.Context, body []byte, modelName string) func() {
	oldPath := c.Request.URL.Path
	oldRawPath := c.Request.URL.RawPath
	oldRequestURI := c.Request.RequestURI
	oldBody := c.Request.Body
	oldCachedBody, hadCachedBody := c.Get(ctxkey.KeyRequestBody)
	oldRequestModel, hadRequestModel := c.Get(ctxkey.RequestModel)

	c.Request.URL.Path = "/v1/chat/completions"
	c.Request.URL.RawPath = ""
	c.Request.RequestURI = "/v1/chat/completions"
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Set(ctxkey.KeyRequestBody, body)
	c.Set(ctxkey.RequestModel, modelName)

	return func() {
		c.Request.URL.Path = oldPath
		c.Request.URL.RawPath = oldRawPath
		c.Request.RequestURI = oldRequestURI
		c.Request.Body = oldBody
		if hadCachedBody {
			c.Set(ctxkey.KeyRequestBody, oldCachedBody)
		}
		if hadRequestModel {
			c.Set(ctxkey.RequestModel, oldRequestModel)
		}
	}
}

func RelayResponses(c *gin.Context) {
	var req relaymodel.ResponsesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	chatReq, err := req.ToChatRequest()
	if err != nil {
		var unsupported *relaymodel.ResponsesUnsupportedInputError
		status := http.StatusBadRequest
		if errors.As(err, &unsupported) {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "one_api_error"}})
		return
	}

	restoreContext := rewriteResponsesContextForChatRelay(c, chatBody, chatReq.Model)
	defer restoreContext()

	realWriter := c.Writer
	capture := newResponsesCaptureWriter(realWriter)
	c.Writer = capture
	Relay(c)
	c.Writer = realWriter

	if chatReq.Stream {
		writeResponsesStream(c, capture.BodyBytes(), chatReq.Model, capture.Status())
		return
	}
	resp, _, err := relaymodel.ChatCompletionToResponses(capture.BodyBytes(), chatReq.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "one_api_error"}})
		return
	}
	c.JSON(capture.Status(), resp)
}
```

In the same file, add a temporary stream passthrough function so this task compiles before Task 3:

```go
func writeResponsesStream(c *gin.Context, raw []byte, modelName string, status int) {
	c.Data(status, "text/event-stream", raw)
}
```

- [ ] **Step 8: Run controller and route tests to verify GREEN**

Run:

```powershell
go test ./controller ./router ./relay/relaymode -run "TestResponses|TestSetRelayRouterRegistersResponsesRoute|TestGetByPathResponses" -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

Run:

```powershell
git add relay/relaymode/define.go relay/relaymode/helper.go relay/relaymode/helper_test.go controller/responses.go controller/responses_test.go middleware/utils.go router/relay.go router/relay_test.go
git commit -m "feat: add responses relay shim"
```

Expected: commit succeeds.

---

## Task 3: Responses Streaming Event Conversion

**Files:**
- Modify: `relay/model/responses.go`
- Modify: `relay/model/responses_test.go`
- Modify: `controller/responses.go`
- Modify: `controller/responses_test.go`

**Interfaces:**
- Consumes: captured OpenAI chat-completions SSE from existing relay.
- Produces: `func ChatCompletionStreamToResponsesEvents(raw []byte, fallbackModel string) ([]ResponsesSSEEvent, error)`.
- Produces: `func WriteResponsesSSE(w io.Writer, events []ResponsesSSEEvent) error`.
- Produces: Responses event types `response.created`, `response.output_text.delta`, `response.function_call_arguments.delta`, `response.completed`, and `response.failed`.

- [ ] **Step 1: Write failing streaming conversion tests**

Append to `relay/model/responses_test.go`:

```go
func TestChatCompletionStreamToResponsesEventsMapsTextDeltaAndCompletion(t *testing.T) {
	raw := []byte("data: {\"id\":\"chatcmpl-1\",\"created\":1710000000,\"model\":\"llama-free\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"hel\"}}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"created\":1710000000,\"model\":\"llama-free\",\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n")

	events, err := ChatCompletionStreamToResponsesEvents(raw, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionStreamToResponsesEvents returned error: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected created, two deltas, completed; got %#v", events)
	}
	if events[0].Event != "response.created" {
		t.Fatalf("expected response.created, got %#v", events[0])
	}
	if events[1].Event != "response.output_text.delta" || events[1].Data["delta"] != "hel" {
		t.Fatalf("unexpected first delta: %#v", events[1])
	}
	if events[2].Event != "response.output_text.delta" || events[2].Data["delta"] != "lo" {
		t.Fatalf("unexpected second delta: %#v", events[2])
	}
	if events[3].Event != "response.completed" {
		t.Fatalf("expected response.completed, got %#v", events[3])
	}
}

func TestChatCompletionStreamToResponsesEventsMapsToolCallDelta(t *testing.T) {
	raw := []byte("data: {\"id\":\"chatcmpl-tool\",\"model\":\"llama-free\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{\\\"q\\\"\"}}]}}]}\n\n")

	events, err := ChatCompletionStreamToResponsesEvents(raw, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionStreamToResponsesEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Event == "response.function_call_arguments.delta" {
			found = true
			if event.Data["item_id"] != "call_1" || event.Data["delta"] != "{\"q\"" {
				t.Fatalf("unexpected tool delta: %#v", event)
			}
		}
	}
	if !found {
		t.Fatalf("expected function call argument delta in %#v", events)
	}
}
```

- [ ] **Step 2: Run streaming conversion tests to verify RED**

Run:

```powershell
go test ./relay/model -run "TestChatCompletionStreamToResponsesEvents" -count=1
```

Expected: FAIL because stream conversion types and functions do not exist.

- [ ] **Step 3: Implement streaming conversion helpers**

Append to `relay/model/responses.go`:

```go
type ResponsesSSEEvent struct {
	Event string         `json:"-"`
	Data  map[string]any `json:"-"`
}

func ChatCompletionStreamToResponsesEvents(raw []byte, fallbackModel string) ([]ResponsesSSEEvent, error) {
	lines := strings.Split(string(raw), "\n")
	events := make([]ResponsesSSEEvent, 0)
	responseID := responsesID()
	modelName := fallbackModel
	createdSent := false
	completedSent := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			if !completedSent {
				events = append(events, ResponsesSSEEvent{Event: "response.completed", Data: map[string]any{
					"id":     responseID,
					"status": "completed",
				}})
				completedSent = true
			}
			continue
		}
		var chunk struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Delta        Message `json:"delta"`
				FinishReason *string `json:"finish_reason,omitempty"`
			} `json:"choices"`
			Error *Error `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, err
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			events = append(events, ResponsesSSEEvent{Event: "response.failed", Data: map[string]any{
				"id": responseID,
				"error": map[string]any{
					"message": chunk.Error.Message,
					"type":    chunk.Error.Type,
					"code":    chunk.Error.Code,
				},
			}})
			continue
		}
		if chunk.ID != "" {
			responseID = chunk.ID
		}
		if chunk.Model != "" {
			modelName = chunk.Model
		}
		if !createdSent {
			createdAt := chunk.Created
			if createdAt == 0 {
				createdAt = time.Now().Unix()
			}
			events = append(events, ResponsesSSEEvent{Event: "response.created", Data: map[string]any{
				"id":         responseID,
				"object":     "response",
				"created_at": createdAt,
				"status":     "in_progress",
				"model":      modelName,
			}})
			createdSent = true
		}
		for _, choice := range chunk.Choices {
			if text := choice.Delta.StringContent(); text != "" {
				events = append(events, ResponsesSSEEvent{Event: "response.output_text.delta", Data: map[string]any{
					"response_id":   responseID,
					"output_index":  0,
					"content_index": 0,
					"delta":         text,
				}})
			}
			for _, toolCall := range choice.Delta.ToolCalls {
				if arg := fmt.Sprint(toolCall.Function.Arguments); arg != "" && arg != "<nil>" {
					events = append(events, ResponsesSSEEvent{Event: "response.function_call_arguments.delta", Data: map[string]any{
						"response_id": responseID,
						"item_id":     toolCall.Id,
						"output_index": 0,
						"delta":       arg,
					}})
				}
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" && !completedSent {
				events = append(events, ResponsesSSEEvent{Event: "response.completed", Data: map[string]any{
					"id":            responseID,
					"status":        "completed",
					"finish_reason": *choice.FinishReason,
				}})
				completedSent = true
			}
		}
	}
	if !createdSent {
		events = append(events, ResponsesSSEEvent{Event: "response.created", Data: map[string]any{
			"id":         responseID,
			"object":     "response",
			"created_at": time.Now().Unix(),
			"status":     "in_progress",
			"model":      modelName,
		}})
	}
	if !completedSent {
		events = append(events, ResponsesSSEEvent{Event: "response.completed", Data: map[string]any{
			"id":     responseID,
			"status": "completed",
		}})
	}
	return events, nil
}
```

- [ ] **Step 4: Run streaming conversion tests to verify GREEN**

Run:

```powershell
go test ./relay/model -run "TestChatCompletionStreamToResponsesEvents" -count=1
```

Expected: PASS.

- [ ] **Step 5: Replace temporary stream passthrough**

In `controller/responses.go`, replace `writeResponsesStream` with:

```go
func writeResponsesStream(c *gin.Context, raw []byte, modelName string, status int) {
	events, err := relaymodel.ChatCompletionStreamToResponsesEvents(raw, modelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "one_api_error"}})
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(status)
	for _, event := range events {
		payload, err := json.Marshal(event.Data)
		if err != nil {
			continue
		}
		_, _ = c.Writer.Write([]byte("event: " + event.Event + "\n"))
		_, _ = c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
		c.Writer.Flush()
	}
}
```

- [ ] **Step 6: Run package tests**

Run:

```powershell
go test ./relay/model ./controller ./router ./relay/relaymode -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```powershell
git add relay/model/responses.go relay/model/responses_test.go controller/responses.go controller/responses_test.go
git commit -m "feat: translate responses streaming events"
```

Expected: commit succeeds.

---

## Task 4: Free Provider Save Validation and Explicit Key Clearing

**Files:**
- Modify: `router/fallback_gateway_types.go`
- Modify: `router/fallback_gateway_projection.go`
- Modify: `router/fallback_gateway.go`
- Modify: `router/fallback_gateway_test.go`

**Interfaces:**
- Consumes: `fallback.ValidateFreeProviderName`.
- Consumes: `fallback.BuiltinFreeProviders`.
- Produces: `clear_keys: true` input behavior.
- Produces: `validateGatewayFreeProviders(current map[string]fallback.FreeProviderConfig, payload map[string]gatewayV2FreeProviderInput) error`.
- Produces: `mergeGatewayFreeProviderInput(existing fallback.FreeProviderConfig, input gatewayV2FreeProviderInput) fallback.FreeProviderConfig` with clear-key support.

- [ ] **Step 1: Write failing gateway validation tests**

Append to `router/fallback_gateway_test.go`:

```go
func TestGatewayUpdateConfig_UnknownFreeProviderRejected(t *testing.T) {
	setupGatewayConfigReadOnly(t, baseValidConfigJSON)
	payload := `{
		"enabled": true,
		"virtual_models": {"test/auto": {"enabled": true, "strategy": "quality_first", "pools": ["high"]}},
		"deployments": {"dep-1": {"enabled": true, "channel_id": 1, "real_model": "gpt-4", "pool": "high"}},
		"free_providers": {"not-real": {"enabled": true}}
	}`

	w := callGatewayPUT(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d\nbody: %s", w.Code, w.Body.String())
	}
	if !searchString(w.Body.String(), "unknown free provider") {
		t.Fatalf("expected unknown provider message, got %s", w.Body.String())
	}
}

func TestGatewayUpdateConfig_RequiresKeyEnabledWithoutKeysRejected(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
	defer cleanup()
	payload := `{
		"enabled": true,
		"virtual_models": {"test/auto": {"enabled": true, "strategy": "quality_first", "pools": ["high"]}},
		"deployments": {"dep-1": {"enabled": true, "channel_id": 1, "real_model": "gpt-4", "pool": "high"}},
		"free_providers": {"groq": {"enabled": true}}
	}`

	w := callGatewayPUT(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d\nbody: %s", w.Code, w.Body.String())
	}
	if !searchString(w.Body.String(), "requires at least one key") {
		t.Fatalf("expected requires-key message, got %s", w.Body.String())
	}
}

func TestGatewayUpdateConfig_KeylessProviderCanEnableWithoutKeys(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
	defer cleanup()
	payload := `{
		"enabled": true,
		"virtual_models": {"test/auto": {"enabled": true, "strategy": "quality_first", "pools": ["high"]}},
		"deployments": {"dep-1": {"enabled": true, "channel_id": 1, "real_model": "gpt-4", "pool": "high"}},
		"free_providers": {"pollinations": {"enabled": true}}
	}`

	w := callGatewayPUT(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	fp := fallback.GetConfig().FreeProviders["pollinations"]
	if !fp.Enabled || len(fp.Keys) != 0 {
		t.Fatalf("expected enabled keyless provider with no keys, got %+v", fp)
	}
}

func TestGatewayUpdateConfig_ClearKeysDeletesStoredKeys(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigWithFreeProviderJSON)
	defer cleanup()
	payload := `{
		"enabled": true,
		"virtual_models": {"test/auto": {"enabled": true, "strategy": "quality_first", "pools": ["high"]}},
		"deployments": {"dep-1": {"enabled": true, "channel_id": 1, "real_model": "gpt-4", "pool": "high"}},
		"free_providers": {"groq": {"enabled": false, "clear_keys": true}}
	}`

	w := callGatewayPUT(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	if got := fallback.GetConfig().FreeProviders["groq"].Keys; len(got) != 0 {
		t.Fatalf("expected stored keys cleared, got %v", got)
	}
}

func TestGatewayUpdateConfig_ClearKeysWithReplacementKeysRejected(t *testing.T) {
	setupGatewayConfigReadOnly(t, baseValidConfigWithFreeProviderJSON)
	payload := `{
		"enabled": true,
		"virtual_models": {"test/auto": {"enabled": true, "strategy": "quality_first", "pools": ["high"]}},
		"deployments": {"dep-1": {"enabled": true, "channel_id": 1, "real_model": "gpt-4", "pool": "high"}},
		"free_providers": {"groq": {"enabled": false, "clear_keys": true, "keys": ["gsk_new"]}}
	}`

	w := callGatewayPUT(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d\nbody: %s", w.Code, w.Body.String())
	}
	if !searchString(w.Body.String(), "clear_keys cannot be combined with keys") {
		t.Fatalf("expected clear conflict message, got %s", w.Body.String())
	}
}
```

- [ ] **Step 2: Run validation tests to verify RED**

Run:

```powershell
go test ./router -run "TestGatewayUpdateConfig_(UnknownFreeProviderRejected|RequiresKeyEnabledWithoutKeysRejected|KeylessProviderCanEnableWithoutKeys|ClearKeysDeletesStoredKeys|ClearKeysWithReplacementKeysRejected)" -count=1
```

Expected: FAIL because `clear_keys` and the new validation are not implemented.

- [ ] **Step 3: Add clear-key field**

Modify `router/fallback_gateway_types.go`:

```go
type gatewayV2FreeProviderInput struct {
	Enabled        bool                     `json:"enabled"`
	Keys           []string                 `json:"keys,omitempty"`
	ClearKeys      bool                     `json:"clear_keys,omitempty"`
	Models         []string                 `json:"models,omitempty"`
	LimitsOverride *gatewayV2LimitsOverride `json:"limits_override,omitempty"`
}
```

- [ ] **Step 4: Implement sanitized keys, validation, and merge**

Modify `router/fallback_gateway_projection.go`:

```go
func sanitizedGatewayProviderKeys(keys []string) []string {
	freshKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" || strings.Contains(k, "*") {
			continue
		}
		freshKeys = append(freshKeys, k)
	}
	return freshKeys
}

func validateGatewayFreeProviders(current map[string]fallback.FreeProviderConfig, payload map[string]gatewayV2FreeProviderInput) error {
	for name, input := range payload {
		if err := fallback.ValidateFreeProviderName(name); err != nil {
			return err
		}
		replacementKeys := sanitizedGatewayProviderKeys(input.Keys)
		if input.ClearKeys && len(replacementKeys) > 0 {
			return fmt.Errorf("free_provider %q clear_keys cannot be combined with keys", name)
		}
		if input.LimitsOverride != nil {
			if err := fallback.ValidateFreeProviderLimits(toFreeProviderLimits(input.LimitsOverride)); err != nil {
				return fmt.Errorf("free_provider %q limits_override: %w", name, err)
			}
		}
		meta := fallback.BuiltinFreeProviders[name]
		existingKeys := 0
		if current != nil {
			existingKeys = len(current[name].Keys)
		}
		keyCountAfterSave := existingKeys
		if input.ClearKeys {
			keyCountAfterSave = 0
		} else if len(replacementKeys) > 0 {
			keyCountAfterSave = len(replacementKeys)
		}
		if input.Enabled && meta.RequiresKey && !meta.Keyless && keyCountAfterSave == 0 {
			return fmt.Errorf("free_provider %q requires at least one key before it can be enabled", name)
		}
	}
	return nil
}
```

Update `mergeGatewayFreeProviderInput` key block:

```go
keys := append([]string{}, existing.Keys...)
if input.ClearKeys {
	keys = []string{}
} else if freshKeys := sanitizedGatewayProviderKeys(input.Keys); len(freshKeys) > 0 {
	keys = freshKeys
}
```

Add `fmt` to the imports for `router/fallback_gateway_projection.go`.

- [ ] **Step 5: Call validation in both gateway save handlers**

In `router/fallback_gateway.go`, after loading `current := fallback.CloneConfig()` and before building `merged`, call:

```go
if err := validateGatewayFreeProviders(current.FreeProviders, payload.FreeProviders); err != nil {
	c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
	return
}
```

Use the same call in `updateManualConfig` after `current` is loaded. Remove the earlier duplicated limit-only validation loops after the new combined validation is in place.

- [ ] **Step 6: Run validation tests to verify GREEN**

Run:

```powershell
go test ./router -run "TestGatewayUpdateConfig_(UnknownFreeProviderRejected|RequiresKeyEnabledWithoutKeysRejected|KeylessProviderCanEnableWithoutKeys|ClearKeysDeletesStoredKeys|ClearKeysWithReplacementKeysRejected|MaskedKeyNotWrittenBack|EmptyKeyPreservesOld|NewKeyCanUpdate|NegativeLimitsOverrideRejected|ValidLimitsOverrideAccepted)" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```powershell
git add router/fallback_gateway_types.go router/fallback_gateway_projection.go router/fallback_gateway.go router/fallback_gateway_test.go
git commit -m "fix: validate free provider save semantics"
```

Expected: commit succeeds.

---

## Task 5: Free Provider Usage API

**Files:**
- Modify: `fallback/free_provider_ledger.go`
- Modify: `fallback/free_provider_ledger_test.go`
- Create: `router/fallback_usage.go`
- Create: `router/fallback_usage_test.go`
- Modify: `router/fallback.go`

**Interfaces:**
- Consumes: existing `FreeProviderUsageLedger`.
- Produces: `type FreeProviderUsageFilter struct`.
- Produces: `func ListFreeProviderUsage(filter FreeProviderUsageFilter) ([]FreeProviderUsageLedger, error)`.
- Produces: admin API `GET /api/fallback/free-pool/usage?provider=&key_hash=&model=&period=`.

- [ ] **Step 1: Write failing ledger list tests**

Append to `fallback/free_provider_ledger_test.go`:

```go
func TestListFreeProviderUsageFiltersByProviderAndPeriod(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := RecordFreeProviderUsage("free:groq-001122ff", "llama-free", UsageInfo{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}); err != nil {
		t.Fatalf("record groq: %v", err)
	}
	if err := RecordFreeProviderUsage("free:nvidia-deadbeef", "qwen-free", UsageInfo{PromptTokens: 4, CompletionTokens: 5, TotalTokens: 9}); err != nil {
		t.Fatalf("record nvidia: %v", err)
	}

	rows, err := ListFreeProviderUsage(FreeProviderUsageFilter{Provider: "groq", Period: todayString()})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Provider != "groq" || rows[0].KeyHash != "001122ff" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestListFreeProviderUsageDefaultsToToday(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := RecordFreeProviderUsage("free:groq-001122ff", "llama-free", UsageInfo{TotalTokens: 3}); err != nil {
		t.Fatalf("record: %v", err)
	}

	rows, err := ListFreeProviderUsage(FreeProviderUsageFilter{})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Period != todayString() {
		t.Fatalf("expected one current-period row, got %+v", rows)
	}
}
```

- [ ] **Step 2: Run ledger list tests to verify RED**

Run:

```powershell
go test ./fallback -run "TestListFreeProviderUsage" -count=1
```

Expected: FAIL because `ListFreeProviderUsage` does not exist.

- [ ] **Step 3: Implement ledger list query**

Append to `fallback/free_provider_ledger.go`:

```go
type FreeProviderUsageFilter struct {
	Provider  string
	KeyHash   string
	ModelName string
	Period    string
}

func ListFreeProviderUsage(filter FreeProviderUsageFilter) ([]FreeProviderUsageLedger, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if err := InitFreeProviderLedgerStore(); err != nil {
		return nil, err
	}
	period := strings.TrimSpace(filter.Period)
	if period == "" {
		period = todayString()
	}
	query := model.DB.Model(&FreeProviderUsageLedger{}).Where("period = ?", period)
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if keyHash := strings.TrimSpace(filter.KeyHash); keyHash != "" {
		query = query.Where("key_hash = ?", keyHash)
	}
	if modelName := strings.TrimSpace(filter.ModelName); modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	var rows []FreeProviderUsageLedger
	if err := query.Order("updated_at DESC").Limit(500).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
```

- [ ] **Step 4: Run ledger list tests to verify GREEN**

Run:

```powershell
go test ./fallback -run "TestListFreeProviderUsage|TestRecordFreeProviderUsage" -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing usage API tests**

Create `router/fallback_usage_test.go`:

```go
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRouterFreeProviderLedgerDB(t *testing.T) func() {
	t.Helper()
	originalDB := dbmodel.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	dbmodel.DB = db
	return func() {
		dbmodel.DB = originalDB
	}
}

func callFreePoolUsageGET(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	getFreePoolUsage(c)
	return w
}

func TestGetFreePoolUsageReturnsRows(t *testing.T) {
	cleanupDB := setupRouterFreeProviderLedgerDB(t)
	defer cleanupDB()
	if err := fallback.InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := fallback.RecordFreeProviderUsage("free:groq-001122ff", "llama-free", fallback.UsageInfo{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}); err != nil {
		t.Fatalf("record usage: %v", err)
	}

	w := callFreePoolUsageGET(t, "/api/fallback/free-pool/usage?provider=groq")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool                               `json:"success"`
		Data    []fallback.FreeProviderUsageLedger `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data[0].Provider != "groq" || resp.Data[0].KeyHash != "001122ff" {
		t.Fatalf("unexpected row: %+v", resp.Data[0])
	}
	if searchString(w.Body.String(), "gsk_") {
		t.Fatalf("usage API must not expose raw keys: %s", w.Body.String())
	}
}
```

- [ ] **Step 6: Run usage API tests to verify RED**

Run:

```powershell
go test ./router -run TestGetFreePoolUsageReturnsRows -count=1
```

Expected: FAIL because `getFreePoolUsage` does not exist.

- [ ] **Step 7: Implement usage API and route**

Create `router/fallback_usage.go`:

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
)

func getFreePoolUsage(c *gin.Context) {
	rows, err := fallback.ListFreeProviderUsage(fallback.FreeProviderUsageFilter{
		Provider:  c.Query("provider"),
		KeyHash:   c.Query("key_hash"),
		ModelName: c.Query("model"),
		Period:    c.Query("period"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}
```

Modify `router/fallback.go` inside `adminGroup` near the free-pool endpoints:

```go
adminGroup.GET("/free-pool/usage", getFreePoolUsage)
```

- [ ] **Step 8: Run usage tests to verify GREEN**

Run:

```powershell
go test ./fallback ./router -run "TestListFreeProviderUsage|TestGetFreePoolUsageReturnsRows" -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

Run:

```powershell
git add fallback/free_provider_ledger.go fallback/free_provider_ledger_test.go router/fallback_usage.go router/fallback_usage_test.go router/fallback.go
git commit -m "feat: expose free provider usage"
```

Expected: commit succeeds.

---

## Task 6: Free Pool UI for Key Clearing and Usage

**Files:**
- Modify: `web/default/src/components/fallback-gateway/gatewayConfigApi.js`
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.js`
- Modify: `web/default/src/components/fallback-gateway/freePoolUtils.test.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProvidersEditor.js`
- Modify: `web/default/src/components/fallback-gateway/FreeProviderRow.js`
- Modify: `web/default/src/components/fallback-gateway/FreeModelPool.js`

**Interfaces:**
- Consumes: `GET /api/fallback/free-pool/usage`.
- Consumes: gateway free-provider `clear_keys`.
- Produces: `getFreePoolUsage()`.
- Produces: `indexUsageRows(rows)`.
- Produces: explicit clear-key payload behavior.
- Produces: compact read-only usage table showing provider, key hash, model, period, tokens, requests, successes, and updated time.

- [ ] **Step 1: Write failing frontend utility tests**

Append to `web/default/src/components/fallback-gateway/freePoolUtils.test.js`:

```js
import {
  buildClearKeysProviderConfig,
  indexUsageRows,
} from './freePoolUtils';

it('indexes usage rows by provider and key hash', () => {
  const index = indexUsageRows([
    {
      provider: 'groq',
      key_hash: '001122ff',
      model_name: 'llama-free',
      total_tokens: 10,
      request_count: 2,
      success_count: 2,
    },
  ]);

  expect(index['groq:001122ff']).toEqual([
    expect.objectContaining({ model_name: 'llama-free', total_tokens: 10 }),
  ]);
});

it('builds explicit clear key payload without raw keys', () => {
  const next = buildClearKeysProviderConfig(
    { groq: { enabled: false, key_count: 1 } },
    'groq',
  );

  expect(next.groq.clear_keys).toBe(true);
  expect(next.groq.keys).toBeUndefined();
});
```

- [ ] **Step 2: Run frontend utility tests to verify RED**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false --runTestsByPath src/components/fallback-gateway/freePoolUtils.test.js
```

Expected: FAIL because the helper functions do not exist.

- [ ] **Step 3: Implement frontend helpers**

Append to `web/default/src/components/fallback-gateway/freePoolUtils.js`:

```js
export const indexUsageRows = (rows = []) => {
  const index = {};
  (Array.isArray(rows) ? rows : []).forEach((row) => {
    const provider = String(row.provider || '').trim().toLowerCase();
    const keyHash = String(row.key_hash || '').trim();
    if (!provider || !keyHash) return;
    const key = `${provider}:${keyHash}`;
    if (!index[key]) index[key] = [];
    index[key].push(row);
  });
  return index;
};

export const buildClearKeysProviderConfig = (freeProviders = {}, providerKey) => {
  const current = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const existing = current[providerKey] || {};
  const nextProvider = { ...existing, clear_keys: true };
  delete nextProvider.keys;
  return {
    ...current,
    [providerKey]: nextProvider,
  };
};
```

Modify `web/default/src/components/fallback-gateway/gatewayConfigApi.js`:

```js
export const getFreePoolUsage = (params = {}) =>
  API.get('/api/fallback/free-pool/usage', { params });
```

- [ ] **Step 4: Run frontend utility tests to verify GREEN**

Run:

```powershell
npm test -- --watchAll=false --runTestsByPath src/components/fallback-gateway/freePoolUtils.test.js
```

Expected: PASS.

- [ ] **Step 5: Add explicit clear-key UI and usage table**

Modify `FreeProvidersEditor.js` to import `buildClearKeysProviderConfig`, add:

```js
const clearKeys = (providerKey) => {
  onChange(buildClearKeysProviderConfig(providerConfig, providerKey));
};
```

Pass `onClearKeys={clearKeys}` and `usageRows={usageByProviderKey[provider.name] || []}` to `FreeProviderRow`. Update the component signature to accept `usageByProviderKey = {}`.

Modify `FreeProviderRow.js` component signature:

```js
const FreeProviderRow = ({
  provider,
  providerConfig,
  onUpdateProvider,
  onUpdateLimit,
  onUpdateKeys,
  onClearKeys,
  usageRows = [],
}) => {
```

Add a clear button under the key textarea:

```jsx
<Button
  type='button'
  basic
  size='mini'
  color='red'
  icon
  labelPosition='left'
  disabled={keyCount === 0 && stagedKeys.length === 0}
  onClick={() => onClearKeys(key)}
>
  <Icon name='trash' />
  Clear stored keys
</Button>
{providerConfig[key]?.clear_keys && (
  <Label basic color='red' size='mini'>keys will be cleared on save</Label>
)}
```

Add a compact usage summary in the status cell:

```jsx
{usageRows.length > 0 && (
  <div style={{ marginTop: 6 }}>
    <Label basic size='mini'>
      {usageRows.reduce((sum, row) => sum + Number(row.total_tokens || 0), 0).toLocaleString()} tokens
    </Label>
  </div>
)}
```

Modify `FreeModelPool.js` imports:

```js
import {
  cleanupDryRun,
  getFreePoolUsage,
  getGatewayConfig,
  getRuntimeStatus,
  reloadConfig,
  saveGatewayConfig,
  syncFreePool,
} from './gatewayConfigApi';
import { indexUsageRows } from './freePoolUtils';
```

Add state and load the usage API:

```js
const [usageRows, setUsageRows] = useState([]);
```

Inside `loadAll`:

```js
const [configRes, runtimeRes, usageRes] = await Promise.all([
  getGatewayConfig(),
  getRuntimeStatus(),
  getFreePoolUsage(),
]);
const usageData = usageRes.data || {};
if (usageData.success !== false) {
  setUsageRows(Array.isArray(usageData.data) ? usageData.data : []);
}
```

Add memoized usage index:

```js
const usageByProviderKey = useMemo(() => indexUsageRows(usageRows), [usageRows]);
```

Pass usage to editor:

```jsx
<FreeProvidersEditor
  freeProviders={config.free_providers || {}}
  freeProviderCatalog={config.free_provider_catalog || []}
  usageByProviderKey={usageByProviderKey}
  onChange={updateFreeProviders}
/>
```

Add a read-only usage section after the provider editor:

```jsx
<section className='fallback-virtual-panel'>
  <div className='fallback-virtual-header'>
    <div>
      <h3>Free provider usage</h3>
      <span>Read-only token and request totals by provider, key hash, model, and period.</span>
    </div>
  </div>
  <Table compact celled striped>
    <Table.Header>
      <Table.Row>
        <Table.HeaderCell>Provider</Table.HeaderCell>
        <Table.HeaderCell>Key hash</Table.HeaderCell>
        <Table.HeaderCell>Model</Table.HeaderCell>
        <Table.HeaderCell>Period</Table.HeaderCell>
        <Table.HeaderCell>Total tokens</Table.HeaderCell>
        <Table.HeaderCell>Requests</Table.HeaderCell>
        <Table.HeaderCell>Successes</Table.HeaderCell>
        <Table.HeaderCell>Updated</Table.HeaderCell>
      </Table.Row>
    </Table.Header>
    <Table.Body>
      {usageRows.length === 0 ? (
        <Table.Row>
          <Table.Cell colSpan='8' textAlign='center'>No usage rows for the selected period</Table.Cell>
        </Table.Row>
      ) : usageRows.map((row) => (
        <Table.Row key={`${row.provider}-${row.key_hash}-${row.model_name}-${row.period}`}>
          <Table.Cell>{PROVIDER_LABELS[row.provider] || row.provider}</Table.Cell>
          <Table.Cell><Label basic>{row.key_hash}</Label></Table.Cell>
          <Table.Cell>{row.model_name || '-'}</Table.Cell>
          <Table.Cell>{row.period || '-'}</Table.Cell>
          <Table.Cell>{formatNumber(row.total_tokens)}</Table.Cell>
          <Table.Cell>{formatNumber(row.request_count)}</Table.Cell>
          <Table.Cell>{formatNumber(row.success_count)}</Table.Cell>
          <Table.Cell>{row.updated_at ? new Date(row.updated_at).toLocaleString() : '-'}</Table.Cell>
        </Table.Row>
      ))}
    </Table.Body>
  </Table>
</section>
```

- [ ] **Step 6: Run frontend tests and build**

Run:

```powershell
npm test -- --watchAll=false --runTestsByPath src/components/fallback-gateway/freePoolUtils.test.js src/components/fallback-gateway/freeProviderDisplay.test.js
npm run build
```

Expected: tests PASS; build exits 0. Existing build warnings are acceptable only if they already existed before this task.

- [ ] **Step 7: Commit**

Run:

```powershell
git add web/default/src/components/fallback-gateway/gatewayConfigApi.js web/default/src/components/fallback-gateway/freePoolUtils.js web/default/src/components/fallback-gateway/freePoolUtils.test.js web/default/src/components/fallback-gateway/FreeProvidersEditor.js web/default/src/components/fallback-gateway/FreeProviderRow.js web/default/src/components/fallback-gateway/FreeModelPool.js
git commit -m "feat: show free provider usage in gateway UI"
```

Expected: commit succeeds.

---

## Task 7: Free Provider Health Probe Hardening

**Files:**
- Modify: `fallback/health.go`
- Create: `fallback/health_test.go`

**Interfaces:**
- Consumes: `FreeProviderNameFromDeploymentID`, `BuiltinFreeProviders`, and `common/freeproviderquirks`.
- Produces: `buildHealthProbeRequest(deploymentID string, channel *model.Channel, dep DeploymentConfig) (*http.Request, error)`.
- Produces: keyless providers without `Authorization`.
- Produces: provider default user-agent and max-token body quirks.
- Produces: no import from `relay/adaptor`.

- [ ] **Step 1: Write failing health helper tests**

Create `fallback/health_test.go`:

```go
package fallback

import (
	"io"
	"strings"
	"testing"

	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

func strPtr(s string) *string { return &s }

func TestBuildHealthProbeRequestKeylessOmitsAuthorization(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:      1,
		Type:    channeltype.OpenAICompatible,
		Key:     "",
		BaseURL: strPtr("https://text.pollinations.ai/openai"),
	}
	req, err := buildHealthProbeRequest("free:pollinations-"+SafeKeyHash(""), channel, DeploymentConfig{RealModel: "openai-fast"})
	if err != nil {
		t.Fatalf("buildHealthProbeRequest returned error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no Authorization header for keyless provider, got %q", got)
	}
	if req.URL.String() != "https://text.pollinations.ai/openai/v1/chat/completions" {
		t.Fatalf("unexpected URL: %s", req.URL.String())
	}
}

func TestBuildHealthProbeRequestAppliesUserAgentQuirk(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:      2,
		Type:    channeltype.OpenAICompatible,
		Key:     "routeway-key",
		BaseURL: strPtr("https://api.routeway.ai/v1"),
	}
	req, err := buildHealthProbeRequest("free:routeway-001122ff", channel, DeploymentConfig{RealModel: "auto"})
	if err != nil {
		t.Fatalf("buildHealthProbeRequest returned error: %v", err)
	}
	if got := req.Header.Get("User-Agent"); got != "cctapi-free-pool/1.0" {
		t.Fatalf("expected routeway user-agent quirk, got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer routeway-key" {
		t.Fatalf("expected auth header, got %q", got)
	}
}

func TestBuildHealthProbeRequestAppliesMaxOutputTokenQuirk(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:      3,
		Type:    channeltype.OpenAICompatible,
		Key:     "",
		BaseURL: strPtr("https://oai.aihorde.net/v1"),
	}
	req, err := buildHealthProbeRequest("free:aihorde-001122ff", channel, DeploymentConfig{RealModel: "horde"})
	if err != nil {
		t.Fatalf("buildHealthProbeRequest returned error: %v", err)
	}
	body, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(body), `"max_tokens":1`) {
		t.Fatalf("expected max_tokens=1 body, got %s", string(body))
	}
	if strings.Contains(string(body), `"stream":true`) {
		t.Fatalf("health body must not stream, got %s", string(body))
	}
}
```

- [ ] **Step 2: Run health helper tests to verify RED**

Run:

```powershell
go test ./fallback -run "TestBuildHealthProbeRequest" -count=1
```

Expected: FAIL because `buildHealthProbeRequest` does not exist and current ping always sets Authorization.

- [ ] **Step 3: Implement provider-aware probe helpers**

Modify `fallback/health.go`:

```go
func pingDeployment(deploymentID string, channel *dbmodel.Channel, dep DeploymentConfig, timeout time.Duration) (int, error) {
	req, err := buildHealthProbeRequest(deploymentID, channel, dep)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func buildHealthProbeRequest(deploymentID string, channel *dbmodel.Channel, dep DeploymentConfig) (*http.Request, error) {
	baseURL := buildChannelBaseURL(channel)
	if baseURL == "" {
		return nil, fmt.Errorf("channel %d has empty base url", channel.Id)
	}
	providerName, _ := FreeProviderNameFromDeploymentID(deploymentID)
	quirks := freeProviderQuirks(providerName)
	body := buildHealthProbeBody(dep, quirks)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/chat/completions", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(channel.Key) != "" {
		req.Header.Set("Authorization", "Bearer "+channel.Key)
	}
	if quirks != nil && quirks.DefaultUserAgent != "" {
		req.Header.Set("User-Agent", quirks.DefaultUserAgent)
	}
	return req, nil
}

func buildHealthProbeBody(dep DeploymentConfig, quirks *FreeProviderQuirks) string {
	maxTokens := 1
	if quirks != nil && quirks.MaxOutputTokens > 0 && quirks.MaxOutputTokens < maxTokens {
		maxTokens = quirks.MaxOutputTokens
	}
	return fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"ping"}],"max_tokens":%d,"stream":false}`, dep.RealModel, maxTokens)
}
```

Update `checkOneDeployment`:

```go
statusCode, err := pingDeployment(deploymentID, channel, dep, timeout)
```

Keep `buildChannelBaseURL` in `fallback` and do not import `relay/adaptor`.

- [ ] **Step 4: Run health tests to verify GREEN**

Run:

```powershell
go test ./fallback -run "TestBuildHealthProbeRequest|TestRecordFreeProviderUsage" -count=1
```

Expected: PASS.

- [ ] **Step 5: Run fallback package tests**

Run:

```powershell
go test ./fallback -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```powershell
git add fallback/health.go fallback/health_test.go
git commit -m "fix: harden free provider health probes"
```

Expected: commit succeeds.

---

## Task 8: Docs, Smoke Checks, and Full Verification

**Files:**
- Create: `docs/freellmapi-route-a.md`
- Modify: `scripts/fallback-smoke.ps1`
- Read: `docs/superpowers/specs/2026-07-04-freellmapi-route-a-design.md`
- Read: `docs/superpowers/plans/2026-07-04-freellmapi-route-a-implementation.md`

**Interfaces:**
- Consumes: all task outputs.
- Produces: operator documentation and smoke commands.
- Produces: verified branch ready for review and merge.

- [ ] **Step 1: Write documentation**

Create `docs/freellmapi-route-a.md`:

```markdown
# FreeLLMAPI Route A

Route A adds cctapi-native compatibility for the core FreeLLMAPI workflow while keeping the existing fallback gateway as the only execution path.

## Supported

- `POST /v1/responses` for text-only Responses API clients.
- Streaming Responses SSE translated from chat-completions SSE.
- Explicit free-provider key replacement and `clear_keys`.
- Read-only free-provider usage rows at `GET /api/fallback/free-pool/usage`.
- Provider-aware health probes for keyless providers and request quirks.

## Not Supported In This Slice

- Assistants, Threads, files, fine-tuning, and vector stores.
- Image or file input through `/v1/responses`.
- Signed remote catalog sync.
- Encrypted free-provider key storage.

## Key Save Semantics

- Omitting `keys` preserves stored keys.
- Sending non-empty `keys` replaces stored keys after trimming blank and masked values.
- Sending `clear_keys: true` deletes stored keys.
- Sending `clear_keys: true` with replacement keys is rejected.
- Gateway GET responses and usage APIs never return raw provider keys.

## Usage API

`GET /api/fallback/free-pool/usage` accepts optional `provider`, `key_hash`, `model`, and `period` query parameters. The response contains provider, key hash, model name, period, prompt tokens, completion tokens, total tokens, request count, success count, and timestamps.
```

- [ ] **Step 2: Add parser-safe smoke examples**

Append to `scripts/fallback-smoke.ps1` a non-executing help block or existing smoke section matching the current script style:

```powershell
# Route A smoke examples:
# Invoke-RestMethod -Method Post "$BaseUrl/v1/responses" -Headers @{ Authorization = "Bearer $Token" } -ContentType "application/json" -Body (@{
#   model = "cct/free"
#   input = "ping"
# } | ConvertTo-Json)
#
# Invoke-RestMethod -Method Get "$BaseUrl/api/fallback/free-pool/usage?provider=groq" -Headers @{ Authorization = "Bearer $AdminAccessToken" }
```

- [ ] **Step 3: Run backend focused tests**

Run:

```powershell
$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
$env:CGO_ENABLED='1'
go test ./relay/model ./relay/relaymode ./controller ./router ./fallback -count=1
```

Expected: PASS.

- [ ] **Step 4: Run backend full tests and build**

Run:

```powershell
go test ./... -count=1
go build -o one-api.exe
```

Expected: PASS and build exits 0.

- [ ] **Step 5: Run frontend tests and build**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false
npm run build
```

Expected: tests PASS and build exits 0. Record existing warnings without treating old warnings as task failures.

- [ ] **Step 6: Run smoke script parser check**

Run:

```powershell
Set-Location D:\ct\project
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\fallback-smoke.ps1 -WhatIf
```

Expected: script parses and exits without syntax errors. If `-WhatIf` is not supported by the script, run:

```powershell
powershell -NoProfile -Command "$null = [scriptblock]::Create((Get-Content -Raw '.\scripts\fallback-smoke.ps1')); 'parser ok'"
```

Expected: prints `parser ok`.

- [ ] **Step 7: Self-review against the spec**

Run:

```powershell
rg -n "responses|clear_keys|free-pool/usage|health|Authorization|input_image|raw key|keys" docs\superpowers\specs\2026-07-04-freellmapi-route-a-design.md docs\freellmapi-route-a.md relay controller router fallback web\default\src\components\fallback-gateway scripts\fallback-smoke.ps1
git status --short
```

Expected: every Route A spec goal has a matching code, test, or doc location; `git status` only shows intended files before the final commit.

- [ ] **Step 8: Commit docs and smoke updates**

Run:

```powershell
git add docs/freellmapi-route-a.md scripts/fallback-smoke.ps1
git commit -m "docs: document freellmapi route a"
```

Expected: commit succeeds.

- [ ] **Step 9: Push and request review**

Run:

```powershell
git status --short --branch
git push origin cleanup/structure-boundaries
```

Expected: branch is clean before push; push updates `origin/cleanup/structure-boundaries`.

---

## Self-Review Checklist

- Spec coverage:
  - `/v1/responses` route and text-only request conversion: Tasks 1, 2, 3.
  - Preserve existing fallback execution path: Task 2 calls the existing `Relay(c)` after body/path rewrite.
  - Unsupported image input returns 422: Task 1 and Task 2.
  - Free-provider name validation and requires-key validation: Task 4.
  - Key preservation, replacement, and deletion: Task 4 and Task 6.
  - Raw keys never returned: Task 4, Task 5, Task 6.
  - Usage API and UI: Task 5 and Task 6.
  - Health probe keyless and quirks: Task 7.
  - Docs, smoke checks, and full verification: Task 8.
- Type consistency:
  - `ResponsesRequest.ToChatRequest` returns `*GeneralOpenAIRequest`.
  - `ResponsesUnsupportedInputError` is used by the controller to return HTTP 422.
  - `clear_keys` is represented by `gatewayV2FreeProviderInput.ClearKeys`.
  - Usage filter query parameter `model` maps to `FreeProviderUsageFilter.ModelName`.
- Boundary consistency:
  - `relay/model` has pure translation only.
  - top-level `controller` owns the route shim because it can call existing `Relay`.
  - `router` owns admin gateway validation and usage HTTP handlers.
  - `fallback` owns provider metadata, usage ledger queries, and health probes.
  - `web/default` owns the active UI.
