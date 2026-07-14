package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/middleware"
)

func TestCORSScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.TokenAPICORS())
	SetApiRouter(engine)
	SetDashboardRouter(engine)
	SetRelayRouter(engine)
	SetFallbackRouter(engine)

	tests := []struct {
		name       string
		path       string
		method     string
		wantOrigin string
		tokenAPI   bool
	}{
		{name: "session api", path: "/api/user/self", method: http.MethodPut},
		{name: "fallback admin api", path: "/api/fallback/providers", method: http.MethodPut},
		{name: "future session route under v1", path: "/v1/session/probe", method: http.MethodPost},
		{name: "models token api", path: "/v1/models", method: http.MethodGet, wantOrigin: "*", tokenAPI: true},
		{name: "relay token api", path: "/v1/chat/completions", method: http.MethodPost, wantOrigin: "*", tokenAPI: true},
		{name: "dashboard token api", path: "/dashboard/billing/usage", method: http.MethodGet, wantOrigin: "*", tokenAPI: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodOptions, tt.path, nil)
			request.Header.Set("Origin", "https://client.example")
			request.Header.Set("Access-Control-Request-Method", tt.method)
			request.Header.Set("Access-Control-Request-Headers", "authorization,content-type,openai-organization,openai-project,x-stainless-lang,x-stainless-package-version")

			engine.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
			if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Fatalf("credentialed CORS must remain disabled, got %q", got)
			}
			if tt.tokenAPI {
				if recorder.Code != http.StatusNoContent {
					t.Fatalf("preflight status = %d, want %d", recorder.Code, http.StatusNoContent)
				}
				if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, tt.method) {
					t.Fatalf("Access-Control-Allow-Methods = %q, want %s", got, tt.method)
				}
				for _, header := range []string{"Authorization", "Content-Type", "OpenAI-Organization", "OpenAI-Project", "X-Stainless-Lang", "X-Stainless-Package-Version"} {
					if got := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), strings.ToLower(header)) {
						t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", got, header)
					}
				}
			}
		})
	}
}
