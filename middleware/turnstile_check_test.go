package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/config"
)

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial tcp secret-internal-address")
}

func TestTurnstileCheckHidesNetworkErrors(t *testing.T) {
	originalEnabled := config.TurnstileCheckEnabled
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		config.TurnstileCheckEnabled = originalEnabled
		http.DefaultTransport = originalTransport
	})
	config.TurnstileCheckEnabled = true
	http.DefaultTransport = failingRoundTripper{}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret-32-bytes-long-value"))))
	engine.Use(TurnstileCheck())
	engine.POST("/probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/probe?turnstile=test-token", nil)

	engine.ServeHTTP(recorder, request)

	if strings.Contains(recorder.Body.String(), "secret-internal-address") {
		t.Fatalf("Turnstile response leaked network error: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Turnstile 服务暂时不可用") {
		t.Fatalf("response lacks generic error: %s", recorder.Body.String())
	}
}
