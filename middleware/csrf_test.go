package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSessionCSRF(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		cookie           bool
		headers          map[string]string
		requestURL       string
		trustedAddresses []string
		wantStatus       int
	}{
		{
			name: "cross-site fetch metadata is rejected", method: http.MethodPost, cookie: true,
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusForbidden,
		},
		{
			name: "mismatched origin is rejected", method: http.MethodPut, cookie: true,
			headers: map[string]string{"Origin": "https://evil.example"}, wantStatus: http.StatusForbidden,
		},
		{
			name: "same origin is allowed", method: http.MethodPut, cookie: true,
			headers: map[string]string{"Origin": "https://app.example"}, wantStatus: http.StatusNoContent,
		},
		{
			name: "default https port is normalized", method: http.MethodPut, cookie: true,
			headers: map[string]string{"Origin": "https://app.example"}, requestURL: "https://app.example:443/api/probe", wantStatus: http.StatusNoContent,
		},
		{
			name: "configured public address survives proxy host rewrite", method: http.MethodPut, cookie: true,
			headers: map[string]string{"Origin": "https://app.example"}, requestURL: "http://127.0.0.1:3008/api/probe",
			trustedAddresses: []string{"https://app.example"}, wantStatus: http.StatusNoContent,
		},
		{
			name: "mismatched referer is rejected", method: http.MethodDelete, cookie: true,
			headers: map[string]string{"Referer": "https://evil.example/settings"}, wantStatus: http.StatusForbidden,
		},
		{
			name: "same origin referer is allowed", method: http.MethodDelete, cookie: true,
			headers: map[string]string{"Referer": "https://app.example/settings"}, wantStatus: http.StatusNoContent,
		},
		{
			name: "non-browser compatibility is preserved", method: http.MethodPost, cookie: true,
			wantStatus: http.StatusNoContent,
		},
		{
			name: "token request without session cookie bypasses guard", method: http.MethodPost,
			headers: map[string]string{"Origin": "https://evil.example"}, wantStatus: http.StatusNoContent,
		},
		{
			name: "safe method bypasses guard", method: http.MethodGet, cookie: true,
			headers: map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.Use(SessionCSRF("session", tt.trustedAddresses...))
			engine.Handle(tt.method, "/api/probe", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			requestURL := tt.requestURL
			if requestURL == "" {
				requestURL = "https://app.example/api/probe"
			}
			request := httptest.NewRequest(tt.method, requestURL, nil)
			if tt.cookie {
				request.AddCookie(&http.Cookie{Name: "session", Value: "signed-session"})
			}
			for key, value := range tt.headers {
				request.Header.Set(key, value)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}
