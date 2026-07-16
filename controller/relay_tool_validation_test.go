package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidateToolCallsSupportsGatewayProtocols(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "chat valid", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{\"q\":\"ok\"}"}}]},"finish_reason":"tool_calls"}]}`, want: true},
		{name: "chat invalid", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{bad"}}]},"finish_reason":"tool_calls"}]}`, want: false},
		{name: "chat text response", body: `{"choices":[{"message":{"content":"no tool needed"},"finish_reason":"stop"}]}`, want: true},
		{name: "responses valid", body: `{"output":[{"type":"function_call","arguments":"{\"q\":\"ok\"}"}]}`, want: true},
		{name: "responses invalid", body: `{"output":[{"type":"function_call","arguments":"not-json"}]}`, want: false},
		{name: "anthropic valid", body: `{"content":[{"type":"tool_use","input":{"q":"ok"}}]}`, want: true},
		{name: "anthropic invalid", body: `{"content":[{"type":"tool_use","input":"not-an-object"}]}`, want: false},
		{name: "unknown schema", body: `{"result":"ok"}`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateToolCalls([]byte(tc.body)); got != tc.want {
				t.Fatalf("validateToolCalls() = %v, want %v", got, tc.want)
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
