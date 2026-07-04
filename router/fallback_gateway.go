package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
)

// getGatewayConfig handles GET /api/fallback/gateway/config.
func getGatewayConfig(c *gin.Context) {
	cfg := fallback.CloneConfig()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback config is not loaded"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": buildGatewayV2Config(cfg)})
}

// getManualConfig handles GET /api/fallback/manual-config.
// Returns gateway config excluding free pool deployments and cct/free virtual model.
func getManualConfig(c *gin.Context) {
	cfg := fallback.CloneConfig()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "fallback config is not loaded"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": buildManualConfig(cfg)})
}

// updateManualConfig handles PUT /api/fallback/manual-config.
// Updates only non-free pool virtual models and deployments, preserving free pool data.
func updateManualConfig(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

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

	var payload gatewayV2ConfigInput
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

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

	current := fallback.CloneConfig()
	if current == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "fallback config is not loaded"})
		return
	}

	merged := *current
	merged.Enabled = payload.Enabled

	// Deep-copy the three map fields so concurrent GetConfig() readers
	// never see a half-merged state. Without this, merged.VirtualModels etc.
	// share the live config's underlying map headers, and the delete/write
	// loops below mutate the live map mid-merge. Matches updateGatewayConfig
	// pattern (~L575).
	merged.VirtualModels = make(map[string]fallback.VirtualModelConfig, len(current.VirtualModels))
	for k, v := range current.VirtualModels {
		merged.VirtualModels[k] = v
	}
	merged.Deployments = make(map[string]fallback.DeploymentConfig, len(current.Deployments))
	for k, v := range current.Deployments {
		merged.Deployments[k] = v
	}
	merged.FreeProviders = make(map[string]fallback.FreeProviderConfig, len(current.FreeProviders))
	for k, v := range current.FreeProviders {
		merged.FreeProviders[k] = v
	}

	// Virtual models: merge non-free, preserve cct/free
	if merged.VirtualModels == nil {
		merged.VirtualModels = make(map[string]fallback.VirtualModelConfig)
	}
	for name, vm := range payload.VirtualModels {
		if name == "cct/free" {
			continue
		}
		pools := vm.Pools
		if len(pools) == 0 {
			pools = []string{"default"}
		}
		mergedVM := fallback.VirtualModelConfig{
			Enabled:             vm.Enabled,
			Strategy:            fallback.NormalizeStrategy(vm.Strategy),
			Pools:               append([]string{}, pools...),
			RoutingMode:         fallback.NormalizeRoutingMode(vm.RoutingMode),
			PreferredDeployment: vm.PreferredDeployment,
			AllowDegradeToLow:   vm.AllowDegradeToLow,
			AllowDegradeToFree:  vm.AllowDegradeToFree,
		}
		// Use FallbackOrder from payload if provided; otherwise preserve existing.
		if len(vm.FallbackOrder) > 0 {
			mergedVM.FallbackOrder = append([]string{}, vm.FallbackOrder...)
		} else if existing, ok := current.VirtualModels[name]; ok {
			mergedVM.FallbackOrder = append([]string{}, existing.FallbackOrder...)
		}
		if vm.PreferredDeployment != "" {
			dep, ok := current.Deployments[vm.PreferredDeployment]
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": fmt.Sprintf("preferred deployment %s for manual config %s does not exist", vm.PreferredDeployment, name),
				})
				return
			}
			if mergedVM.Enabled && !dep.Enabled {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": fmt.Sprintf("preferred deployment %s for manual config %s is disabled", vm.PreferredDeployment, name),
				})
				return
			}
			fallbackSet := make(map[string]bool)
			for _, id := range mergedVM.FallbackOrder {
				if !strings.HasPrefix(id, "---") {
					fallbackSet[id] = true
				}
			}
			for _, pool := range vm.Pools {
				for id, dep := range current.Deployments {
					if dep.Pool == pool && !strings.HasPrefix(id, "---") {
						fallbackSet[id] = true
					}
				}
			}
			if !fallbackSet[vm.PreferredDeployment] {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": fmt.Sprintf("preferred deployment %s for manual config %s is not in fallback order or pools", vm.PreferredDeployment, name),
				})
				return
			}
		}
		merged.VirtualModels[name] = mergedVM
	}

	// Remove virtual models that are missing from the payload (user deleted them).
	// Keep cct/free (preserved by frontend).
	vmInPayload := make(map[string]bool)
	for name := range payload.VirtualModels {
		vmInPayload[name] = true
	}
	for name := range merged.VirtualModels {
		if name == "cct/free" {
			continue
		}
		if !vmInPayload[name] {
			delete(merged.VirtualModels, name)
		}
	}

	// Deployments: merge non-free, preserve free pool deployments
	if merged.Deployments == nil {
		merged.Deployments = make(map[string]fallback.DeploymentConfig)
	}
	for id, dep := range payload.Deployments {
		if strings.HasPrefix(id, "---") {
			continue
		}
		if !isManualDeployment(id, dep.Pool) {
			continue
		}
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
		if existingDep, ok := current.Deployments[id]; ok {
			mergedDep.MaxConcurrentRequests = existingDep.MaxConcurrentRequests
		}
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

	// Remove manual deployments that are missing from the payload (user deleted them).
	// Keep free deployments (preserved by frontend) and separator keys.
	manualInPayload := make(map[string]bool)
	for id := range payload.Deployments {
		if !strings.HasPrefix(id, "---") {
			manualInPayload[id] = true
		}
	}
	for id, dep := range merged.Deployments {
		if strings.HasPrefix(id, "---") {
			continue
		}
		if !isManualDeployment(id, dep.Pool) {
			continue // free deployment, keep
		}
		if !manualInPayload[id] {
			delete(merged.Deployments, id)
		}
	}

	// Free providers: merge keys carefully
	if merged.FreeProviders == nil {
		merged.FreeProviders = make(map[string]fallback.FreeProviderConfig)
	}
	for name, fpInput := range payload.FreeProviders {
		existing := merged.FreeProviders[name]
		merged.FreeProviders[name] = mergeGatewayFreeProviderInput(existing, fpInput)
	}

	backupPath, err := saveGatewayConfigPayload(merged)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	freshCfg := fallback.CloneConfig()
	response := gin.H{
		"success": true,
		"message": "manual config saved",
		"data":    buildManualConfig(freshCfg),
	}
	if backupPath != "" {
		response["backup_path"] = backupPath
	}
	c.JSON(http.StatusOK, response)
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
	current := fallback.CloneConfig()
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
			Enabled:             vm.Enabled,
			Strategy:            fallback.NormalizeStrategy(vm.Strategy),
			Pools:               append([]string{}, pools...),
			RoutingMode:         fallback.NormalizeRoutingMode(vm.RoutingMode),
			PreferredDeployment: vm.PreferredDeployment,
			FallbackOrder:       append([]string{}, vm.FallbackOrder...),
			AllowDegradeToLow:   vm.AllowDegradeToLow,
			AllowDegradeToFree:  vm.AllowDegradeToFree,
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

	for name, vm := range merged.VirtualModels {
		if vm.PreferredDeployment == "" {
			continue
		}
		dep, ok := merged.Deployments[vm.PreferredDeployment]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("preferred deployment %s for gateway config %s does not exist", vm.PreferredDeployment, name),
			})
			return
		}
		if vm.Enabled && !dep.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("preferred deployment %s for gateway config %s is disabled", vm.PreferredDeployment, name),
			})
			return
		}
		fallbackSet := make(map[string]bool)
		for _, id := range vm.FallbackOrder {
			if !strings.HasPrefix(id, "---") {
				fallbackSet[id] = true
			}
		}
		for _, pool := range vm.Pools {
			for id, dep := range merged.Deployments {
				if dep.Pool == pool && !strings.HasPrefix(id, "---") {
					fallbackSet[id] = true
				}
			}
		}
		if !fallbackSet[vm.PreferredDeployment] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("preferred deployment %s for gateway config %s is not in fallback order or pools", vm.PreferredDeployment, name),
			})
			return
		}
	}

	// Free providers: merge keys carefully 鈥?never overwrite real keys with
	// masked or empty values.
	if merged.FreeProviders == nil {
		merged.FreeProviders = make(map[string]fallback.FreeProviderConfig)
	}
	for name, fpInput := range payload.FreeProviders {
		existing := merged.FreeProviders[name]
		merged.FreeProviders[name] = mergeGatewayFreeProviderInput(existing, fpInput)
	}

	// Step 5: serialise, backup, write, reload.
	backupPath, err := saveGatewayConfigPayload(merged)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Step 6: return fresh config (same shape as GET).
	freshCfg := fallback.CloneConfig()
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
