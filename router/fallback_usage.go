package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
)

func getFreePoolUsage(c *gin.Context) {
	rows, err := fallback.ListFreeProviderUsage(fallback.FreeProviderUsageFilter{
		Provider:  c.Query("provider"),
		KeyHash:   c.Query("key_hash"),
		ModelName: c.Query("model"),
		Period:    c.Query("period"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}
