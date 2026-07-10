package fallback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

type desiredFreeProviderResource struct {
	name     string
	ch       model.Channel
	provider string
	keyHash  string
}

func buildDesiredFreeProviderResources(cfg *Config) ([]desiredFreeProviderResource, map[string]DeploymentConfig) {
	desiredChannels := []desiredFreeProviderResource{}
	autoDeployments := map[string]DeploymentConfig{}
	if cfg == nil {
		return desiredChannels, autoDeployments
	}

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
			models = []string{providerName + "/free"}
			logger.SysLog(fmt.Sprintf("[free_pool] provider %q has no models, using placeholder: %v", providerName, models))
		}
		realModel := models[0]

		addResource := func(key string) {
			key = strings.TrimSpace(key)
			keyHash := SafeKeyHash(key)
			name := channelName(providerName, keyHash)
			weight := uint(0)
			baseURL := meta.DefaultBaseURL
			ch := model.Channel{
				Name:    name,
				Type:    meta.ChannelType,
				Key:     key,
				BaseURL: &baseURL,
				Models:  strings.Join(models, ","),
				Status:  model.ChannelStatusEnabled,
				Weight:  &weight,
			}
			desiredChannels = append(desiredChannels, desiredFreeProviderResource{name: name, ch: ch, provider: providerName, keyHash: keyHash})

			rpm, rpd, tpm, tpd := ResolveFreeProviderLimits(meta, fp)
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
		}

		if len(fp.Keys) == 0 {
			if !meta.Keyless {
				logger.SysWarn(fmt.Sprintf("[free_pool] provider %q requires at least one key, skipping", providerName))
				continue
			}
			addResource("")
			continue
		}

		for _, key := range fp.Keys {
			if strings.TrimSpace(key) == "" {
				continue
			}
			addResource(key)
		}
	}

	return desiredChannels, autoDeployments
}

