package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/logger"
)

func SessionCSRF(cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) || !hasCookie(c.Request, cookieName) {
			c.Next()
			return
		}

		if strings.EqualFold(c.GetHeader("Sec-Fetch-Site"), "cross-site") {
			rejectCrossSiteRequest(c, "cross-site fetch metadata")
			return
		}

		if origin := c.GetHeader("Origin"); origin != "" {
			if !sourceMatchesHost(origin, c.Request.Host) {
				rejectCrossSiteRequest(c, "origin mismatch")
				return
			}
			c.Next()
			return
		}

		if referer := c.GetHeader("Referer"); referer != "" {
			if !sourceMatchesHost(referer, c.Request.Host) {
				rejectCrossSiteRequest(c, "referer mismatch")
				return
			}
			c.Next()
			return
		}

		logger.Debug(c.Request.Context(), "session mutation has no browser provenance headers; allowing compatibility request")
		c.Next()
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func hasCookie(request *http.Request, name string) bool {
	_, err := request.Cookie(name)
	return err == nil
}

func sourceMatchesHost(rawSource string, requestHost string) bool {
	source, err := url.Parse(rawSource)
	if err != nil || source.Host == "" {
		return false
	}
	return strings.EqualFold(normalizeHost(source.Host), normalizeHost(requestHost))
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.TrimSpace(host), ".")
}

func rejectCrossSiteRequest(c *gin.Context, reason string) {
	logger.Warnf(c.Request.Context(), "rejected session request: %s", reason)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "跨站请求已被拒绝",
	})
}
