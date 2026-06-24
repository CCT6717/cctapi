package claudeutil

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/helper"
)

// IsClaudeRequest checks if the request is a Claude format request.
// Checks context flag first (set during request parsing), then falls back to header/path.
func IsClaudeRequest(c *gin.Context) bool {
	if v, ok := c.Get("claude_format"); ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	if c.GetHeader("anthropic-version") != "" {
		return true
	}
	return strings.HasPrefix(c.Request.URL.Path, "/v1/messages")
}

// WriteClaudeOrOpenAIError writes an error response in Claude or OpenAI format.
func WriteClaudeOrOpenAIError(c *gin.Context, statusCode int, errType string, message string) {
	msg := helper.MessageWithRequestId(message, c.GetString(helper.RequestIdKey))
	if IsClaudeRequest(c) {
		c.JSON(statusCode, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": msg,
			},
		})
	} else {
		c.JSON(statusCode, gin.H{
			"error": gin.H{
				"message": msg,
				"type":    errType,
			},
		})
	}
}
