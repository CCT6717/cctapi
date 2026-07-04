package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/middleware"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

func TestResponsesCaptureWriterCapturesStatusHeadersAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	capture := newResponsesCaptureWriter(c.Writer)

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

	if c.Request.URL.Path != "/v1/chat/completions" {
		t.Fatalf("expected chat completions path, got %s", c.Request.URL.Path)
	}
	if got := c.GetString(ctxkey.RequestModel); got != "cct/free" {
		t.Fatalf("expected request model cct/free, got %q", got)
	}

	restore()

	if c.Request.URL.Path != "/v1/responses" {
		t.Fatalf("expected path restored, got %s", c.Request.URL.Path)
	}
}

func TestResponsesConversionStatusUsesConvertedErrorStatusForCapturedOK(t *testing.T) {
	if got := responsesConversionStatus(http.StatusOK, http.StatusTooManyRequests); got != http.StatusTooManyRequests {
		t.Fatalf("expected converted status to win for captured 200, got %d", got)
	}
}

func TestResponsesConversionStatusKeepsCapturedNonOKStatus(t *testing.T) {
	if got := responsesConversionStatus(http.StatusBadGateway, http.StatusTooManyRequests); got != http.StatusBadGateway {
		t.Fatalf("expected captured non-200 status to be preserved, got %d", got)
	}
}

func TestRelayResponsesRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model"`))
	c.Request.Header.Set("Content-Type", "application/json")

	RelayResponses(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["error"]["type"] != "invalid_request_error" {
		t.Fatalf("expected invalid_request_error, got %#v", body)
	}
}

func TestRelayResponsesRejectsUnsupportedInputWithUnprocessableEntity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"cct/free",
		"input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/a.png"}]}]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	RelayResponses(c)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", rec.Code)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["error"]["type"] != "invalid_request_error" {
		t.Fatalf("expected invalid_request_error, got %#v", body)
	}
}

func TestRelayResponsesRestoresWriterBeforeRelayPanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalRelay := relayResponsesRelay
	relayResponsesRelay = func(c *gin.Context) {
		panic("boom")
	}
	defer func() {
		relayResponsesRelay = originalRelay
	}()

	engine := gin.New()
	engine.Use(middleware.RelayPanicRecover())
	engine.POST("/v1/responses", RelayResponses)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"cct/free","input":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d with body %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "one_api_panic") {
		t.Fatalf("expected panic error body written to real client, got %q", rec.Body.String())
	}
}

func TestRelayResponsesSuccessfulNonStreamShim(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalRelay := relayResponsesRelay
	var seenPath string
	var seenModel string
	var seenChatReq relaymodel.GeneralOpenAIRequest
	relayResponsesRelay = func(c *gin.Context) {
		seenPath = c.Request.URL.Path
		seenModel = c.GetString(ctxkey.RequestModel)
		body, err := common.GetRequestBody(c)
		if err != nil {
			t.Fatalf("get request body: %v", err)
		}
		if err := json.Unmarshal(body, &seenChatReq); err != nil {
			t.Fatalf("unmarshal rewritten chat request: %v", err)
		}
		c.Data(http.StatusOK, "application/json", []byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1710000000,
			"model":"llama-free",
			"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
		}`))
	}
	defer func() {
		relayResponsesRelay = originalRelay
	}()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"cct/free","input":"ping"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	RelayResponses(c)

	if seenPath != "/v1/chat/completions" {
		t.Fatalf("expected relay path /v1/chat/completions, got %q", seenPath)
	}
	if seenModel != "cct/free" {
		t.Fatalf("expected request model cct/free, got %q", seenModel)
	}
	if seenChatReq.Model != "cct/free" {
		t.Fatalf("expected rewritten chat model cct/free, got %#v", seenChatReq)
	}
	if len(seenChatReq.Messages) != 1 || seenChatReq.Messages[0].Role != "user" || seenChatReq.Messages[0].StringContent() != "ping" {
		t.Fatalf("unexpected rewritten chat messages: %#v", seenChatReq.Messages)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", rec.Code, rec.Body.String())
	}

	var resp struct {
		Object string `json:"object"`
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Object != "response" || resp.Status != "completed" || resp.Model != "llama-free" {
		t.Fatalf("unexpected shim response envelope: %#v", resp)
	}
	if len(resp.Output) != 1 || resp.Output[0].Type != "message" || resp.Output[0].Role != "assistant" {
		t.Fatalf("unexpected shim output: %#v", resp.Output)
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Type != "output_text" || resp.Output[0].Content[0].Text != "pong" {
		t.Fatalf("unexpected shim content: %#v", resp.Output[0].Content)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 || resp.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected shim usage: %#v", resp.Usage)
	}
}
