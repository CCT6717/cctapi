package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/claudeutil"
	"github.com/songquanpeng/one-api/common/logger"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				ctx := c.Request.Context()
				logger.Errorf(ctx, "panic detected: %v", err)
				logger.Errorf(ctx, "stacktrace from panic: %s", string(debug.Stack()))
				logger.Errorf(ctx, "request: %s %s", c.Request.Method, c.Request.URL.Path)
				claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_panic", "服务内部错误，请稍后重试")
				c.Abort()
			}
		}()
		c.Next()
	}
}
