package fallback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

func SyncFreePool(cfg *Config) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// 1. Scan existing auto channels
	// Note: escape [ and ] in LIKE — SQL treats [] as character class wildcard
	var existing []*model.Channel
	if err := model.DB.Where("name LIKE ?", autoChannelPrefix+"%").Find(&existing).Error; err != nil {
		return fmt.Errorf("failed to query auto channels: %w", err)
	}
	existingByName := map[string]*model.Channel{}
	for _, ch := range existing {
		existingByName[ch.Name] = ch
	}

	// 2. Compute desired channels and collect deployments
	type desired struct {
		name     string
		ch       model.Channel
		provider string
		keyHash  string
	}
	var desiredChannels []desired
	// Preserve existing non-auto deployments (collect them first)
	autoDeployments := map[string]DeploymentConfig{}

	for providerName, fp := range cfg.FreeProviders {
		meta, ok := BuiltinFreeProviders[providerName]
		if !ok {
			logger.SysWarn(fmt.Sprintf("[free_pool] unknown provider %q, skipping", providerName))
			continue
		}
		if !fp.Enabled {
			logger.SysLog(fmt.Sprintf("[free_pool] provider %q disabled, skipping", providerName))
			continue
		}
		logger.SysLog(fmt.Sprintf("[free_pool] processing provider %q (enabled=%v, keys=%d)", providerName, fp.Enabled, len(fp.Keys)))

		models := fp.Models
		if len(models) == 0 {
			models = meta.DefaultModels
		}
		if len(models) == 0 {
			// 需要动态拉取的供应商，用占位模型，后续 syncAllProviderModels 会更新
			models = []string{providerName + "/free"}
			logger.SysLog(fmt.Sprintf("[free_pool] provider %q has no models, using placeholder: %v", providerName, models))
		}
		realModel := models[0]

		// keyless 供应商:用空 key 创建单个 channel
		if len(fp.Keys) == 0 {
			if !meta.Keyless {
				logger.SysWarn(fmt.Sprintf("[free_pool] provider %q requires at least one key, skipping", providerName))
				continue
			}
			keyHash := SafeKeyHash("")
			name := channelName(providerName, keyHash)
			now := helper.GetTimestamp()
			weight := uint(0)
			baseURL := meta.DefaultBaseURL
			ch := model.Channel{
				Name:        name,
				Type:        meta.ChannelType,
				Key:         "",
				BaseURL:     &baseURL,
				Models:      strings.Join(models, ","),
				Status:      model.ChannelStatusEnabled,
				Weight:      &weight,
				CreatedTime: now,
			}
			desiredChannels = append(desiredChannels, desired{name, ch, providerName, keyHash})

			// Build deployment
			rpm := fp.DefaultRPM
			if rpm <= 0 {
				rpm = meta.DefaultRPM
			}
			rpd := fp.DefaultRPD
			if rpd <= 0 {
				rpd = meta.DefaultRPD
			}
			tpm := fp.DefaultTPM
			if tpm <= 0 {
				tpm = meta.DefaultTPM
			}
			tpd := fp.DefaultTPD
			if tpd <= 0 {
				tpd = meta.DefaultTPD
			}
			rpm, rpd, tpm, tpd = ApplyLimitsOverride(rpm, rpd, tpm, tpd, fp.LimitsOverride)

			depID := deploymentID(providerName, keyHash)
			autoDeployments[depID] = DeploymentConfig{
				ID:                    depID,
				Enabled:               true,
				ChannelID:             0,
				RealModel:             realModel,
				Pool:                  "free",
				QualityTier:           "medium",
				CostTier:              "free",
				SupportsVision:        meta.SupportsVision,
				SupportsStream:        meta.SupportsStream,
				SupportsTools:         meta.SupportsTools,
				SupportsJSON:          meta.SupportsJSON,
				ContextLength:         meta.ContextLength,
				Priority:              10,
				Weight:                100,
				MaxConcurrentRequests: 5,
				QuotaMode:             "free",
				SoftLimitRatio:        0.95,
				HardLimitRatio:        1.0,
				RPMLimit:              rpm,
				RPDLimit:              rpd,
				TPMLimit:              tpm,
				TPDLimit:              tpd,
			}
			continue
		}

		// 有 key 的供应商:遍历每个 key
		for _, key := range fp.Keys {
			if strings.TrimSpace(key) == "" {
				continue
			}
			keyHash := SafeKeyHash(key)
			name := channelName(providerName, keyHash)
			now := helper.GetTimestamp()
			weight := uint(0)
			baseURL := meta.DefaultBaseURL
			ch := model.Channel{
				Name:        name,
				Type:        meta.ChannelType,
				Key:         strings.TrimSpace(key),
				BaseURL:     &baseURL,
				Models:      strings.Join(models, ","),
				Status:      model.ChannelStatusEnabled,
				Weight:      &weight,
				CreatedTime: now,
			}
			desiredChannels = append(desiredChannels, desired{name, ch, providerName, keyHash})

			// Build deployment
			rpm := fp.DefaultRPM
			if rpm <= 0 {
				rpm = meta.DefaultRPM
			}
			rpd := fp.DefaultRPD
			if rpd <= 0 {
				rpd = meta.DefaultRPD
			}
			tpm := fp.DefaultTPM
			if tpm <= 0 {
				tpm = meta.DefaultTPM
			}
			tpd := fp.DefaultTPD
			if tpd <= 0 {
				tpd = meta.DefaultTPD
			}

			// Apply limits_override on top of merged defaults
			rpm, rpd, tpm, tpd = ApplyLimitsOverride(rpm, rpd, tpm, tpd, fp.LimitsOverride)

			depID := deploymentID(providerName, keyHash)
			autoDeployments[depID] = DeploymentConfig{
				ID:                    depID,
				Enabled:               true,
				ChannelID:             0, // filled after channel insert/update
				RealModel:             realModel,
				Pool:                  "free",
				QualityTier:           "medium",
				CostTier:              "free",
				SupportsVision:        meta.SupportsVision,
				SupportsStream:        meta.SupportsStream,
				SupportsTools:         meta.SupportsTools,
				SupportsJSON:          meta.SupportsJSON,
				ContextLength:         meta.ContextLength,
				Priority:              10,
				Weight:                100,
				MaxConcurrentRequests: 5,
				QuotaMode:             "free",
				SoftLimitRatio:        0.95,
				HardLimitRatio:        1.0,
				RPMLimit:              rpm,
				RPDLimit:              rpd,
				TPMLimit:              tpm,
				TPDLimit:              tpd,
			}
		}
	}

	// 3. Reconcile channels
	for _, d := range desiredChannels {
		existingCh, found := existingByName[d.name]
		if found {
			// Update if key changed
			if existingCh.Key != d.ch.Key {
				existingCh.Key = d.ch.Key
				existingCh.Models = d.ch.Models
				existingCh.Type = d.ch.Type
				if err := model.DB.Model(existingCh).Updates(map[string]interface{}{
					"key":    existingCh.Key,
					"models": existingCh.Models,
					"type":   existingCh.Type,
				}).Error; err != nil {
					logger.SysError(fmt.Sprintf("[free_pool] failed to update channel %s: %v", d.name, err))
					continue
				}
				_ = existingCh.UpdateAbilities()
				logger.SysWarn(fmt.Sprintf("[free_pool] auto channel %s (id=%d) key updated — config sync overwrote previous key", d.name, existingCh.Id))
			}
			// Ensure enabled
			if existingCh.Status != model.ChannelStatusEnabled {
				model.UpdateChannelStatusById(existingCh.Id, model.ChannelStatusEnabled)
			}
			// Fill ChannelID in deployment
			depID := deploymentID(d.provider, d.keyHash)
			if dep, ok := autoDeployments[depID]; ok {
				dep.ChannelID = existingCh.Id
				autoDeployments[depID] = dep
			}
		} else {
			// Create new channel
			if err := d.ch.Insert(); err != nil {
				logger.SysError(fmt.Sprintf("[free_pool] failed to insert channel %s: %v", d.name, err))
				continue
			}
			// Fill ChannelID in deployment
			depID := deploymentID(d.provider, d.keyHash)
			if dep, ok := autoDeployments[depID]; ok {
				dep.ChannelID = d.ch.Id
				autoDeployments[depID] = dep
			}
		}
	}

	// 4. Disable removed channels
	for _, existingCh := range existing {
		stillNeeded := false
		for _, d := range desiredChannels {
			if d.name == existingCh.Name {
				stillNeeded = true
				break
			}
		}
		if !stillNeeded && existingCh.Status == model.ChannelStatusEnabled {
			model.UpdateChannelStatusById(existingCh.Id, model.ChannelStatusManuallyDisabled)
			logger.SysLog(fmt.Sprintf("[free_pool] disabled removed auto channel %s (id=%d)", existingCh.Name, existingCh.Id))
		}
	}

	// 5. Write auto deployments into cfg.Deployments (merge, don't wipe).
	// Only write deployments whose ID matches IsAutoDeploymentID — user-created
	// deployments with a "free:*" prefix are preserved and never overwritten.
	if cfg.Deployments == nil {
		cfg.Deployments = map[string]DeploymentConfig{}
	}
	for id, dep := range autoDeployments {
		if dep.ChannelID <= 0 {
			logger.SysWarn(fmt.Sprintf("[free_pool] skipping deployment %s with no channel", id))
			continue
		}
		cfg.Deployments[id] = dep
	}

	// 6. Clean up stale auto deployments: remove from cfg.Deployments any auto
	// deployment that no longer has a corresponding enabled channel.
	for id := range cfg.Deployments {
		if !IsAutoDeploymentID(id) {
			continue // skip user-created deployments
		}
		if _, active := autoDeployments[id]; !active {
			// Stale: either the provider was disabled or the key was removed.
			// Remove it from the config so it won't be considered for routing.
			delete(cfg.Deployments, id)
			logger.SysLog(fmt.Sprintf("[free_pool] removed stale auto deployment %s (channel no longer active)", id))
		}
	}

	return nil
}

