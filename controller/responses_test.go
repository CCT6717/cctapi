package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/ctxkey"
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
