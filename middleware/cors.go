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
	return path == "/v1" ||
		strings.HasPrefix(path, "/v1/") ||
		path == "/dashboard/billing/subscription" ||
		path == "/dashboard/billing/usage"
}