// StaleCleanupReport describes auto-generated resources that exist but no longer
// have a corresponding entry in the current free_providers config.
type StaleCleanupReport struct {
	StaleChannels    []StaleItem `json:"stale_channels"`
	StaleDeployments []StaleItem `json:"stale_deployments"`
	WillDelete       bool        `json:"will_delete"`
}

// StaleItem represents a single stale auto resource.
type StaleItem struct {
	Name   string `json:"name"`
	ID     int    `json:"id,omitempty"`
	Reason string `json:"reason"`
}

// computeExpectedAutoResources computes the set of expected auto channel names
// and deployment IDs from the current FreeProviders config. Used by both
// DryRunCleanStale and tests to determine which auto resources are still valid.
func computeExpectedAutoResources(cfg *Config) (expectedChannels map[string]bool, expectedDeployments map[string]bool) {
	expectedChannels = make(map[string]bool)
	expectedDeployments = make(map[string]bool)

	for providerName, fp := range cfg.FreeProviders {
		if !fp.Enabled {
			continue
		}
		meta, ok := BuiltinFreeProviders[providerName]
		if !ok {
			continue
		}
		if len(fp.Keys) == 0 {
			if !meta.Keyless {
				continue
			}
			keyHash := SafeKeyHash("")
			expectedChannels[channelName(providerName, keyHash)] = true
			expectedDeployments[deploymentID(providerName, keyHash)] = true
			continue
		}
		for _, key := range fp.Keys {
			if strings.TrimSpace(key) == "" {
				continue
			}
			keyHash := SafeKeyHash(key)
			expectedChannels[channelName(providerName, keyHash)] = true
			expectedDeployments[deploymentID(providerName, keyHash)] = true
		}
	}
	return expectedChannels, expectedDeployments
}

