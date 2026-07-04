package model

import (
	"encoding/json"
	"errors"
	"reflect"
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
