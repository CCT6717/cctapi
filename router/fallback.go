package router

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/fallback"
	"github.com/songquanpeng/one-api/middleware"
	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
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
					"deployment_id":   deploymentID,
					"success":         err == nil,
					"cooldown_until":  until,
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
	}

	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, common.FormatPrometheusMetrics())
	})

	router.GET("/fallback/dashboard", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/fallback/status")
	})
}

const fallbackEditorConfigPath = "data/fallback.json"

type fallbackEditorConfig struct {
	Enabled       bool                         `json:"enabled"`
	VirtualModels []fallbackEditorVirtualModel `json:"virtual_models"`
	Deployments   []fallbackEditorDeployment   `json:"deployments"`
	Alert         fallback.AlertConfig         `json:"alert"`
	SmartSort     fallback.SmartSortConfig     `json:"smart_sort"`
}

type fallbackEditorVirtualModel struct {
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Description   string   `json:"description"`
	RoutingMode   string   `json:"routing_mode"`
	FallbackOrder []string `json:"fallback_order"`
}

type fallbackEditorDeployment struct {
	ID                    string                `json:"id"`
	Enabled               bool                  `json:"enabled"`
	ChannelID             int                   `json:"channel_id"`
	RealModel             string                `json:"real_model"`
	Priority              int                   `json:"priority"`
	Weight                int                   `json:"weight"`
	MaxConcurrentRequests int                   `json:"max_concurrent_requests"`
	DailyLimitTokens      int64                 `json:"daily_limit_tokens"`
	QuotaMode            string                `json:"quota_mode"`
	SoftLimitRatio        float64               `json:"soft_limit_ratio"`
	HardLimitRatio        float64               `json:"hard_limit_ratio"`
	MaxContext            int                   `json:"max_context"`
	MinContext            int                   `json:"min_context"`
	Channel               fallbackEditorChannel `json:"channel"`
}

type fallbackEditorChannel struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    int    `json:"type"`
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
	Models  string `json:"models"`
	Status  int    `json:"status"`
}

func getFallbackEditorConfig(c *gin.Context) {
	cfg := fallback.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback config is not loaded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": buildFallbackEditorConfig(cfg)})
}