// DryRunCleanStale returns a report of stale auto resources (channels and
// deployments) that exist in the DB or config but no longer have a matching
// entry in the current free_providers config. Does NOT mutate any state.
// Report is non-nil even when an error is returned.
func DryRunCleanStale() (*StaleCleanupReport, error) {
	cfg := GetConfig()
	if cfg == nil {
		return &StaleCleanupReport{WillDelete: false}, fmt.Errorf("config is nil")
	}

	report := &StaleCleanupReport{WillDelete: false}
	expectedChannels, expectedDeployments := computeExpectedAutoResources(cfg)

	// Scan DB for auto channels that no longer have a matching config entry
	if model.DB == nil {
		return report, fmt.Errorf("database not initialized")
	}

	var dbChannels []*model.Channel
	if err := model.DB.Where("name LIKE ?", autoChannelPrefix+"%").Find(&dbChannels).Error; err != nil {
		return report, fmt.Errorf("failed to query auto channels: %w", err)
	}

	for _, ch := range dbChannels {
		if !expectedChannels[ch.Name] {
			report.StaleChannels = append(report.StaleChannels, StaleItem{
				Name:   ch.Name,
				ID:     ch.Id,
				Reason: "not found in current free_providers config",
			})
		}
	}

	// Scan config deployments for auto deployments that no longer have a matching key
	if cfg.Deployments != nil {
		for id, dep := range cfg.Deployments {
			if !IsAutoDeploymentID(id) {
				continue
			}
			if !expectedDeployments[id] {
				report.StaleDeployments = append(report.StaleDeployments, StaleItem{
					Name:   id,
					ID:     dep.ChannelID,
					Reason: "not found in current free_providers config",
				})
			}
		}
	}

	// Sort for deterministic output
	sort.Slice(report.StaleChannels, func(i, j int) bool {
		return report.StaleChannels[i].Name < report.StaleChannels[j].Name
	})
	sort.Slice(report.StaleDeployments, func(i, j int) bool {
		return report.StaleDeployments[i].Name < report.StaleDeployments[j].Name
	})

	return report, nil
}

