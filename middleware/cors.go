package middleware

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func TokenAPICORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = false
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{
		"Authorization",
		"Content-Type",
		"Accept",
		"Origin",
		"X-API-Key",
		"Anthropic-Version",
		"Anthropic-Beta",
		"OpenAI-Beta",
	}
	handler := cors.New(config)
	return func(c *gin.Context) {
		if !isTokenAPIPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		handler(c)
	}
}

func isTokenAPIPath(path string) bool {
	if path == "/dashboard/billing/subscription" || path == "/dashboard/billing/usage" {
		return true
	}
	for _, prefix := range []string{
		"/v1/models",
		"/v1/dashboard/billing",
		"/v1/oneapi/proxy",
		"/v1/completions",
		"/v1/chat/completions",
		"/v1/responses",
		"/v1/messages",
		"/v1/edits",
		"/v1/images",
		"/v1/embeddings",
		"/v1/engines",
		"/v1/audio",
		"/v1/files",
		"/v1/fine_tuning",
		"/v1/moderations",
		"/v1/assistants",
		"/v1/threads",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