func updateFallbackEditorConfig(c *gin.Context) {
	var payload fallbackEditorConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	virtualModels, deployments, err := normalizeFallbackEditorPayload(payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	deployments, err = upsertFallbackEditorChannels(deployments)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	cfg := buildFallbackConfigFromEditor(payload, virtualModels, deployments)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	data = append(data, '\n')

	backupPath, err := backupFallbackEditorConfig(fallbackEditorConfigPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := os.WriteFile(fallbackEditorConfigPath, data, 0644); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := fallback.ReloadConfig(fallbackEditorConfigPath); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	response := gin.H{"success": true, "message": "fallback config saved", "data": buildFallbackEditorConfig(&cfg)}
	if backupPath != "" {
		response["backup_path"] = backupPath
	}
	c.JSON(http.StatusOK, response)
}

func backupFallbackEditorConfig(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read old fallback config for backup: %w", err)
	}

	ext := filepath.Ext(configPath)
	base := strings.TrimSuffix(filepath.Base(configPath), ext)
	backupDir := filepath.Join(filepath.Dir(configPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create fallback config backup directory: %w", err)
	}

	now := time.Now()
	backupName := fmt.Sprintf("%s.%s-%09d%s", base, now.Format("20060102-150405"), now.Nanosecond(), ext)
	backupPath := filepath.Join(backupDir, backupName)
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write fallback config backup: %w", err)
	}
	return backupPath, nil
}

func buildFallbackEditorConfig(cfg *fallback.Config) fallbackEditorConfig {
	vmNames := make([]string, 0, len(cfg.VirtualModels))
	for name := range cfg.VirtualModels {
		vmNames = append(vmNames, name)
	}
	sort.Strings(vmNames)

	virtualModels := make([]fallbackEditorVirtualModel, 0, len(vmNames))
	for _, name := range vmNames {
		vm := cfg.VirtualModels[name]
		virtualModels = append(virtualModels, fallbackEditorVirtualModel{
			Name:          name,
			Enabled:       vm.Enabled,
			Description:   vm.Description,
			RoutingMode:   vm.RoutingMode,
			FallbackOrder: append([]string{}, vm.FallbackOrder...),
		})
	}

	deploymentIDs := make([]string, 0, len(cfg.Deployments))
	for id := range cfg.Deployments {
		deploymentIDs = append(deploymentIDs, id)
	}
	sort.SliceStable(deploymentIDs, func(i, j int) bool {
		left := cfg.Deployments[deploymentIDs[i]]
		right := cfg.Deployments[deploymentIDs[j]]
		if left.Priority == right.Priority {
			return deploymentIDs[i] < deploymentIDs[j]
		}
		return left.Priority < right.Priority
	})

	deployments := make([]fallbackEditorDeployment, 0, len(deploymentIDs))
	for _, id := range deploymentIDs {
		dep := cfg.Deployments[id]
		dep.ID = id
		editorDep := fallbackEditorDeployment{
			ID:                    id,
			Enabled:               dep.Enabled,
			ChannelID:             dep.ChannelID,
			RealModel:             dep.RealModel,
			Priority:              dep.Priority,
			Weight:                dep.Weight,
			MaxConcurrentRequests: dep.MaxConcurrentRequests,
			DailyLimitTokens:      dep.DailyLimitTokens,
			QuotaMode:             dep.QuotaMode,
			SoftLimitRatio:        dep.SoftLimitRatio,
			HardLimitRatio:        dep.HardLimitRatio,
			MaxContext:            dep.MaxContext,
			MinContext:            dep.MinContext,
		}
		if dep.ChannelID > 0 {
			if channel, err := dbmodel.GetChannelById(dep.ChannelID, true); err == nil {
				editorDep.Channel = buildFallbackEditorChannel(channel)
			}
		}
		deployments = append(deployments, editorDep)
	}

	return fallbackEditorConfig{
		Enabled:       cfg.Enabled,
		VirtualModels: virtualModels,
		Deployments:   deployments,
		Alert:         cfg.Alert,
		SmartSort:     cfg.SmartSort,
	}
}

func buildFallbackEditorChannel(channel *dbmodel.Channel) fallbackEditorChannel {
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = *channel.BaseURL
	}

	return fallbackEditorChannel{
		ID:      channel.Id,
		Name:    channel.Name,
		Type:    channel.Type,
		BaseURL: baseURL,
		Key:     channel.Key,
		Models:  channel.Models,
		Status:  channel.Status,
	}
}

func normalizeFallbackEditorPayload(payload fallbackEditorConfig) ([]fallbackEditorVirtualModel, []fallbackEditorDeployment, error) {
	if len(payload.VirtualModels) == 0 {
		return nil, nil, fmt.Errorf("at least one virtual model is required")
	}

	deployments := make([]fallbackEditorDeployment, 0, len(payload.Deployments))
	deploymentIDs := make(map[string]bool)
	for _, dep := range payload.Deployments {
		dep.ID = strings.TrimSpace(dep.ID)
		dep.RealModel = strings.TrimSpace(dep.RealModel)
		dep.Channel.Name = strings.TrimSpace(dep.Channel.Name)
		dep.Channel.BaseURL = strings.TrimSpace(dep.Channel.BaseURL)
		dep.Channel.Models = strings.TrimSpace(dep.Channel.Models)
		if dep.ID == "" {
			return nil, nil, fmt.Errorf("deployment id is required")
		}
		if deploymentIDs[dep.ID] {
			return nil, nil, fmt.Errorf("duplicate deployment id: %s", dep.ID)
		}
		if dep.RealModel == "" {
			return nil, nil, fmt.Errorf("real model is required for deployment %s", dep.ID)
		}
		if dep.SoftLimitRatio <= 0 {
			dep.SoftLimitRatio = 0.9
		}
		if dep.Weight <= 0 {
			dep.Weight = 100
		}
		if dep.MaxConcurrentRequests < 0 {
			dep.MaxConcurrentRequests = 0
		}
		if dep.HardLimitRatio <= 0 {
			dep.HardLimitRatio = 0.98
		}
		deploymentIDs[dep.ID] = true
		deployments = append(deployments, dep)
	}

	virtualModels := make([]fallbackEditorVirtualModel, 0, len(payload.VirtualModels))
	vmNames := make(map[string]bool)
	for _, vm := range payload.VirtualModels {
		vm.Name = strings.TrimSpace(vm.Name)
		vm.RoutingMode = fallback.NormalizeRoutingMode(vm.RoutingMode)
		if vm.Name == "" {
			return nil, nil, fmt.Errorf("virtual model name is required")
		}
		if vmNames[vm.Name] {
			return nil, nil, fmt.Errorf("duplicate virtual model: %s", vm.Name)
		}
		if vm.Enabled && len(vm.FallbackOrder) == 0 {
			return nil, nil, fmt.Errorf("enabled virtual model %s needs at least one deployment", vm.Name)
		}

		order := make([]string, 0, len(vm.FallbackOrder))
		for _, id := range vm.FallbackOrder {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if !deploymentIDs[id] {
				return nil, nil, fmt.Errorf("virtual model %s references unknown deployment %s", vm.Name, id)
			}
			order = append(order, id)
		}
		vm.FallbackOrder = order
		vmNames[vm.Name] = true
		virtualModels = append(virtualModels, vm)
	}

	return virtualModels, deployments, nil
}

