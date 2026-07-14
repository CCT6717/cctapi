package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRelayPanicRecoverHidesRecoveredError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RelayPanicRecover())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		panic("secret-internal-path")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "secret-internal-path") {
		t.Fatalf("panic response leaked recovered error: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "服务内部错误") {
		t.Fatalf("panic response lacks generic error: %s", recorder.Body.String())
	}
}
