package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
)

// ── v2 gateway config response types ────────────────────────────────────────

type gatewayV2Config struct {
	Enabled       bool                             `json:"enabled"`
	VirtualModels map[string]gatewayV2VirtualModel `json:"virtual_models"`
	Deployments   map[string]gatewayV2Deployment   `json:"deployments"`
	FreeProviders map[string]gatewayV2FreeProvider `json:"free_providers"`
}

type gatewayV2VirtualModel struct {
	Enabled            bool     `json:"enabled"`
	Strategy           string   `json:"strategy"`
	Pools              []string `json:"pools"`
	AllowDegradeToLow  bool     `json:"allow_degrade_to_low"`
	AllowDegradeToFree bool     `json:"allow_degrade_to_free"`
}

type gatewayV2Deployment struct {
	Enabled        bool   `json:"enabled"`
	ChannelID      int    `json:"channel_id"`
	RealModel      string `json:"real_model"`
	Pool           string `json:"pool"`
	QualityTier    string `json:"quality_tier"`
	CostTier       string `json:"cost_tier"`
	QuotaMode      string `json:"quota_mode"`
	SupportsStream bool   `json:"supports_stream"`
	SupportsVision bool   `json:"supports_vision"`
	SupportsTools  bool   `json:"supports_tools"`
	SupportsJSON   bool   `json:"supports_json"`
	ContextLength  int    `json:"context_length"`
	RPMLimit       int    `json:"rpm_limit"`
	RPDLimit       int    `json:"rpd_limit"`
	TPMLimit       int    `json:"tpm_limit"`
	TPDLimit       int    `json:"tpd_limit"`
	Priority         int     `json:"priority"`
	Weight           int     `json:"weight"`
	DailyLimitTokens int64   `json:"daily_limit_tokens"`
	SoftLimitRatio   float64 `json:"soft_limit_ratio"`
	HardLimitRatio   float64 `json:"hard_limit_ratio"`
}

type gatewayV2FreeProvider struct {
	Enabled        bool                     `json:"enabled"`
	KeyCount       int                      `json:"key_count"`
	LimitsOverride *gatewayV2LimitsOverride `json:"limits_override,omitempty"`
}

type gatewayV2LimitsOverride struct {
	RPMLimit *int `json:"rpm_limit,omitempty"`
	RPDLimit *int `json:"rpd_limit,omitempty"`
	TPMLimit *int `json:"tpm_limit,omitempty"`
	TPDLimit *int `json:"tpd_limit,omitempty"`
}

// ── v2 gateway config request types (PUT) ───────────────────────────────────

type gatewayV2ConfigInput struct {
	Enabled       bool                                  `json:"enabled"`
	VirtualModels map[string]gatewayV2VirtualModel      `json:"virtual_models"`
	Deployments   map[string]gatewayV2Deployment        `json:"deployments"`
	FreeProviders map[string]gatewayV2FreeProviderInput `json:"free_providers"`
}

type gatewayV2FreeProviderInput struct {
	Enabled        bool                     `json:"enabled"`
	Keys           []string                 `json:"keys,omitempty"`
	LimitsOverride *gatewayV2LimitsOverride `json:"limits_override,omitempty"`
}

// legacyGatewayFields lists JSON keys that belong to the legacy (v1) config
// format and must be rejected by the v2 endpoint.
var legacyGatewayFields = []string{"routing_mode", "fallback_order", "fixed_deployment"}

// containsLegacyFields recursively inspects a decoded JSON value and returns
// true if any object at any depth contains a legacy v1 key.
func containsLegacyFields(v interface{}) bool {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, field := range legacyGatewayFields {
			if _, ok := val[field]; ok {
				return true
			}
		}
		for _, child := range val {
			if containsLegacyFields(child) {
				return true
			}
		}
	case []interface{}:
		for _, item := range val {
			if containsLegacyFields(item) {
				return true
			}
		}
	}
	return false
}

// toFreeProviderLimits converts the v2 override struct to the fallback package type.
func toFreeProviderLimits(v *gatewayV2LimitsOverride) *fallback.FreeProviderLimits {
	if v == nil {
		return nil
	}
	return &fallback.FreeProviderLimits{
		RPMLimit: v.RPMLimit,
		RPDLimit: v.RPDLimit,
		TPMLimit: v.TPMLimit,
		TPDLimit: v.TPDLimit,
	}
}

