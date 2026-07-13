package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
)

func TestGlobalWebRateLimitSkipsStaticAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalLimit := config.GlobalWebRateLimitNum
	originalDuration := config.GlobalWebRateLimitDuration
	originalDebug := config.DebugEnabled
	originalRedis := common.RedisEnabled
	t.Cleanup(func() {
		config.GlobalWebRateLimitNum = originalLimit
		config.GlobalWebRateLimitDuration = originalDuration
		config.DebugEnabled = originalDebug
		common.RedisEnabled = originalRedis
	})

	config.GlobalWebRateLimitNum = 1
	config.GlobalWebRateLimitDuration = 60
	config.DebugEnabled = false
	common.RedisEnabled = false

	router := gin.New()
	router.Use(GlobalWebRateLimit())
	router.GET("/static/app.js", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/dashboard", func(c *gin.Context) { c.Status(http.StatusOK) })

	request := func(path string) int {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.0.2.10:1234"
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}

	if got := request("/static/app.js"); got != http.StatusOK {
		t.Fatalf("first static request: got %d, want %d", got, http.StatusOK)
	}
	if got := request("/static/app.js"); got != http.StatusOK {
		t.Fatalf("second static request: got %d, want %d", got, http.StatusOK)
	}
	if got := request("/dashboard"); got != http.StatusOK {
		t.Fatalf("first page request: got %d, want %d", got, http.StatusOK)
	}
	if got := request("/dashboard"); got != http.StatusTooManyRequests {
		t.Fatalf("second page request: got %d, want %d", got, http.StatusTooManyRequests)
	}
}
