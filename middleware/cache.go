package middleware

import (
	"github.com/gin-gonic/gin"
	"path/filepath"
	"strings"
)

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if isStaticAsset(path) {
			c.Header("Cache-Control", "public, max-age=604800, immutable")
		} else {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		}
		c.Next()
	}
}

func isStaticAsset(path string) bool {
	if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".css", ".js", ".map", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot":
		return true
	default:
		return false
	}
}