// buildGatewayV2Config projects the full fallback.Config into the simplified v2 view.
func buildGatewayV2Config(cfg *fallback.Config) gatewayV2Config {
	vms := make(map[string]gatewayV2VirtualModel, len(cfg.VirtualModels))
	for name, vm := range cfg.VirtualModels {
		vms[name] = gatewayV2VirtualModel{
			Enabled:            vm.Enabled,
			Strategy:           vm.Strategy,
			Pools:              append([]string{}, vm.Pools...),
			AllowDegradeToLow:  vm.AllowDegradeToLow,
			AllowDegradeToFree: vm.AllowDegradeToFree,
		}
	}

	deps := make(map[string]gatewayV2Deployment, len(cfg.Deployments))
	for id, dep := range cfg.Deployments {
		deps[id] = gatewayV2Deployment{
			Enabled:         dep.Enabled,
			ChannelID:       dep.ChannelID,
			RealModel:       dep.RealModel,
			Pool:            dep.Pool,
			QualityTier:     dep.QualityTier,
			CostTier:        dep.CostTier,
			QuotaMode:       dep.QuotaMode,
			SupportsStream:  dep.SupportsStream,
			SupportsVision:  dep.SupportsVision,
			SupportsTools:   dep.SupportsTools,
			SupportsJSON:    dep.SupportsJSON,
			ContextLength:   dep.ContextLength,
			RPMLimit:        dep.RPMLimit,
			RPDLimit:        dep.RPDLimit,
			TPMLimit:        dep.TPMLimit,
			TPDLimit:        dep.TPDLimit,
			Priority:        dep.Priority,
			Weight:          dep.Weight,
			DailyLimitTokens: dep.DailyLimitTokens,
			SoftLimitRatio:   dep.SoftLimitRatio,
			HardLimitRatio:   dep.HardLimitRatio,
		}
	}

	fps := make(map[string]gatewayV2FreeProvider, len(cfg.FreeProviders))
	for name, fp := range cfg.FreeProviders {
		gfp := gatewayV2FreeProvider{
			Enabled:  fp.Enabled,
			KeyCount: len(fp.Keys),
		}
		if fp.LimitsOverride != nil {
			gfp.LimitsOverride = &gatewayV2LimitsOverride{
				RPMLimit: fp.LimitsOverride.RPMLimit,
				RPDLimit: fp.LimitsOverride.RPDLimit,
				TPMLimit: fp.LimitsOverride.TPMLimit,
				TPDLimit: fp.LimitsOverride.TPDLimit,
			}
		}
		fps[name] = gfp
	}

	return gatewayV2Config{
		Enabled:       cfg.Enabled,
		VirtualModels: vms,
		Deployments:   deps,
		FreeProviders: fps,
	}
}

// getGatewayConfig handles GET /api/fallback/gateway/config.
func getGatewayConfig(c *gin.Context) {
	cfg := fallback.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback config is not loaded"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": buildGatewayV2Config(cfg)})
}

