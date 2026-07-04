package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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

func TestResponsesRequestToChatRequestRejectsMissingInputWithRequiredFieldError(t *testing.T) {
	req := ResponsesRequest{Model: "cct/free"}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "field input is required" {
		t.Fatalf("expected field input is required, got %v", err)
	}
	var unsupported *ResponsesUnsupportedInputError
	if errors.As(err, &unsupported) {
		t.Fatalf("expected non-unsupported error, got %T %v", err, err)
	}
}

func TestResponsesRequestToChatRequestPreservesMessageMetadata(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[
			{
				"role":"assistant",
				"name":"helper",
				"reasoning_content":"thinking",
				"tool_call_id":"call_1",
				"tool_calls":[
					{
						"id":"call_1",
						"type":"function",
						"function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}
					}
				],
				"content":[{"type":"output_text","text":"hi"}]
			}
		]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	chat, err := req.ToChatRequest()
	if err != nil {
		t.Fatalf("ToChatRequest returned error: %v", err)
	}
	if len(chat.Messages) != 1 {
		t.Fatalf("expected one message, got %#v", chat.Messages)
	}
	msg := chat.Messages[0]
	if msg.Name == nil || *msg.Name != "helper" {
		t.Fatalf("expected name helper, got %#v", msg.Name)
	}
	if msg.ReasoningContent != "thinking" {
		t.Fatalf("expected reasoning_content thinking, got %#v", msg.ReasoningContent)
	}
	if msg.ToolCallId != "call_1" {
		t.Fatalf("expected tool_call_id call_1, got %#v", msg.ToolCallId)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %#v", msg.ToolCalls)
	}
	if msg.ToolCalls[0].Id != "call_1" || msg.ToolCalls[0].Type != "function" {
		t.Fatalf("unexpected tool call: %#v", msg.ToolCalls[0])
	}
	if msg.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("expected tool call name lookup, got %#v", msg.ToolCalls[0].Function.Name)
	}
	if !reflect.DeepEqual(msg.ToolCalls[0].Function.Arguments, `{"q":"x"}`) {
		t.Fatalf("expected tool call arguments preserved, got %#v", msg.ToolCalls[0].Function.Arguments)
	}
}