func SyncFreePool(cfg *Config) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// 1. Scan existing auto channels
	// Note: escape [ and ] in LIKE; SQL treats [] as character class wildcard.
	var existing []*model.Channel
	if err := model.DB.Where("name LIKE ?", autoChannelPrefix+"%").Find(&existing).Error; err != nil {
		return fmt.Errorf("failed to query auto channels: %w", err)
	}
	existingByName := map[string]*model.Channel{}
	existingAutoChannelIDs := map[int]bool{}
	for _, ch := range existing {
		existingByName[ch.Name] = ch
		existingAutoChannelIDs[ch.Id] = true
	}

	// 2. Compute desired channels and collect deployments.
	desiredChannels, autoDeployments := buildDesiredFreeProviderResources(cfg)

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
				logger.SysWarn(fmt.Sprintf("[free_pool] auto channel %s (id=%d) key updated - config sync overwrote previous key", d.name, existingCh.Id))
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
			d.ch.CreatedTime = helper.GetTimestamp()
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
	// Only write deployments whose ID matches IsAutoDeploymentID; user-created
	// deployments with a "free:*" prefix are preserved and never overwritten.
	if cfg.Deployments == nil {
		cfg.Deployments = map[string]DeploymentConfig{}
	}
	for id, dep := range autoDeployments {
		if dep.ChannelID <= 0 {
			logger.SysWarn(fmt.Sprintf("[free_pool] skipping deployment %s with no channel", id))
			continue
		}
		if existing, ok := cfg.Deployments[id]; ok {
			providerName := providerNameForAutoDeploymentID(id)
			if !providerConfigOwnsAutoRealModel(cfg, providerName) {
				dep = preserveDeploymentRealModelOverride(existing, dep, id, providerName)
			}
		}
		cfg.Deployments[id] = dep
	}

	// 6. Clean up stale auto deployments: remove from cfg.Deployments any auto
	// deployment that no longer has a corresponding enabled channel.
	for id, dep := range cfg.Deployments {
		if !IsAutoDeploymentID(id) {
			continue // skip user-created deployments
		}
		if !deploymentUsesAutoChannel(dep, existingAutoChannelIDs) {
			continue
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

func RefreshFreePoolRuntimeState() {
	cfg := GetConfig()
	syncAllProviderModels(cfg)
	syncOpenRouterCredits(cfg)
}

func deploymentUsesAutoChannel(dep DeploymentConfig, autoChannelIDs map[int]bool) bool {
	if dep.ChannelID <= 0 {
		return false
	}
	return autoChannelIDs[dep.ChannelID]
}

func preserveDeploymentRealModelOverride(existing DeploymentConfig, generated DeploymentConfig, deploymentID string, providerName string) DeploymentConfig {
	if isDeploymentRealModelOverride(existing.RealModel, generated.RealModel, providerName) {
		generated.RealModel = strings.TrimSpace(existing.RealModel)
		logger.SysLog(fmt.Sprintf("[free_pool] preserved real_model override for %s: %s", deploymentID, generated.RealModel))
	}
	return generated
}

func routingModelForFetchedModels(providerName string, fetchedModels []string) string {
	meta, ok := BuiltinFreeProviders[providerName]
	if ok && meta.ModelFetchMode == ModelFetchOpenRouterFree && len(meta.DefaultModels) > 0 {
		return meta.DefaultModels[0]
	}
	if len(fetchedModels) == 0 {
		return ""
	}
	return fetchedModels[0]
}

func providerConfigOwnsAutoRealModel(cfg *Config, providerName string) bool {
	if cfg == nil {
		return false
	}
	fp, ok := cfg.FreeProviders[providerName]
	if !ok {
		return false
	}
	if len(fp.Models) > 0 {
		return true
	}
	meta, ok := BuiltinFreeProviders[providerName]
	return ok && meta.ModelFetchMode == ModelFetchOpenRouterFree
}

func isDeploymentRealModelOverride(currentRealModel string, generatedRealModel string, providerName string) bool {
	currentRealModel = strings.TrimSpace(currentRealModel)
	generatedRealModel = strings.TrimSpace(generatedRealModel)
	if currentRealModel == "" {
		return false
	}
	if currentRealModel == generatedRealModel {
		return false
	}
	if providerName != "" && currentRealModel == providerName+"/free" {
		return false
	}
	return true
}

func shouldSyncDeploymentRealModel(deploymentID string, generatedRealModel string, providerName string) bool {
	dep, ok := CloneDeployment(deploymentID)
	if !ok {
		return true
	}
	return !isDeploymentRealModelOverride(dep.RealModel, generatedRealModel, providerName)
}

func providerNameForAutoDeploymentID(id string) string {
	providerName, _ := FreeProviderNameFromDeploymentID(id)
	return providerName
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

	desiredChannels, deployments := buildDesiredFreeProviderResources(cfg)
	for _, resource := range desiredChannels {
		expectedChannels[resource.name] = true
	}
	for deploymentID := range deployments {
		expectedDeployments[deploymentID] = true
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
	autoChannelIDs := make(map[int]bool, len(dbChannels))
	for _, ch := range dbChannels {
		autoChannelIDs[ch.Id] = true
	}

	// Scan config deployments for auto deployments that no longer have a matching key
	if cfg.Deployments != nil {
		for id, dep := range cfg.Deployments {
			if !IsAutoDeploymentID(id) {
				continue
			}
			if !deploymentUsesAutoChannel(dep, autoChannelIDs) {
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

// syncAllProviderModels refreshes dynamic model lists for enabled providers.
// It only uses the passed config snapshot and applies live model updates via
// the dedicated update helpers.
func syncAllProviderModels(cfg *Config) {
	if cfg == nil || cfg.FreeProviders == nil {
		return
	}
	for providerName, fp := range cfg.FreeProviders {
		if !fp.Enabled {
			continue
		}
		if len(fp.Models) > 0 {
			logger.SysLog(fmt.Sprintf("[free-pool] %s has configured models, skipping dynamic model sync", providerName))
			continue
		}
		// keyless 渚涘簲鍟?鐢ㄧ┖ key 璋冧竴娆?fetchModels
		if len(fp.Keys) == 0 {
			keyHash := SafeKeyHash("")
			depID := deploymentID(providerName, keyHash)
			models, err := fetchModels(providerName, "")
			if err != nil {
				recordModelSyncFailure(depID, providerName, err)
				logger.SysWarn(fmt.Sprintf("[free-pool] %s model fetch failed: %v (keeping static default)", providerName, err))
				continue
			}
			if len(models) == 0 {
				recordModelSyncFailure(depID, providerName, fmt.Errorf("returned no models"))
				logger.SysWarn(fmt.Sprintf("[free-pool] %s returned no models (keeping static default)", providerName))
				continue
			}
			recordModelSyncSuccess(depID)
			// keyless 渚涘簲鍟嗗彧鏈変竴涓?channel,鐢ㄧ┖ key 鐨?hash
			routingModel := routingModelForFetchedModels(providerName, models)
			if shouldSyncDeploymentRealModel(depID, routingModel, providerName) && UpdateDeploymentRealModel(depID, routingModel) {
				logger.SysLog(fmt.Sprintf("[free-pool] %s %s real_model synced to %s", providerName, depID, routingModel))
			}
			name := channelName(providerName, keyHash)
			var ch model.Channel
			if err := model.DB.Where("name = ?", name).First(&ch).Error; err != nil {
				logger.SysWarn(fmt.Sprintf("[free-pool] %s channel %s not found in DB: %v", providerName, name, err))
				continue
			}
			newModels := strings.Join(models, ",")
			if ch.Models == newModels {
				continue // 鏃犲彉鍖?璺宠繃
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
		// 鏈?key 鐨勪緵搴斿晢:閬嶅巻姣忎釜 key
		for _, key := range fp.Keys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			keyHash := SafeKeyHash(key)
			depID := deploymentID(providerName, keyHash)
			models, err := fetchModels(providerName, key)
			if err != nil {
				recordModelSyncFailure(depID, providerName, err)
				logger.SysWarn(fmt.Sprintf("[free-pool] %s model fetch failed for %s: %v (keeping static default)", providerName, depID, err))
				continue
			}
			if len(models) == 0 {
				recordModelSyncFailure(depID, providerName, fmt.Errorf("returned no models"))
				logger.SysWarn(fmt.Sprintf("[free-pool] %s returned no models for %s (keeping static default)", providerName, depID))
				continue
			}
			recordModelSyncSuccess(depID)
			routingModel := routingModelForFetchedModels(providerName, models)
			if shouldSyncDeploymentRealModel(depID, routingModel, providerName) && UpdateDeploymentRealModel(depID, routingModel) {
				logger.SysLog(fmt.Sprintf("[free-pool] %s %s real_model synced to %s", providerName, depID, routingModel))
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
				continue // 鏃犲彉鍖?璺宠繃
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

func recordModelSyncFailure(deploymentID string, providerName string, err error) {
	if deploymentID == "" || err == nil {
		return
	}
	RecordFailure(deploymentID, fmt.Sprintf("model sync failed for %s: %v", providerName, err), false)
	setHealthStatus(deploymentID, HealthError)
}

func recordModelSyncSuccess(deploymentID string) {
	if deploymentID == "" {
		return
	}
	snap := SnapshotRuntimeState(deploymentID)
	if strings.Contains(snap.LastError, "model sync failed") {
		clearRuntimeError(deploymentID)
		if GetHealthStatus(deploymentID) == HealthError {
			setHealthStatus(deploymentID, HealthHealthy)
		}
	}
}
