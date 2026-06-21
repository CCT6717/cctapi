package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/fallback"
	"github.com/songquanpeng/one-api/middleware"
)

func SetFallbackRouter(router *gin.Engine) {
	adminGroup := router.Group("/api/fallback", middleware.AdminAuth())
	{
		adminGroup.GET("/states", func(c *gin.Context) {
			virtualModelNames := fallback.GetAllVirtualModelNames()
			if virtualModelNames == nil {
				c.JSON(http.StatusOK, gin.H{"states": map[string]interface{}{}})
				return
			}

			allStates := make(map[string]interface{})
			for _, vmName := range virtualModelNames {
				states, err := fallback.GetAllDeploymentStates(vmName)
				if err != nil {
					continue
				}
				allStates[vmName] = states
			}

			c.JSON(http.StatusOK, gin.H{"states": allStates})
		})

		adminGroup.POST("/deployments/:id/reset", func(c *gin.Context) {
			deploymentID := c.Param("id")
			if err := fallback.ResetDeploymentState(deploymentID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "deployment state reset successfully"})
		})

		adminGroup.POST("/deployments/:id/clear-exhausted", func(c *gin.Context) {
			deploymentID := c.Param("id")
			if err := fallback.ClearDeploymentExhausted(deploymentID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "deployment exhausted state cleared successfully"})
		})

		adminGroup.POST("/deployments/:id/clear-cooldown", func(c *gin.Context) {
			deploymentID := c.Param("id")
			if err := fallback.ClearDeploymentCooldown(deploymentID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "deployment cooldown state cleared successfully"})
		})

		adminGroup.POST("/deployments/:id/cooldown", func(c *gin.Context) {
			deploymentID := c.Param("id")
			durationSeconds := helper.String2Int(c.DefaultQuery("duration_seconds", "300"))
			if durationSeconds <= 0 {
				durationSeconds = 300
			}
			if durationSeconds > 86400 {
				durationSeconds = 86400
			}

			until := time.Now().UTC().Add(time.Duration(durationSeconds) * time.Second)
			if err := fallback.MarkDeploymentCooldown(deploymentID, "manual cooldown", until); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
				return
			}
			_ = fallback.RecordAlertEvent(fallback.AlertEvent{
				DeploymentID: deploymentID,
				Level:        fallback.AlertWarning,
				Type:         fallback.AlertCooldown,
				Message:      fmt.Sprintf("deployment %s manually cooled down until %s", deploymentID, until.Format(time.RFC3339)),
				CreatedAt:    time.Now().UTC(),
			})
			fallback.GlobalAlertManager.MarkAlertFired(deploymentID, fallback.AlertCooldown)

			c.JSON(http.StatusOK, gin.H{"success": true, "message": "deployment manually cooled down", "cooldown_until": until})
		})

		adminGroup.POST("/deployments/:id/recover", func(c *gin.Context) {
			deploymentID := c.Param("id")
			if err := fallback.ResetDeploymentState(deploymentID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
				return
			}
			_ = fallback.RecordAlertEvent(fallback.AlertEvent{
				DeploymentID: deploymentID,
				Level:        fallback.AlertInfo,
				Type:         fallback.AlertRecovered,
				Message:      fmt.Sprintf("deployment %s manually recovered", deploymentID),
				CreatedAt:    time.Now().UTC(),
			})
			fallback.GlobalAlertManager.ClearFiredAlerts(deploymentID)

			c.JSON(http.StatusOK, gin.H{"success": true, "message": "deployment recovered"})
		})

		adminGroup.POST("/deployments/batch-recover", func(c *gin.Context) {
			var payload struct {
				DeploymentIDs []string `json:"deployment_ids"`
			}
			if err := c.ShouldBindJSON(&payload); err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			results := make([]map[string]interface{}, 0)
			for _, deploymentID := range payload.DeploymentIDs {
				err := fallback.ResetDeploymentState(deploymentID)
				if err == nil {
					_ = fallback.RecordAlertEvent(fallback.AlertEvent{
						DeploymentID: deploymentID,
						Level:        fallback.AlertInfo,
						Type:         fallback.AlertRecovered,
						Message:      fmt.Sprintf("deployment %s batch recovered", deploymentID),
						CreatedAt:    time.Now().UTC(),
					})
					fallback.GlobalAlertManager.ClearFiredAlerts(deploymentID)
				}
				results = append(results, map[string]interface{}{
					"deployment_id": deploymentID,
					"success":       err == nil,
				})
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "results": results})
		})

		adminGroup.POST("/deployments/batch-cooldown", func(c *gin.Context) {
			var payload struct {
				DeploymentIDs   []string `json:"deployment_ids"`
				DurationSeconds int      `json:"duration_seconds"`
			}
			if err := c.ShouldBindJSON(&payload); err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			if payload.DurationSeconds <= 0 {
				payload.DurationSeconds = 300
			}
			if payload.DurationSeconds > 86400 {
				payload.DurationSeconds = 86400
			}
			results := make([]map[string]interface{}, 0)
			for _, deploymentID := range payload.DeploymentIDs {
				until := time.Now().UTC().Add(time.Duration(payload.DurationSeconds) * time.Second)
				err := fallback.MarkDeploymentCooldown(deploymentID, "batch cooldown", until)
				if err == nil {
					_ = fallback.RecordAlertEvent(fallback.AlertEvent{
						DeploymentID: deploymentID,
						Level:        fallback.AlertWarning,
						Type:         fallback.AlertCooldown,
						Message:      fmt.Sprintf("deployment %s batch cooled down until %s", deploymentID, until.Format(time.RFC3339)),
						CreatedAt:    time.Now().UTC(),
					})
					fallback.GlobalAlertManager.MarkAlertFired(deploymentID, fallback.AlertCooldown)
				}
				results = append(results, map[string]interface{}{
					"deployment_id":  deploymentID,
					"success":        err == nil,
					"cooldown_until": until,
				})
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "results": results})
		})

		adminGroup.POST("/config/reload", func(c *gin.Context) {
			path := c.Query("path")
			if path == "" {
				path = "data/fallback.json"
			}

			if err := fallback.ReloadConfig(path); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "configuration reloaded successfully"})
		})

		adminGroup.GET("/editor/config", getFallbackEditorConfig)
		adminGroup.POST("/editor/config", updateFallbackEditorConfig)

		adminGroup.POST("/free-pool/sync", func(c *gin.Context) {
			cfg := fallback.GetConfig()
			if cfg == nil || !cfg.Enabled {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback not enabled"})
				return
			}
			if err := fallback.SyncFreePool(cfg); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "free pool synced successfully"})
		})

		adminGroup.POST("/free-pool/cleanup/dry-run", func(c *gin.Context) {
			report, err := fallback.DryRunCleanStale()
			if err != nil {
				// Report is non-nil even on error — include partial results
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": report})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
		})

		adminGroup.GET("/alert/status", func(c *gin.Context) {
			status := fallback.GetAlertStatus()
			c.JSON(http.StatusOK, gin.H{"status": status})
		})

		adminGroup.GET("/alert/history", func(c *gin.Context) {
			limit := helper.String2Int(c.DefaultQuery("limit", "100"))
			events, err := fallback.GetAlertHistory(limit)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": events})
		})

			adminGroup.POST("/alert/read-all", func(c *gin.Context) {
				if err := fallback.MarkAllAlertsRead(); err != nil {
					c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true, "message": "all alerts marked as read"})
			})

			adminGroup.GET("/alert/unread-count", func(c *gin.Context) {
				count, err := fallback.GetUnreadAlertCount()
				if err != nil {
					c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true, "data": count})
			})

		adminGroup.GET("/alert/config", func(c *gin.Context) {
			cfg := fallback.GlobalAlertManager.GetAlertConfig()
			c.JSON(http.StatusOK, gin.H{"config": cfg})
		})

		adminGroup.POST("/alert/deployments/:id/silence", func(c *gin.Context) {
			deploymentID := c.Param("id")
			fallback.GlobalAlertManager.SilenceDeployment(deploymentID)
			c.JSON(http.StatusOK, gin.H{"message": "deployment silenced"})
		})

		adminGroup.POST("/alert/deployments/:id/unsilence", func(c *gin.Context) {
			deploymentID := c.Param("id")
			fallback.GlobalAlertManager.UnsilenceDeployment(deploymentID)
			c.JSON(http.StatusOK, gin.H{"message": "deployment unsilenced"})
		})

		adminGroup.GET("/alert/silenced", func(c *gin.Context) {
			silenced := fallback.GlobalAlertManager.GetSilencedDeployments()
			c.JSON(http.StatusOK, gin.H{"silenced": silenced})
		})

		adminGroup.GET("/sort/scores", func(c *gin.Context) {
			vmNames := fallback.GetAllVirtualModelNames()
			allScores := make(map[string]interface{})
			for _, vmName := range vmNames {
				scores, err := fallback.GetDeploymentScores(vmName)
				if err != nil {
					continue
				}
				allScores[vmName] = scores
			}
			c.JSON(http.StatusOK, gin.H{"scores": allScores})
		})

		adminGroup.GET("/sort/history", func(c *gin.Context) {
			limit := helper.String2Int(c.DefaultQuery("limit", "300"))
			events, err := fallback.GetScoreHistory(limit)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": events})
		})

		adminGroup.GET("/summary", func(c *gin.Context) {
			oneHourAgo := time.Now().UTC().Add(-1 * time.Hour)

			switchEvents, err := fallback.GetSwitchEvents(500)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			switchCount := 0
			for _, event := range switchEvents {
				if !event.CreatedAt.Before(oneHourAgo) {
					switchCount++
				}
			}

			alertEvents, err := fallback.GetAlertHistory(500)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			rateLimitedMap := make(map[string]int)
			for _, event := range alertEvents {
				if event.CreatedAt.Before(oneHourAgo) {
					continue
				}
				if event.Type == string(fallback.AlertExhausted) || event.Type == string(fallback.AlertHardLimit) {
					rateLimitedMap[event.DeploymentID]++
				}
			}
			rateLimited := make([]map[string]interface{}, 0, len(rateLimitedMap))
			for depID, count := range rateLimitedMap {
				rateLimited = append(rateLimited, map[string]interface{}{
					"deployment_id": depID,
					"count":         count,
				})
			}

			coolingDown := make([]string, 0)
			for _, s := range fallback.GetAlertStatus() {
				if alertType, ok := s["alert_type"].(string); ok && alertType == "cooldown" {
					if depID, ok := s["deployment_id"].(string); ok {
						coolingDown = append(coolingDown, depID)
					}
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"switch_count": switchCount,
					"rate_limited": rateLimited,
					"cooling_down": coolingDown,
				},
			})
		})

		adminGroup.GET("/logs", func(c *gin.Context) {
			limit := helper.String2Int(c.DefaultQuery("limit", "100"))
			events, err := fallback.GetSwitchEvents(limit)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": events})
		})

		adminGroup.GET("/sort/order/*model", func(c *gin.Context) {
			modelName := strings.TrimPrefix(c.Param("model"), "/")
			deployments, err := fallback.GetDeploymentsForVirtualModel(modelName)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}
			order := make([]map[string]interface{}, 0)
			for _, dep := range deployments {
				entry := map[string]interface{}{
					"id":         dep.ID,
					"channel_id": dep.ChannelID,
					"real_model": dep.RealModel,
					"priority":   dep.Priority,
					"weight":     dep.Weight,
				}
				order = append(order, entry)
			}
			c.JSON(http.StatusOK, gin.H{"virtual_model": modelName, "order": order})
		})

		adminGroup.POST("/sort/toggle", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "toggle smart_sort.enabled in fallback.json, then reload config"})
		})

		// ── CCT three-tier virtual model gateway endpoints ──

		adminGroup.GET("/virtual-models", func(c *gin.Context) {
			cfg := fallback.GetConfig()
			if cfg == nil || !cfg.Enabled {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback not enabled"})
				return
			}
			names := fallback.GetAllVirtualModelNames()
			vms := make([]map[string]interface{}, 0, len(names))
			for _, name := range names {
				vm, ok := fallback.GetVirtualModel(name)
				if !ok {
					continue
				}
				vms = append(vms, map[string]interface{}{
					"name":                 name,
					"enabled":              vm.Enabled,
					"description":          vm.Description,
					"strategy":             vm.Strategy,
					"pools":                vm.Pools,
					"allow_degrade_to_low": vm.AllowDegradeToLow,
					"allow_degrade_to_free": vm.AllowDegradeToFree,
				})
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": vms})
		})

		adminGroup.GET("/deployments/runtime-status", func(c *gin.Context) {
			cfg := fallback.GetConfig()
			if cfg == nil || !cfg.Enabled {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback not enabled"})
				return
			}
			healthSnap := fallback.SnapshotAllHealth()
			rows := make([]map[string]interface{}, 0, len(cfg.Deployments))
			for id, dep := range cfg.Deployments {
				rt := fallback.SnapshotRuntimeState(id)
				rows = append(rows, map[string]interface{}{
					"deployment_id":       id,
					"enabled":             dep.Enabled,
					"pool":                dep.Pool,
					"real_model":          dep.RealModel,
					"quality_tier":        dep.QualityTier,
					"cost_tier":           dep.CostTier,
					"supports_vision":     dep.SupportsVision,
					"supports_tools":      dep.SupportsTools,
					"supports_json":       dep.SupportsJSON,
					"supports_stream":     dep.SupportsStream,
					"context_length":      dep.ContextLength,
					"rpm_limit":           dep.RPMLimit,
					"rpd_limit":           dep.RPDLimit,
					"tpm_limit":           dep.TPMLimit,
					"tpd_limit":            dep.TPDLimit,
					"minute_requests":     rt.MinuteRequests,
					"day_requests":        rt.DayRequests,
					"minute_tokens":       rt.MinuteTokens,
					"day_tokens":          rt.DayTokens,
					"success_count":       rt.SuccessCount,
					"failure_count":       rt.FailureCount,
					"rate_limit_score":    rt.RateLimitScore,
					"health":              string(fallback.GetHealthStatus(id)),
					"last_error":          rt.LastError,
					"last_error_at":       rt.LastErrorAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": rows, "health": healthSnap})
		})

		adminGroup.GET("/deployments/:id/health", func(c *gin.Context) {
			deploymentID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": map[string]interface{}{
					"deployment_id": deploymentID,
					"health":        string(fallback.GetHealthStatus(deploymentID)),
					"runtime":       fallback.SnapshotRuntimeState(deploymentID),
				},
			})
		})

		adminGroup.POST("/deployments/:id/health-check", func(c *gin.Context) {
			deploymentID := c.Param("id")
			status, err := fallback.TriggerHealthCheckForDeployment(deploymentID)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": map[string]interface{}{
					"deployment_id": deploymentID,
					"health":        string(status),
				},
			})
		})
	}

	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, common.FormatPrometheusMetrics())
	})

	router.GET("/fallback/dashboard", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/fallback/status")
	})
}