// syncAllProviderModels 是通用的模型同步入口:遍历 cfg.FreeProviders 中
// 所有 Enabled 条目,对每个供应商的每个 key 调 fetchModels,
// 成功则更新对应 channel 的 Models 字段;
// 失败 log warn 保静态默认,不动 channel。
// 仅操作传入的 cfg(调用方负责锁),不在持有 configLock 时调 HTTP。
// keyless 供应商（如 kilo）用空 key,fetchModels 会处理。
func syncAllProviderModels(cfg *Config) {
	if cfg == nil || cfg.FreeProviders == nil {
		return
	}
	for providerName, fp := range cfg.FreeProviders {
		if !fp.Enabled {
			continue
		}
		// keyless 供应商:用空 key 调一次 fetchModels
		if len(fp.Keys) == 0 {
			models, err := fetchModels(providerName, "")
			if err != nil {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s model fetch failed: %v (keeping static default)", providerName, err))
				continue
			}
			if len(models) == 0 {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s returned no models (keeping static default)", providerName))
				continue
			}
			// keyless 供应商只有一个 channel,用空 key 的 hash
			keyHash := SafeKeyHash("")
			depID := deploymentID(providerName, keyHash)
			if UpdateDeploymentRealModel(depID, models[0]) {
				logger.SysLog(fmt.Sprintf("[free-pool] %s %s real_model synced to %s", providerName, depID, models[0]))
			}
			name := channelName(providerName, keyHash)
			var ch model.Channel
			if err := model.DB.Where("name = ?", name).First(&ch).Error; err != nil {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s channel %s not found in DB: %v", providerName, name, err))
				continue
			}
			newModels := strings.Join(models, ",")
			if ch.Models == newModels {
				continue // 无变化,跳过
			}
			if err := model.DB.Model(&ch).Update("models", newModels).Error; err != nil {
				logger.SysError(fmt.Sprintf("[free-pool] failed to update models for %s: %v", name, err))
				continue
			}
			ch.Models = newModels
			_ = ch.UpdateAbilities()
			logger.SysLog(fmt.Sprintf("[free-pool] %s %s models synced: %d models", providerName, name, len(models)))
			continue
		}
		// 有 key 的供应商:遍历每个 key
		for _, key := range fp.Keys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			keyHash := SafeKeyHash(key)
			depID := deploymentID(providerName, keyHash)
			models, err := fetchModels(providerName, key)
			if err != nil {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s model fetch failed for %s: %v (keeping static default)", providerName, depID, err))
				continue
			}
			if len(models) == 0 {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s returned no models for %s (keeping static default)", providerName, depID))
				continue
			}
			if UpdateDeploymentRealModel(depID, models[0]) {
				logger.SysLog(fmt.Sprintf("[free-pool] %s %s real_model synced to %s", providerName, depID, models[0]))
			}
			// Update channel.Models so the admin UI and channel abilities see the
			// same dynamic model list that routing now sees via RealModel.
			name := channelName(providerName, keyHash)
			var ch model.Channel
			if err := model.DB.Where("name = ?", name).First(&ch).Error; err != nil {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s channel %s not found in DB: %v", providerName, name, err))
				continue
			}
			newModels := strings.Join(models, ",")
			if ch.Models == newModels {
				continue // 无变化,跳过
			}
			if err := model.DB.Model(&ch).Update("models", newModels).Error; err != nil {
				logger.SysError(fmt.Sprintf("[free-pool] failed to update models for %s: %v", name, err))
				continue
			}
			ch.Models = newModels
			_ = ch.UpdateAbilities()
			logger.SysLog(fmt.Sprintf("[free-pool] %s %s models synced: %d models", providerName, name, len(models)))
		}
	}
}
