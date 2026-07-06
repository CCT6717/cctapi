package router

import (
	"fmt"
	"strings"

	"github.com/songquanpeng/one-api/fallback"
)

// legacyGatewayFields lists JSON keys that belong to the legacy (v1) config
// format and must be rejected by the v2 endpoint.
var legacyGatewayFields = []string{"fixed_deployment"}

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

func sanitizedGatewayProviderKeys(keys []string) []string {
	freshKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" || strings.Contains(k, "*") {
			continue
		}
		freshKeys = append(freshKeys, k)
	}
	return freshKeys
}

func validateGatewayFreeProviders(current map[string]fallback.FreeProviderConfig, payload map[string]gatewayV2FreeProviderInput) error {
	for name, input := range payload {
		if err := fallback.ValidateFreeProviderName(name); err != nil {
			return err
		}
		replacementKeys := sanitizedGatewayProviderKeys(input.Keys)
		if input.ClearKeys && len(replacementKeys) > 0 {
			return fmt.Errorf("free_provider %q clear_keys cannot be combined with keys", name)
		}
		if input.LimitsOverride != nil {
			if err := fallback.ValidateFreeProviderLimits(toFreeProviderLimits(input.LimitsOverride)); err != nil {
				return fmt.Errorf("free_provider %q limits_override: %w", name, err)
			}
		}
		meta := fallback.BuiltinFreeProviders[name]
		existingKeys := 0
		if current != nil {
			existingKeys = len(current[name].Keys)
		}
		keyCountAfterSave := existingKeys
		if input.ClearKeys {
			keyCountAfterSave = 0
		} else if len(replacementKeys) > 0 {
			keyCountAfterSave = len(replacementKeys)
		}
		if input.Enabled && meta.RequiresKey && !meta.Keyless && keyCountAfterSave == 0 {
			return fmt.Errorf("free_provider %q requires at least one key before it can be enabled", name)
		}
	}
	return nil
}

func mergeGatewayFreeProviderInput(existing fallback.FreeProviderConfig, input gatewayV2FreeProviderInput) fallback.FreeProviderConfig {
	keys := append([]string{}, existing.Keys...)
	if input.ClearKeys {
		keys = []string{}
	} else if freshKeys := sanitizedGatewayProviderKeys(input.Keys); len(freshKeys) > 0 {
		keys = freshKeys
	}

	models := append([]string{}, existing.Models...)
	if input.Models != nil {
		models = make([]string, 0, len(input.Models))
		for _, model := range input.Models {
			model = strings.TrimSpace(model)
			if model != "" {
				models = append(models, model)
			}
		}
	}

	var mergedLimits *fallback.FreeProviderLimits
	if input.LimitsOverride != nil {
		mergedLimits = toFreeProviderLimits(input.LimitsOverride)
	} else if existing.LimitsOverride != nil {
		mergedLimits = &fallback.FreeProviderLimits{
			RPMLimit: cloneInt(existing.LimitsOverride.RPMLimit),
			RPDLimit: cloneInt(existing.LimitsOverride.RPDLimit),
			TPMLimit: cloneInt(existing.LimitsOverride.TPMLimit),
			TPDLimit: cloneInt(existing.LimitsOverride.TPDLimit),
		}
	}

	return fallback.FreeProviderConfig{
		Enabled:        input.Enabled,
		Keys:           keys,
		Models:         models,
		DefaultRPM:     existing.DefaultRPM,
		DefaultRPD:     existing.DefaultRPD,
		DefaultTPM:     existing.DefaultTPM,
		DefaultTPD:     existing.DefaultTPD,
		LimitsOverride: mergedLimits,
	}
}

func cloneInt(src *int) *int {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}