func TestResponsesRequestToChatRequestAllowsToolCallOnlyAssistantMessage(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[
			{
				"role":"assistant",
				"tool_calls":[
					{
						"id":"call_1",
						"type":"function",
						"function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}
					}
				]
			}
		]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	chat, err := req.ToChatRequest()
	if err != nil {
		t.Fatalf("ToChatRequest returned error: %v", err)
	}
	if len(chat.Messages) != 1 {
		t.Fatalf("expected one message, got %#v", chat.Messages)
	}
	msg := chat.Messages[0]
	if msg.Role != "assistant" {
		t.Fatalf("expected assistant role, got %#v", msg.Role)
	}
	if msg.Content != nil {
		t.Fatalf("expected empty content, got %#v", msg.Content)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Id != "call_1" {
		t.Fatalf("expected preserved tool call, got %#v", msg.ToolCalls)
	}
}

func TestResponsesRequestToChatRequestRejectsUserToolCallsWithoutContent(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[
			{
				"role":"user",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]
			}
		]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResponsesRequestToChatRequestRejectsMissingRoleToolCallsWithoutContent(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[
			{
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]
			}
		]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResponsesRequestToChatRequestRejectsEmptyInputWithInstructions(t *testing.T) {
	req := ResponsesRequest{
		Model:        "cct/free",
		Instructions: "x",
		Input:        []any{},
	}

	chat, err := req.ToChatRequest()
	if err == nil {
		t.Fatalf("expected error, got chat %#v", chat)
	}
}

func TestResponsesRequestToChatRequestRejectsBlankStringContentWithoutToolCalls(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[{"role":"assistant","content":""}]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResponsesRequestToChatRequestRejectsEmptyContentArrayWithoutToolCalls(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[{"role":"assistant","content":[]}]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResponsesRequestToChatRequestRejectsBlankTextInContentArrayWithoutToolCalls(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[{"role":"assistant","content":[{"type":"input_text","text":""}]}]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResponsesRequestToChatRequestRejectsWhitespaceTextInContentArrayWithoutToolCalls(t *testing.T) {
	var req ResponsesRequest
	raw := []byte(`{
		"model":"cct/free",
		"input":[{"role":"assistant","content":[{"type":"input_text","text":"   "}]}]
	}`)
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, err := req.ToChatRequest()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

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

func TestChatCompletionToResponsesLeavesUsageNilWhenOmitted(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-test",
		"object":"chat.completion",
		"created":1710000000,
		"model":"llama-free",
		"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if resp.Usage != nil {
		t.Fatalf("expected nil usage, got %#v", resp.Usage)
	}
}

func TestChatCompletionToResponsesUsesTopLevelErrorStatusCode(t *testing.T) {
	body := []byte(`{
		"status_code":429,
		"error":{"message":"rate limited","type":"rate_limit_error","code":"rate_limit"}
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 429 {
		t.Fatalf("expected status 429, got %d", status)
	}
	if resp == nil || resp.Error == nil || resp.Error.Message != "rate limited" {
		t.Fatalf("unexpected error response: %#v", resp)
	}
}

func TestChatCompletionToResponsesUsesOnlyFirstChoiceForAssistantMessage(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-multi",
		"object":"chat.completion",
		"created":1710000000,
		"model":"llama-free",
		"choices":[
			{"index":0,"message":{"role":"assistant","content":"first"},"finish_reason":"stop"},
			{"index":1,"message":{"role":"assistant","content":"second"},"finish_reason":"stop"}
		],
		"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected exactly one output item, got %#v", resp.Output)
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Role != "assistant" {
		t.Fatalf("unexpected assistant message output: %#v", resp.Output[0])
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Text != "first" {
		t.Fatalf("expected first choice text, got %#v", resp.Output[0].Content)
	}
}

func TestChatCompletionToResponsesEmitsAssistantMessageForEmptyChoices(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-empty",
		"object":"chat.completion",
		"created":1710000000,
		"model":"llama-free",
		"choices":[],
		"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected exactly one assistant output item, got %#v", resp.Output)
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Role != "assistant" {
		t.Fatalf("unexpected assistant output: %#v", resp.Output[0])
	}
	if len(resp.Output[0].Content) != 0 {
		t.Fatalf("expected empty assistant content, got %#v", resp.Output[0].Content)
	}
}

func TestChatCompletionToResponsesUsesFirstChoiceToolCallsOnly(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-tools",
		"object":"chat.completion",
		"created":1710000000,
		"model":"llama-free",
		"choices":[
			{"index":0,"message":{"role":"assistant","content":"pong","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]},"finish_reason":"tool_calls"},
			{"index":1,"message":{"role":"assistant","content":"ignored"},"finish_reason":"stop"}
		],
		"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if len(resp.Output) != 2 {
		t.Fatalf("expected one assistant message and one function call, got %#v", resp.Output)
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Role != "assistant" {
		t.Fatalf("unexpected assistant output: %#v", resp.Output[0])
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Text != "pong" {
		t.Fatalf("expected first choice text, got %#v", resp.Output[0].Content)
	}
	if resp.Output[1].Type != "function_call" || resp.Output[1].ID != "call_1" || resp.Output[1].CallID != "call_1" || resp.Output[1].Name != "lookup" {
		t.Fatalf("unexpected function call output: %#v", resp.Output[1])
	}
}

func TestChatCompletionToResponsesPreservesAssistantToolCalls(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-tool",
		"object":"chat.completion",
		"created":1710000000,
		"model":"llama-free",
		"choices":[{"index":0,"message":{"role":"assistant","content":"pong","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]},"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
	}`)

	resp, status, err := ChatCompletionToResponses(body, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionToResponses returned error: %v", err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if len(resp.Output) != 2 {
		t.Fatalf("expected text plus tool call outputs, got %#v", resp.Output)
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Role != "assistant" {
		t.Fatalf("unexpected assistant output: %#v", resp.Output[0])
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Text != "pong" {
		t.Fatalf("unexpected assistant text content: %#v", resp.Output[0].Content)
	}
	if resp.Output[1].Type != "function_call" || resp.Output[1].ID != "call_1" || resp.Output[1].CallID != "call_1" || resp.Output[1].Name != "lookup" {
		t.Fatalf("unexpected function call output: %#v", resp.Output[1])
	}
	if resp.Output[1].Arguments != `{"q":"x"}` {
		t.Fatalf("expected function call arguments preserved, got %#v", resp.Output[1].Arguments)
	}
}

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

func TestChatCompletionStreamToResponsesEventsEmitsFailedBeforeCreatedForErrorChunk(t *testing.T) {
	raw := []byte("data: {\"error\":{\"message\":\"upstream failed\",\"type\":\"server_error\",\"code\":\"bad_gateway\"}}\n\n")

	events, err := ChatCompletionStreamToResponsesEvents(raw, "cct/free")
	if err != nil {
		t.Fatalf("ChatCompletionStreamToResponsesEvents returned error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected failure event, got no events")
	}
	if events[0].Event != "response.failed" {
		t.Fatalf("expected first event to be response.failed, got %#v", events[0])
	}
	for _, event := range events {
		if event.Event == "response.created" {
			t.Fatalf("did not expect response.created for pure error chunk: %#v", events)
		}
	}
}

func TestWriteResponsesSSEWritesEventAndDataLines(t *testing.T) {
	var buf bytes.Buffer
	events := []ResponsesSSEEvent{
		{
			Event: "response.created",
			Data: map[string]any{
				"id":     "chatcmpl-1",
				"status": "in_progress",
			},
		},
		{
			Event: "response.completed",
			Data: map[string]any{
				"id":     "chatcmpl-1",
				"status": "completed",
			},
		},
	}

	if err := WriteResponsesSSE(&buf, events); err != nil {
		t.Fatalf("WriteResponsesSSE returned error: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, "event: response.created\n") || !strings.Contains(body, "event: response.completed\n") {
		t.Fatalf("expected event lines, got %q", body)
	}
	if !strings.Contains(body, "\"id\":\"chatcmpl-1\"") || !strings.Contains(body, "\"status\":\"completed\"") {
		t.Fatalf("expected marshaled data lines, got %q", body)
	}
}
