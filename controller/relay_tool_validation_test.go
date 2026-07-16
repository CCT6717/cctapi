package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidateToolCallsSupportsGatewayProtocols(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		requireToolCall bool
		want            bool
	}{
		{name: "chat valid", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{\"q\":\"ok\"}"}}]},"finish_reason":"tool_calls"}]}`, want: true},
		{name: "chat invalid", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{bad"}}]},"finish_reason":"tool_calls"}]}`, want: false},
		{name: "chat text response", body: `{"choices":[{"message":{"content":"no tool needed"},"finish_reason":"stop"}]}`, want: true},
		{name: "chat required tool missing", body: `{"choices":[{"message":{"content":"no tool returned"},"finish_reason":"stop"}]}`, requireToolCall: true, want: false},
		{name: "chat required tool missing from one choice", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{\"q\":\"ok\"}"}}]},"finish_reason":"tool_calls"},{"message":{"content":"no tool returned"},"finish_reason":"stop"}]}`, requireToolCall: true, want: false},
		{name: "responses valid", body: `{"output":[{"type":"function_call","arguments":"{\"q\":\"ok\"}"}]}`, want: true},
		{name: "responses invalid", body: `{"output":[{"type":"function_call","arguments":"not-json"}]}`, want: false},
		{name: "responses required tool missing", body: `{"output":[{"type":"message"}]}`, requireToolCall: true, want: false},
		{name: "anthropic valid", body: `{"content":[{"type":"tool_use","input":{"q":"ok"}}]}`, want: true},
		{name: "anthropic invalid", body: `{"content":[{"type":"tool_use","input":"not-an-object"}]}`, want: false},
		{name: "anthropic required tool missing", body: `{"content":[{"type":"text","text":"no tool returned"}]}`, requireToolCall: true, want: false},
		{name: "unknown schema", body: `{"result":"ok"}`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateToolCalls([]byte(tc.body), tc.requireToolCall); got != tc.want {
				t.Fatalf("validateToolCalls() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequiresToolCall(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "omitted", body: `{}`, want: false},
		{name: "auto", body: `{"tool_choice":"auto"}`, want: false},
		{name: "none", body: `{"tool_choice":"none"}`, want: false},
		{name: "required", body: `{"tool_choice":"required"}`, want: true},
		{name: "selected OpenAI function", body: `{"tool_choice":{"type":"function","function":{"name":"lookup"}}}`, want: true},
		{name: "selected Responses function", body: `{"tool_choice":{"type":"function","name":"lookup"}}`, want: true},
		{name: "selected Anthropic tool", body: `{"tool_choice":{"type":"tool","name":"lookup"}}`, want: true},
		{name: "Anthropic any", body: `{"tool_choice":{"type":"any"}}`, want: true},
		{name: "empty function selection", body: `{"tool_choice":{"type":"function","function":{}}}`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := requiresToolCall([]byte(tc.body)); got != tc.want {
				t.Fatalf("requiresToolCall() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateToolCallsForChoiceSupportsResponsesAndAnthropicContracts(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		response string
		want     bool
	}{
		{name: "Responses selected function matches", request: `{"tool_choice":{"type":"function","name":"lookup"}}`, response: `{"output":[{"type":"function_call","name":"lookup","arguments":"{\"q\":\"ok\"}"}]}`, want: true},
		{name: "Responses selected function mismatch", request: `{"tool_choice":{"type":"function","name":"lookup"}}`, response: `{"output":[{"type":"function_call","name":"other","arguments":"{\"q\":\"wrong\"}"}]}`, want: false},
		{name: "Responses none rejects function", request: `{"tool_choice":"none"}`, response: `{"output":[{"type":"function_call","name":"lookup","arguments":"{\"q\":\"unexpected\"}"}]}`, want: false},
		{name: "Responses required rejects missing function", request: `{"tool_choice":"required"}`, response: `{"output":[{"type":"message"}]}`, want: false},
		{name: "Anthropic any accepts tool", request: `{"tool_choice":{"type":"any"}}`, response: `{"content":[{"type":"tool_use","name":"lookup","input":{"q":"ok"}}]}`, want: true},
		{name: "Anthropic any rejects missing tool", request: `{"tool_choice":{"type":"any"}}`, response: `{"content":[{"type":"text","text":"no tool"}]}`, want: false},
		{name: "Anthropic selected tool mismatch", request: `{"tool_choice":{"type":"tool","name":"lookup"}}`, response: `{"content":[{"type":"tool_use","name":"other","input":{"q":"wrong"}}]}`, want: false},
		{name: "Anthropic none rejects tool", request: `{"tool_choice":{"type":"none"}}`, response: `{"content":[{"type":"tool_use","name":"lookup","input":{"q":"unexpected"}}]}`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contract := parseToolChoice([]byte(tc.request))
			if got := validateToolCallsForChoice([]byte(tc.response), contract); got != tc.want {
				t.Fatalf("validateToolCallsForChoice() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBufferedResponseWriterDoesNotCommitUntilFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	writer := newBufferedResponseWriter(context.Writer)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_, _ = writer.WriteString(`{"ok":true}`)

	if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
		t.Fatalf("buffer committed early: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	writer.flushTo(context.Writer)
	if recorder.Code != http.StatusCreated || recorder.Body.String() != `{"ok":true}` {
		t.Fatalf("flushed response: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("flushed content type = %q", recorder.Header().Get("Content-Type"))
	}
}