func buildGatewayV2FreeProviders(freeProviders map[string]fallback.FreeProviderConfig) map[string]gatewayV2FreeProvider {
	fps := make(map[string]gatewayV2FreeProvider, len(freeProviders))
	for name, fp := range freeProviders {
		gfp := gatewayV2FreeProvider{
			Enabled:  fp.Enabled,
			KeyCount: len(fp.Keys),
			Models:   append([]string{}, fp.Models...),
		}
		if meta, ok := fallback.BuiltinFreeProviders[name]; ok {
			rpm, rpd, tpm, tpd := fallback.MergeFreeProviderDefaultLimits(meta, fp)
			gfp.ProviderID = meta.ProviderID
			gfp.ChannelType = meta.ChannelType
			gfp.DefaultBaseURL = meta.DefaultBaseURL
			gfp.DefaultModels = append([]string{}, meta.DefaultModels...)
			gfp.DefaultRPM = rpm
			gfp.DefaultRPD = rpd
			gfp.DefaultTPM = tpm
			gfp.DefaultTPD = tpd
			gfp.ContextLength = meta.ContextLength
			gfp.SupportsVision = meta.SupportsVision
			gfp.SupportsStream = meta.SupportsStream
			gfp.SupportsTools = meta.SupportsTools
			gfp.SupportsJSON = meta.SupportsJSON
			gfp.RequiresKey = meta.RequiresKey
			gfp.Keyless = meta.Keyless
			gfp.ModelFetchMode = meta.ModelFetchMode
			gfp.Quirks = cloneGatewayFreeProviderQuirks(meta.Quirks)
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
	return fps
}

func cloneGatewayFreeProviderQuirks(src *fallback.FreeProviderQuirks) *fallback.FreeProviderQuirks {
	if src == nil {
		return nil
	}
	dst := *src
	if src.ForceParallelToolCalls != nil {
		value := *src.ForceParallelToolCalls
		dst.ForceParallelToolCalls = &value
	}
	return &dst
}

// buildGatewayV2Config projects the full fallback.Config into the simplified v2 view.
func buildGatewayV2Config(cfg *fallback.Config) gatewayV2Config {
	vms := make(map[string]gatewayV2VirtualModel, len(cfg.VirtualModels))
	for name, vm := range cfg.VirtualModels {
		vms[name] = gatewayV2VirtualModel{
			Enabled:             vm.Enabled,
			Strategy:            vm.Strategy,
			Pools:               append([]string{}, vm.Pools...),
			RoutingMode:         vm.RoutingMode,
			PreferredDeployment: vm.PreferredDeployment,
			FallbackOrder:       append([]string{}, vm.FallbackOrder...),
			AllowDegradeToLow:   vm.AllowDegradeToLow,
			AllowDegradeToFree:  vm.AllowDegradeToFree,
		}
	}

	deps := make(map[string]gatewayV2Deployment, len(cfg.Deployments))
	for id, dep := range cfg.Deployments {
		deps[id] = gatewayV2Deployment{
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
	}

	return gatewayV2Config{
		Enabled:             cfg.Enabled,
		VirtualModels:       vms,
		Deployments:         deps,
		FreeProviders:       buildGatewayV2FreeProviders(cfg.FreeProviders),
		FreeProviderCatalog: fallback.BuildFreeProviderCatalog(cfg),
	}
}

// isManualDeployment returns true if a deployment ID/pool is NOT a free pool deployment.
func isManualDeployment(id string, pool string) bool {
	if pool == "free" {
		return false
	}
	if strings.HasPrefix(id, "free:") {
		return false
	}
	return true
}

// buildManualConfig projects the fallback.Config into a v2 view that excludes
// free pool deployments and the cct/free virtual model.
func buildManualConfig(cfg *fallback.Config) gatewayV2Config {
	vms := make(map[string]gatewayV2VirtualModel, len(cfg.VirtualModels))
	for name, vm := range cfg.VirtualModels {
		if name == "cct/free" {
			continue
		}
		vms[name] = gatewayV2VirtualModel{
			Enabled:             vm.Enabled,
			Strategy:            vm.Strategy,
			Pools:               append([]string{}, vm.Pools...),
			RoutingMode:         vm.RoutingMode,
			PreferredDeployment: vm.PreferredDeployment,
			FallbackOrder:       append([]string{}, vm.FallbackOrder...),
			AllowDegradeToLow:   vm.AllowDegradeToLow,
			AllowDegradeToFree:  vm.AllowDegradeToFree,
		}
	}

	deps := make(map[string]gatewayV2Deployment, len(cfg.Deployments))
	for id, dep := range cfg.Deployments {
		if strings.HasPrefix(id, "---") {
			continue
		}
		if !isManualDeployment(id, dep.Pool) {
			continue
		}
		deps[id] = gatewayV2Deployment{
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
	}

	return gatewayV2Config{
		Enabled:             cfg.Enabled,
		VirtualModels:       vms,
		Deployments:         deps,
		FreeProviders:       buildGatewayV2FreeProviders(cfg.FreeProviders),
		FreeProviderCatalog: fallback.BuildFreeProviderCatalog(cfg),
	}
}