// updateGatewayConfig handles PUT /api/fallback/gateway/config.
func updateGatewayConfig(c *gin.Context) {
	// Step 1: read raw body.
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Step 1a: reject legacy v1 fields anywhere in the payload.
	var rawCheck interface{}
	if err := json.Unmarshal(rawBody, &rawCheck); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if containsLegacyFields(rawCheck) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "legacy field detected in v2 gateway config",
		})
		return
	}

	// Step 2: parse into typed struct.
	var payload gatewayV2ConfigInput
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Step 3: validate free_provider limits_override.
	for name, fp := range payload.FreeProviders {
		if fp.LimitsOverride != nil {
			if err := fallback.ValidateFreeProviderLimits(toFreeProviderLimits(fp.LimitsOverride)); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": fmt.Sprintf("free_provider %q limits_override: %v", name, err),
				})
				return
			}
		}
	}

	// Step 4: load current config and merge.
	current := fallback.GetConfig()
	if current == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "fallback config is not loaded"})
		return
	}

	// Start from a copy of the current config so we preserve alert, smart_sort,
	// blocked_error_codes, and any other fields not managed by the v2 API.
	merged := *current
	merged.Enabled = payload.Enabled

	// Virtual models: replace with payload (normalise strategy and pools).
	merged.VirtualModels = make(map[string]fallback.VirtualModelConfig, len(payload.VirtualModels))
	for name, vm := range payload.VirtualModels {
		pools := vm.Pools
		if len(pools) == 0 {
			pools = []string{"default"}
		}
		merged.VirtualModels[name] = fallback.VirtualModelConfig{
			Enabled:            vm.Enabled,
			Strategy:           fallback.NormalizeStrategy(vm.Strategy),
			Pools:              append([]string{}, pools...),
			AllowDegradeToLow:  vm.AllowDegradeToLow,
			AllowDegradeToFree: vm.AllowDegradeToFree,
		}
	}

	// Deployments: replace with payload but preserve the hidden field
	// (max_concurrent_requests) from the existing deployment when it already
	// exists. The remaining fields are taken from the payload as-is.
	merged.Deployments = make(map[string]fallback.DeploymentConfig, len(payload.Deployments))
	for id, dep := range payload.Deployments {
		mergedDep := fallback.DeploymentConfig{
			Enabled:          dep.Enabled,
			ChannelID:        dep.ChannelID,
			RealModel:        dep.RealModel,
			Pool:             dep.Pool,
			QualityTier:      dep.QualityTier,
			CostTier:         dep.CostTier,
			QuotaMode:        dep.QuotaMode,
			SupportsStream:   dep.SupportsStream,
			SupportsVision:   dep.SupportsVision,
			SupportsTools:    dep.SupportsTools,
			SupportsJSON:     dep.SupportsJSON,
			ContextLength:    dep.ContextLength,
			RPMLimit:         dep.RPMLimit,
			RPDLimit:         dep.RPDLimit,
			TPMLimit:         dep.TPMLimit,
			TPDLimit:         dep.TPDLimit,
			Priority:         dep.Priority,
			Weight:           dep.Weight,
			DailyLimitTokens: dep.DailyLimitTokens,
			SoftLimitRatio:   dep.SoftLimitRatio,
			HardLimitRatio:   dep.HardLimitRatio,
		}
		// Carry over hidden field (max_concurrent_requests) from the existing
		// deployment if present.
		if existingDep, ok := current.Deployments[id]; ok {
			mergedDep.MaxConcurrentRequests = existingDep.MaxConcurrentRequests
		}
		// Apply sane defaults for required numeric fields.
		if mergedDep.Weight <= 0 {
			mergedDep.Weight = 100
		}
		if mergedDep.SoftLimitRatio <= 0 {
			mergedDep.SoftLimitRatio = 0.95
		}
		if mergedDep.HardLimitRatio <= 0 {
			mergedDep.HardLimitRatio = 1.0
		}
		merged.Deployments[id] = mergedDep
	}

	// Free providers: merge keys carefully — never overwrite real keys with
	// masked or empty values.
	if merged.FreeProviders == nil {
		merged.FreeProviders = make(map[string]fallback.FreeProviderConfig)
	}
	for name, fpInput := range payload.FreeProviders {
		existing := merged.FreeProviders[name]
		keys := existing.Keys // default: keep existing keys

		if len(fpInput.Keys) > 0 {
			// Filter out empty and masked entries.
			freshKeys := make([]string, 0, len(fpInput.Keys))
			for _, k := range fpInput.Keys {
				k = strings.TrimSpace(k)
				if k == "" || strings.Contains(k, "*") {
					continue // empty or masked — skip
				}
				freshKeys = append(freshKeys, k)
			}
			if len(freshKeys) > 0 {
				keys = freshKeys // only replace when at least one real key provided
			}
			// Otherwise all keys were masked/empty — keep existing.
		}

		merged.FreeProviders[name] = fallback.FreeProviderConfig{
			Enabled:        fpInput.Enabled,
			Keys:           keys,
			LimitsOverride: toFreeProviderLimits(fpInput.LimitsOverride),
		}
	}

	// Step 5: serialise, backup, write, reload.
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	data = append(data, '\n')

	backupPath, err := backupFallbackEditorConfig(fallbackEditorConfigPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := os.WriteFile(fallbackEditorConfigPath, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := fallback.ReloadConfig(fallbackEditorConfigPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Step 6: return fresh config (same shape as GET).
	freshCfg := fallback.GetConfig()
	response := gin.H{
		"success": true,
		"message": "gateway config saved",
		"data":    buildGatewayV2Config(freshCfg),
	}
	if backupPath != "" {
		response["backup_path"] = backupPath
	}
	c.JSON(http.StatusOK, response)
}