func buildFallbackConfigFromEditor(payload fallbackEditorConfig, virtualModels []fallbackEditorVirtualModel, deployments []fallbackEditorDeployment) fallback.Config {
	current := fallback.GetConfig()

	alertConfig := payload.Alert
	if alertConfig.CheckIntervalSec <= 0 && current != nil {
		alertConfig = current.Alert
	}
	if alertConfig.CheckIntervalSec <= 0 {
		alertConfig.CheckIntervalSec = 300
	}

	smartSortConfig := payload.SmartSort
	if smartSortConfig.Weights.BasePriorityPenalty <= 0 {
		if current != nil {
			smartSortConfig = current.SmartSort
		} else {
			smartSortConfig = fallback.DefaultSmartSortConfig()
		}
	}

	cfg := fallback.Config{
		Enabled:       payload.Enabled,
		VirtualModels: make(map[string]fallback.VirtualModelConfig),
		Deployments:   make(map[string]fallback.DeploymentConfig),
		Alert:         alertConfig,
		SmartSort:     smartSortConfig,
	}
	if !cfg.Enabled && current != nil {
		cfg.Enabled = current.Enabled
	}
	if !cfg.Enabled {
		cfg.Enabled = true
	}

	for _, vm := range virtualModels {
		cfg.VirtualModels[vm.Name] = fallback.VirtualModelConfig{
			Enabled:       vm.Enabled,
			Description:   vm.Description,
			RoutingMode:   fallback.NormalizeRoutingMode(vm.RoutingMode),
			FallbackOrder: append([]string{}, vm.FallbackOrder...),
		}
	}

	for _, dep := range deployments {
		cfg.Deployments[dep.ID] = fallback.DeploymentConfig{
			Enabled:               dep.Enabled,
			ChannelID:             dep.ChannelID,
			RealModel:             dep.RealModel,
			Priority:              dep.Priority,
			Weight:                dep.Weight,
			MaxConcurrentRequests: dep.MaxConcurrentRequests,
			DailyLimitTokens:      dep.DailyLimitTokens,
			QuotaMode:             dep.QuotaMode,
			SoftLimitRatio:        dep.SoftLimitRatio,
			HardLimitRatio:        dep.HardLimitRatio,
			MaxContext:            dep.MaxContext,
			MinContext:            dep.MinContext,
		}
	}

	return cfg
}

func upsertFallbackEditorChannels(deployments []fallbackEditorDeployment) ([]fallbackEditorDeployment, error) {
	channelModels := make(map[int][]string)

	for i := range deployments {
		channelID, err := upsertFallbackEditorChannel(deployments[i])
		if err != nil {
			return nil, err
		}
		deployments[i].ChannelID = channelID
		channelModels[channelID] = append(channelModels[channelID], deployments[i].RealModel)
	}

	for channelID, realModels := range channelModels {
		if err := ensureFallbackEditorChannelModels(channelID, realModels); err != nil {
			return nil, err
		}
	}

	return deployments, nil
}

