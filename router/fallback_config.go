package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/fallback"
	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

const fallbackEditorConfigPath = "data/fallback.json"

type fallbackEditorConfig struct {
	Enabled       bool                         `json:"enabled"`
	VirtualModels []fallbackEditorVirtualModel `json:"virtual_models"`
	Deployments   []fallbackEditorDeployment   `json:"deployments"`
	Channels      []fallbackEditorChannel      `json:"channels"`
	Alert         fallback.AlertConfig         `json:"alert"`
	SmartSort     fallback.SmartSortConfig     `json:"smart_sort"`
}

type fallbackEditorVirtualModel struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Description     string   `json:"description"`
	RoutingMode     string   `json:"routing_mode"`
	FixedDeployment string   `json:"fixed_deployment,omitempty"`
	FallbackOrder   []string `json:"fallback_order"`
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
	QuotaMode             string                `json:"quota_mode"`
	SoftLimitRatio        float64               `json:"soft_limit_ratio"`
	HardLimitRatio        float64               `json:"hard_limit_ratio"`
	MaxContext            int                   `json:"max_context"`
	MinContext            int                   `json:"min_context"`
	Channel               fallbackEditorChannel `json:"channel"`
}

type fallbackEditorChannel struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Type      int      `json:"type"`
	BaseURL   string   `json:"base_url"`
	Key       string   `json:"key"`
	Models    string   `json:"models"`
	ModelList []string `json:"model_list"`
	Status    int      `json:"status"`
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
	backupStem := fmt.Sprintf("%s.%s-%09d", base, now.Format("20060102-150405"), now.Nanosecond())
	backupPath := filepath.Join(backupDir, backupStem+ext)
	for index := 1; ; index++ {
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", fmt.Errorf("failed to inspect fallback config backup path: %w", err)
		}
		backupPath = filepath.Join(backupDir, fmt.Sprintf("%s.%d%s", backupStem, index, ext))
	}
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
			Name:            name,
			Enabled:         vm.Enabled,
			Description:     vm.Description,
			RoutingMode:     vm.RoutingMode,
			FixedDeployment: vm.FixedDeployment,
			FallbackOrder:   append([]string{}, vm.FallbackOrder...),
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
		Channels:      buildFallbackEditorChannels(),
		Alert:         cfg.Alert,
		SmartSort:     cfg.SmartSort,
	}
}

func buildFallbackEditorChannels() []fallbackEditorChannel {
	if dbmodel.DB == nil {
		return []fallbackEditorChannel{}
	}

	channels, err := dbmodel.GetAllChannels(0, 0, "all")
	if err != nil {
		return []fallbackEditorChannel{}
	}

	editorChannels := make([]fallbackEditorChannel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		editorChannels = append(editorChannels, buildFallbackEditorChannel(channel))
	}
	sort.SliceStable(editorChannels, func(i, j int) bool {
		if editorChannels[i].Status == editorChannels[j].Status {
			return editorChannels[i].ID < editorChannels[j].ID
		}
		return editorChannels[i].Status < editorChannels[j].Status
	})
	return editorChannels
}

func buildFallbackEditorChannel(channel *dbmodel.Channel) fallbackEditorChannel {
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = *channel.BaseURL
	}

	return fallbackEditorChannel{
		ID:        channel.Id,
		Name:      channel.Name,
		Type:      channel.Type,
		BaseURL:   baseURL,
		Key:       channel.Key,
		Models:    channel.Models,
		ModelList: splitFallbackEditorChannelModels(channel.Models),
		Status:    channel.Status,
	}
}

func splitFallbackEditorChannelModels(models string) []string {
	seen := make(map[string]bool)
	modelList := make([]string, 0)
	for _, modelName := range strings.Split(models, ",") {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" || seen[modelName] {
			continue
		}
		seen[modelName] = true
		modelList = append(modelList, modelName)
	}
	return modelList
}

func normalizeFallbackEditorBaseURL(channelType int, baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch channelType {
	case channeltype.Doubao:
		return strings.TrimSuffix(baseURL, "/api/v3")
	case channeltype.OpenRouter:
		return strings.TrimSuffix(baseURL, "/v1")
	default:
		return baseURL
	}
}

func normalizeFallbackEditorPayload(payload fallbackEditorConfig) ([]fallbackEditorVirtualModel, []fallbackEditorDeployment, error) {
	if len(payload.VirtualModels) == 0 {
		return nil, nil, fmt.Errorf("at least one virtual model is required")
	}

	deployments := make([]fallbackEditorDeployment, 0, len(payload.Deployments))
	deploymentIDs := make(map[string]bool)
	deploymentEnabled := make(map[string]bool)
	for _, dep := range payload.Deployments {
		dep.ID = strings.TrimSpace(dep.ID)
		dep.RealModel = strings.TrimSpace(dep.RealModel)
		dep.Channel.Name = strings.TrimSpace(dep.Channel.Name)
		dep.Channel.BaseURL = normalizeFallbackEditorBaseURL(dep.Channel.Type, dep.Channel.BaseURL)
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
		deploymentEnabled[dep.ID] = dep.Enabled
		deployments = append(deployments, dep)
	}

	virtualModels := make([]fallbackEditorVirtualModel, 0, len(payload.VirtualModels))
	vmNames := make(map[string]bool)
	for _, vm := range payload.VirtualModels {
		vm.Name = strings.TrimSpace(vm.Name)
		vm.RoutingMode = fallback.NormalizeRoutingMode(vm.RoutingMode)
		vm.FixedDeployment = strings.TrimSpace(vm.FixedDeployment)
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
		if vm.Enabled && vm.RoutingMode == fallback.RoutingModeFixed {
			if vm.FixedDeployment == "" {
				return nil, nil, fmt.Errorf("fixed virtual model %s needs a fixed deployment", vm.Name)
			}
			fixedInOrder := false
			for _, id := range vm.FallbackOrder {
				if id == vm.FixedDeployment {
					fixedInOrder = true
					break
				}
			}
			if !fixedInOrder {
				return nil, nil, fmt.Errorf("fixed deployment %s must be bound to virtual model %s", vm.FixedDeployment, vm.Name)
			}
			if !deploymentEnabled[vm.FixedDeployment] {
				return nil, nil, fmt.Errorf("fixed deployment %s for virtual model %s must be enabled", vm.FixedDeployment, vm.Name)
			}
		}
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
			Enabled:         vm.Enabled,
			Description:     vm.Description,
			RoutingMode:     fallback.NormalizeRoutingMode(vm.RoutingMode),
			FixedDeployment: vm.FixedDeployment,
			FallbackOrder:   append([]string{}, vm.FallbackOrder...),
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