func upsertFallbackEditorChannel(dep fallbackEditorDeployment) (int, error) {
	channelName := dep.Channel.Name
	if channelName == "" {
		channelName = dep.ID
	}

	channelType := dep.Channel.Type
	if channelType <= 0 {
		channelType = channeltype.OpenAICompatible
	}

	channelStatus := dep.Channel.Status
	if channelStatus <= 0 {
		channelStatus = dbmodel.ChannelStatusEnabled
	}

	if dep.ChannelID <= 0 {
		baseURL := dep.Channel.BaseURL
		channel := dbmodel.Channel{
			Type:        channelType,
			Key:         dep.Channel.Key,
			Status:      channelStatus,
			Name:        channelName,
			CreatedTime: helper.GetTimestamp(),
			BaseURL:     &baseURL,
			Models:      dep.RealModel,
			Group:       "default",
		}
		if err := channel.Insert(); err != nil {
			return 0, fmt.Errorf("failed to create channel for deployment %s: %w", dep.ID, err)
		}
		return channel.Id, nil
	}

	channel, err := dbmodel.GetChannelById(dep.ChannelID, true)
	if err != nil {
		return 0, fmt.Errorf("failed to load channel %d for deployment %s: %w", dep.ChannelID, dep.ID, err)
	}

	if channelName == "" {
		channelName = channel.Name
	}
	if channelName == "" {
		channelName = dep.ID
	}
	if channelType <= 0 {
		channelType = channel.Type
	}
	if channelType <= 0 {
		channelType = channeltype.OpenAICompatible
	}
	if channelStatus <= 0 {
		channelStatus = channel.Status
	}
	if channelStatus <= 0 {
		channelStatus = dbmodel.ChannelStatusEnabled
	}
	group := channel.Group
	if group == "" {
		group = "default"
	}

	updates := map[string]interface{}{
		"name":     channelName,
		"type":     channelType,
		"key":      dep.Channel.Key,
		"base_url": dep.Channel.BaseURL,
		"status":   channelStatus,
		"group":    group,
	}
	if err := dbmodel.DB.Model(&dbmodel.Channel{}).Where("id = ?", channel.Id).Select("name", "type", "key", "base_url", "status", "group").Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("failed to update channel %d for deployment %s: %w", channel.Id, dep.ID, err)
	}

	return channel.Id, nil
}

func ensureFallbackEditorChannelModels(channelID int, realModels []string) error {
	channel, err := dbmodel.GetChannelById(channelID, true)
	if err != nil {
		return fmt.Errorf("failed to load channel %d: %w", channelID, err)
	}

	models := mergeModelList(channel.Models, realModels)
	group := channel.Group
	if group == "" {
		group = "default"
	}
	status := channel.Status
	if status <= 0 {
		status = dbmodel.ChannelStatusEnabled
	}

	if err := dbmodel.DB.Model(&dbmodel.Channel{}).Where("id = ?", channel.Id).Select("models", "group", "status").Updates(map[string]interface{}{
		"models": models,
		"group":  group,
		"status": status,
	}).Error; err != nil {
		return fmt.Errorf("failed to update channel %d models: %w", channel.Id, err)
	}

	updated, err := dbmodel.GetChannelById(channelID, true)
	if err != nil {
		return fmt.Errorf("failed to reload channel %d: %w", channelID, err)
	}
	if err := updated.UpdateAbilities(); err != nil {
		return fmt.Errorf("failed to update channel %d abilities: %w", channelID, err)
	}
	return nil
}

func mergeModelList(existing string, required []string) string {
	seen := make(map[string]bool)
	models := make([]string, 0)
	for _, model := range strings.Split(existing, ",") {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		models = append(models, model)
	}
	for _, model := range required {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		models = append(models, model)
	}
	return strings.Join(models, ",")
}

func renderDashboardHTML(status []map[string]interface{}) string {
	rows := ""
	for _, s := range status {
		level := interfaceToString(s["alert_level"])
		levelClass := "badge normal"
		if level == "warning" {
			levelClass = "badge warning"
		} else if level == "critical" {
			levelClass = "badge critical"
		}

		silenced := ""
		if v, ok := s["silenced"].(bool); ok && v {
			silenced = `<span class="muted">已静音</span>`
		}
		stateNote := fallbackDeploymentStateNote(s)

		rows += `<tr>
<td><strong>` + escapeHTML(s["deployment_id"]) + `</strong>` + silenced + `</td>
<td>` + escapeHTML(s["real_model"]) + `</td>
<td><span class="` + levelClass + `">` + escapeHTML(level) + `</span></td>
<td>` + escapeHTML(s["usage_percent"]) + `</td>
<td class="value">` + escapeHTML(s["used_tokens"]) + ` / ` + escapeHTML(s["daily_limit"]) + `</td>
<td>` + escapeHTML(s["alert_type"]) + `</td>
<td class="value">` + html.EscapeString(stateNote) + `</td>
<td class="actions"><button class="action-btn" data-fallback-action="cooldown" data-deployment-id="` + escapeHTML(s["deployment_id"]) + `">冷却 5 分钟</button><button class="action-btn secondary" data-fallback-action="recover" data-deployment-id="` + escapeHTML(s["deployment_id"]) + `">恢复</button></td>
</tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="8" class="empty">暂无 fallback 部署数据</td></tr>`
	}

	return renderPage("Fallback 面板", "部署状态面板", "查看 fallback deployment 的实时状态、用量限制和告警状态。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/api/fallback/alert/status"><span>原始状态数据</span><small>JSON 接口</small></a>
</nav>
<table>
	<thead><tr><th>部署</th><th>模型</th><th>级别</th><th>用量</th><th>Token</th><th>告警</th><th>状态</th><th>操作</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<script>
async function fallbackDeploymentAction(button, deploymentID, action) {
	var url = "/api/fallback/deployments/" + encodeURIComponent(deploymentID);
	if (action === "cooldown") {
		url += "/cooldown?duration_seconds=300";
	} else if (action === "recover") {
		url += "/recover";
	} else {
		return;
	}
	button.disabled = true;
	try {
		var response = await fetch(url, { method: "POST", credentials: "same-origin" });
		var data = await response.json().catch(function(){ return {}; });
		if (!response.ok || data.success === false || data.error) {
			throw new Error(data.message || (data.error && data.error.message) || "操作失败");
		}
		location.reload();
	} catch (error) {
		alert("操作失败：" + error.message);
		button.disabled = false;
	}
}
document.addEventListener("click", function(event) {
	var button = event.target.closest("[data-fallback-action]");
	if (!button) return;
	fallbackDeploymentAction(button, button.getAttribute("data-deployment-id"), button.getAttribute("data-fallback-action"));
});
setTimeout(function(){ location.reload(); }, 15000);
</script>`)
}

func fallbackDeploymentStateNote(status map[string]interface{}) string {
	alertType := interfaceToString(status["alert_type"])
	switch alertType {
	case "exhausted":
		return "耗尽至 " + formatFallbackTime(status["exhausted_until"])
	case "cooldown":
		return "冷却至 " + formatFallbackTime(status["cooldown_until"])
	default:
		return "可用"
	}
}

func formatFallbackTime(value interface{}) string {
	if value == nil {
		return "-"
	}
	switch v := value.(type) {
	case *time.Time:
		if v == nil || v.IsZero() {
			return "-"
		}
		return v.Local().Format("2006-01-02 15:04:05")
	case time.Time:
		if v.IsZero() {
			return "-"
		}
		return v.Local().Format("2006-01-02 15:04:05")
	default:
		return interfaceToString(v)
	}
}

func renderMetricsHTML(metrics string) string {
	rows := ""
	for _, line := range strings.Split(metrics, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		name := parts[0]
		value := ""
		if len(parts) > 1 {
			value = parts[1]
		}
		rows += `<tr><td><code>` + html.EscapeString(name) + `</code></td><td class="value">` + html.EscapeString(value) + `</td></tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="2" class="empty">暂无指标数据</td></tr>`
	}

	return renderPage("Fallback 监控指标", "监控指标面板", "以面板形式查看 fallback 的 Prometheus 计数器。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/metrics"><span>原始指标数据</span><small>Prometheus 文本</small></a>
</nav>
<table>
	<thead><tr><th>指标</th><th>值</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<details>
	<summary>原始指标内容</summary>
	<pre>`+html.EscapeString(metrics)+`</pre>
</details>
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderScoresHTML(allScores map[string]map[string]float64) string {
	vmNames := make([]string, 0, len(allScores))
	for vmName := range allScores {
		vmNames = append(vmNames, vmName)
	}
	sort.Strings(vmNames)

	content := ""
	for _, vmName := range vmNames {
		scores := allScores[vmName]
		deployments := make([]string, 0, len(scores))
		for deploymentID := range scores {
			deployments = append(deployments, deploymentID)
		}
		sort.SliceStable(deployments, func(i, j int) bool {
			return scores[deployments[i]] > scores[deployments[j]]
		})

		rows := ""
		for i, deploymentID := range deployments {
			rows += `<tr><td>` + fmt.Sprintf("%d", i+1) + `</td><td><strong>` + html.EscapeString(deploymentID) + `</strong></td><td class="value">` + fmt.Sprintf("%.2f", scores[deploymentID]) + `</td></tr>`
		}
		if rows == "" {
			rows = `<tr><td colspan="3" class="empty">暂无排序分数</td></tr>`
		}

		content += `<section class="panel">
<h2>` + html.EscapeString(vmName) + `</h2>
<table><thead><tr><th>排名</th><th>部署</th><th>分数</th></tr></thead><tbody>` + rows + `</tbody></table>
</section>`
	}
	if content == "" {
		content = `<div class="empty">暂无虚拟模型</div>`
	}

	return renderPage("Fallback 排序分数", "排序分数面板", "查看 fallback 智能排序当前使用的 deployment 得分。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/api/fallback/sort/scores"><span>原始分数数据</span><small>JSON 接口</small></a>
</nav>`+content+`
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderLogsHTML(events []fallback.SwitchEvent) string {
	rows := ""
	for _, event := range events {
		status := "-"
		statusClass := "badge normal"
		if event.StatusCode > 0 {
			status = fmt.Sprintf("%d", event.StatusCode)
			if event.StatusCode >= 500 {
				statusClass = "badge critical"
			} else if event.StatusCode >= 400 {
				statusClass = "badge warning"
			}
		}

		duration := "-"
		if event.DurationMs > 0 {
			duration = fmt.Sprintf("%dms", event.DurationMs)
		}
		requestID := event.RequestID
		if requestID == "" {
			requestID = "-"
		}

		rows += `<tr>
<td class="value">` + html.EscapeString(event.CreatedAt.Local().Format("2006-01-02 15:04:05")) + `</td>
<td><strong>` + html.EscapeString(event.VirtualModel) + `</strong></td>
<td><strong>` + html.EscapeString(event.FromDeployment) + `</strong> → <strong>` + html.EscapeString(event.ToDeployment) + `</strong></td>
<td>` + html.EscapeString(event.Reason) + `</td>
<td><span class="` + statusClass + `">` + html.EscapeString(status) + `</span></td>
<td class="value">` + html.EscapeString(duration) + `</td>
<td><code>` + html.EscapeString(requestID) + `</code></td>
</tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="7" class="empty">暂无回退切换事件</td></tr>`
	}

	return renderPage("Fallback 回退事件日志", "回退事件日志", "查看 fallback 最近的部署切换、原因和耗时。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/api/fallback/logs?limit=100"><span>原始事件数据</span><small>JSON 接口</small></a>
</nav>
<table>
	<thead><tr><th>时间</th><th>虚拟模型</th><th>切换</th><th>原因</th><th>状态码</th><th>耗时</th><th>请求 ID</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderAlertHistoryHTML(events []fallback.AlertHistoryEvent) string {
	rows := ""
	for _, event := range events {
		levelClass := "badge normal"
		switch event.Level {
		case string(fallback.AlertWarning):
			levelClass = "badge warning"
		case string(fallback.AlertCritical):
			levelClass = "badge critical"
		}

		tokens := "-"
		if event.DailyLimit > 0 {
			tokens = fmt.Sprintf("%d / %d", event.UsedTokens, event.DailyLimit)
		} else if event.UsedTokens > 0 {
			tokens = fmt.Sprintf("%d", event.UsedTokens)
		}

		percentage := "-"
		if event.Percentage > 0 {
			percentage = fmt.Sprintf("%.1f%%", event.Percentage)
		}

		rows += `<tr>
<td class="value">` + html.EscapeString(event.CreatedAt.Local().Format("2006-01-02 15:04:05")) + `</td>
<td><strong>` + html.EscapeString(event.DeploymentID) + `</strong></td>
<td><span class="` + levelClass + `">` + html.EscapeString(event.Level) + `</span></td>
<td><code>` + html.EscapeString(event.Type) + `</code></td>
<td class="value">` + html.EscapeString(tokens) + `</td>
<td class="value">` + html.EscapeString(percentage) + `</td>
<td>` + html.EscapeString(event.Message) + `</td>
</tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="7" class="empty">暂无告警历史</td></tr>`
	}

	return renderPage("Fallback 告警历史", "告警历史", "查看 fallback deployment 的限额、冷却、耗尽和恢复记录。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/api/fallback/alert/history?limit=100"><span>原始告警数据</span><small>JSON 接口</small></a>
</nav>
<table>
	<thead><tr><th>时间</th><th>部署</th><th>级别</th><th>类型</th><th>Token</th><th>用量</th><th>消息</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderPage(title, heading, subtitle, body string) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
:root { color-scheme: light; --text:#172033; --muted:#667085; --line:#d9e0ea; --soft:#f6f8fb; --blue:#155eef; --green:#067647; --yellow:#b54708; --red:#b42318; }
* { box-sizing: border-box; }
body { margin: 0; background: #fff; color: var(--text); font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
.wrap { max-width: 1180px; margin: 0 auto; padding: 28px 24px 48px; }
.top { display: flex; justify-content: space-between; align-items: flex-end; gap: 16px; margin-bottom: 20px; border-bottom: 1px solid var(--line); padding-bottom: 18px; }
h1 { margin: 0; font-size: 28px; line-height: 1.2; letter-spacing: 0; }
h2 { margin: 24px 0 10px; font-size: 18px; letter-spacing: 0; }
p { margin: 6px 0 0; color: var(--muted); }
.panel-nav { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; margin: 0 0 18px; }
.nav-card { display: flex; flex-direction: column; justify-content: center; min-height: 82px; padding: 14px 16px; border: 1px solid var(--line); border-radius: 8px; color: var(--text); text-decoration: none; background: linear-gradient(180deg, #fff 0%, #f8fafc 100%); box-shadow: 0 1px 2px rgba(16, 24, 40, .04); }
.nav-card:hover { border-color: #b9c7da; box-shadow: 0 6px 18px rgba(16, 24, 40, .08); transform: translateY(-1px); }
.nav-card span { font-size: 16px; font-weight: 700; }
.nav-card small { margin-top: 4px; color: var(--muted); font-size: 12px; }
table { width: 100%; border-collapse: collapse; border: 1px solid var(--line); border-radius: 8px; overflow: hidden; }
th, td { padding: 11px 12px; text-align: left; border-bottom: 1px solid var(--line); vertical-align: middle; }
th { background: var(--soft); color: #344054; font-weight: 600; font-size: 12px; text-transform: uppercase; }
tr:last-child td { border-bottom: 0; }
code, pre { font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace; }
pre { white-space: pre-wrap; background: #0f172a; color: #e2e8f0; border-radius: 8px; padding: 14px; overflow: auto; }
details { margin-top: 16px; }
summary { cursor: pointer; color: var(--blue); font-weight: 600; }
.badge { display: inline-flex; align-items: center; min-width: 72px; justify-content: center; border-radius: 999px; padding: 3px 9px; font-size: 12px; font-weight: 700; text-transform: capitalize; }
.normal { color: var(--green); background: #ecfdf3; }
.warning { color: var(--yellow); background: #fffaeb; }
.critical { color: var(--red); background: #fef3f2; }
.muted { margin-left: 8px; color: var(--muted); font-size: 12px; }
.value { font-variant-numeric: tabular-nums; font-weight: 700; }
.actions { display: flex; gap: 8px; flex-wrap: wrap; min-width: 160px; }
.action-btn { border: 1px solid #b9c7da; border-radius: 8px; background: #fff; color: var(--blue); cursor: pointer; font-weight: 700; padding: 7px 10px; }
.action-btn.secondary { color: var(--green); }
.action-btn:hover { background: #f8fafc; border-color: #8ea3bf; }
.action-btn:disabled { cursor: wait; opacity: .6; }
.panel { margin-top: 18px; }
.empty { color: var(--muted); text-align: center; padding: 24px; }
@media (max-width: 760px) {
	.wrap { padding: 20px 14px 36px; }
	.top { display: block; }
	.panel-nav { grid-template-columns: 1fr; }
	table { display: block; overflow-x: auto; white-space: nowrap; }
}
</style>
</head>
<body>
<main class="wrap">
	<div class="top"><div><h1>` + html.EscapeString(heading) + `</h1><p>` + html.EscapeString(subtitle) + `</p></div></div>
	` + body + `
</main>
</body>
</html>`
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return "-"
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func escapeHTML(v interface{}) string {
	return html.EscapeString(interfaceToString(v))
}


