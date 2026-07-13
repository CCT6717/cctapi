package fallback

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
	"gorm.io/gorm"
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

		addResource := func(key string) {
			key = strings.TrimSpace(key)
			keyHash := SafeKeyHash(key)
			depID := deploymentID(providerName, keyHash)
			models := append([]string{}, fp.Models...)
			if len(models) == 0 {
				models = append([]string{}, meta.DefaultModels...)
			}
			var persistedSnapshot FreeProviderCatalogSnapshot
			if len(fp.Models) == 0 {
				if snapshot, ok := GetFreeProviderCatalogSnapshot(depID); ok && len(snapshot.Models) > 0 {
					persistedSnapshot = snapshot
					models = freeProviderCatalogModelIDs(snapshot.Models)
				}
			}
			if len(models) == 0 {
				models = []string{providerName + "/free"}
				logger.SysLog(fmt.Sprintf("[free_pool] provider %q has no catalog snapshot, using placeholder: %v", providerName, models))
			}
			realModel := models[0]
			if persistedSnapshot.SelectedModel != "" {
				realModel = persistedSnapshot.SelectedModel
			}
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
			deployment := DeploymentConfig{
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
			if entry, ok := findFreeModelCatalogEntry(persistedSnapshot.Models, realModel); ok {
				deployment = applyFreeModelCapabilities(deployment, entry)
			}
			autoDeployments[depID] = deployment
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
			updates := map[string]interface{}{}
			keyChanged := existingCh.Key != d.ch.Key
			modelsChanged := existingCh.Models != d.ch.Models
			if keyChanged {
				updates["key"] = d.ch.Key
			}
			if modelsChanged {
				updates["models"] = d.ch.Models
			}
			if existingCh.Type != d.ch.Type {
				updates["type"] = d.ch.Type
			}
			if len(updates) > 0 {
				if err := model.DB.Transaction(func(tx *gorm.DB) error {
					if err := tx.Model(existingCh).Updates(updates).Error; err != nil {
						return err
					}
					if !modelsChanged {
						return nil
					}
					updatedCh := *existingCh
					updatedCh.Models = d.ch.Models
					return updatedCh.UpdateAbilitiesWithDB(tx)
				}); err != nil {
					logger.SysError(fmt.Sprintf("[free_pool] failed to update channel %s: %v", d.name, err))
					return fmt.Errorf("failed to reconcile channel %s: %w", d.name, err)
				}
				existingCh.Key = d.ch.Key
				existingCh.Models = d.ch.Models
				existingCh.Type = d.ch.Type
				if modelsChanged {
					logger.SysLog(fmt.Sprintf("[free_pool] auto channel %s (id=%d) model inventory refreshed", d.name, existingCh.Id))
				}
				if keyChanged {
					logger.SysWarn(fmt.Sprintf("[free_pool] auto channel %s (id=%d) key updated - config sync overwrote previous key", d.name, existingCh.Id))
				}
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
	_ = RefreshFreePoolRuntimeStateWithReport()
}

func RefreshFreePoolRuntimeStateWithReport() FreeProviderCatalogSyncReport {
	cfg := GetConfig()
	report := syncAllProviderModels(cfg)
	syncOpenRouterCredits(cfg)
	return report
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

func updateChannelModelsAndAbilities(channel *model.Channel, newModels string) error {
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return updateChannelModelsAndAbilitiesWithDB(tx, channel, newModels)
	}); err != nil {
		return err
	}
	channel.Models = newModels
	return nil
}

func updateChannelModelsAndAbilitiesWithDB(tx *gorm.DB, channel *model.Channel, newModels string) error {
	if err := tx.Model(channel).Update("models", newModels).Error; err != nil {
		return err
	}
	updatedChannel := *channel
	updatedChannel.Models = newModels
	return updatedChannel.UpdateAbilitiesWithDB(tx)
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

type FreeProviderCatalogSyncResult struct {
	Provider   string   `json:"provider"`
	Attempted  int      `json:"attempted"`
	Succeeded  int      `json:"succeeded"`
	Failed     int      `json:"failed"`
	Skipped    int      `json:"skipped"`
	ModelCount int      `json:"model_count"`
	Errors     []string `json:"errors"`
}

type FreeProviderCatalogSyncReport struct {
	Attempted int                             `json:"attempted"`
	Succeeded int                             `json:"succeeded"`
	Failed    int                             `json:"failed"`
	Skipped   int                             `json:"skipped"`
	Results   []FreeProviderCatalogSyncResult `json:"results"`
}

var freeProviderCatalogRefreshMu sync.Mutex

var errFreeProviderCatalogRefreshSuperseded = errors.New("free provider catalog refresh superseded by config change")

type freeProviderCatalogAttemptResult struct {
	success    bool
	skipped    bool
	modelCount int
	errorText  string
}

// syncAllProviderModels serializes scheduled and manual refreshes so an older,
// slower request cannot overwrite a newer catalog.
func syncAllProviderModels(cfg *Config) FreeProviderCatalogSyncReport {
	report := FreeProviderCatalogSyncReport{}
	if cfg == nil || cfg.FreeProviders == nil {
		return report
	}
	freeProviderCatalogRefreshMu.Lock()
	defer freeProviderCatalogRefreshMu.Unlock()

	providerNames := make([]string, 0, len(cfg.FreeProviders))
	for providerName := range cfg.FreeProviders {
		providerNames = append(providerNames, providerName)
	}
	sort.Strings(providerNames)
	for _, providerName := range providerNames {
		fp := cfg.FreeProviders[providerName]
		if !fp.Enabled {
			continue
		}
		if len(fp.Models) > 0 {
			logger.SysLog(fmt.Sprintf("[free-pool] %s has configured models, skipping dynamic model sync", providerName))
			continue
		}
		meta, ok := BuiltinFreeProviders[providerName]
		if !ok || !isDynamicFreeProviderCatalog(meta.ModelFetchMode) {
			continue
		}

		keys := append([]string{}, fp.Keys...)
		if len(keys) == 0 && meta.Keyless {
			keys = []string{""}
		}
		providerResult := FreeProviderCatalogSyncResult{
			Provider: providerName,
			Errors:   []string{},
		}
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" && !meta.Keyless {
				continue
			}
			attempt := syncOneFreeProviderCatalog(providerName, key, meta)
			providerResult.Attempted++
			if attempt.modelCount > providerResult.ModelCount {
				providerResult.ModelCount = attempt.modelCount
			}
			if attempt.success {
				providerResult.Succeeded++
			} else if attempt.skipped {
				providerResult.Skipped++
			} else {
				providerResult.Failed++
				if attempt.errorText != "" {
					providerResult.Errors = append(providerResult.Errors, attempt.errorText)
				}
			}
		}
		if providerResult.Attempted == 0 {
			continue
		}
		report.Results = append(report.Results, providerResult)
		report.Attempted++
		if providerResult.Failed > 0 {
			report.Failed++
		} else if providerResult.Succeeded > 0 {
			report.Succeeded++
		} else {
			report.Skipped++
		}
	}
	return report
}

func isDynamicFreeProviderCatalog(fetchMode string) bool {
	switch fetchMode {
	case ModelFetchOpenRouterFree, ModelFetchKiloFree, ModelFetchOpenAIModels, ModelFetchOVHChat:
		return true
	default:
		return false
	}
}

func syncOneFreeProviderCatalog(providerName, key string, meta FreeProviderMeta) freeProviderCatalogAttemptResult {
	keyHash := SafeKeyHash(key)
	depID := deploymentID(providerName, keyHash)
	result := freeProviderCatalogAttemptResult{}
	attemptedAt := time.Now().UTC()
	candidate, err := fetchProviderCatalog(providerName, key)
	if err == nil {
		candidate, err = validateFreeProviderCatalog(candidate)
	}
	if err == nil {
		var channel model.Channel
		name := channelName(providerName, keyHash)
		err = model.DB.Where("name = ?", name).First(&channel).Error
		if err == nil {
			routingModel := routingModelForFetchedModels(providerName, freeProviderCatalogModelIDs(candidate.Models))
			err = applyValidatedFreeProviderCatalog(depID, providerName, key, meta, &channel, candidate, routingModel, attemptedAt)
		}
	}
	if err != nil {
		if errors.Is(err, errFreeProviderCatalogRefreshSuperseded) {
			result.skipped = true
			logger.SysLog(fmt.Sprintf("[free-pool] skipped superseded catalog refresh for %s", depID))
			return result
		}
		result.errorText = sanitizeFreeProviderCatalogError(err, key)
		safeErr := errors.New(result.errorText)
		if storeErr := markFreeProviderCatalogFailure(depID, providerName, meta.ModelFetchMode, attemptedAt, safeErr); storeErr != nil {
			logger.SysError(fmt.Sprintf("[free-pool] failed to persist catalog error for %s: %v", depID, storeErr))
		}
		logger.SysWarn(fmt.Sprintf("[free-pool] %s catalog refresh failed for %s: %s (keeping last successful snapshot)", providerName, depID, result.errorText))
		return result
	}
	result.success = true
	result.modelCount = len(candidate.Models)
	logger.SysLog(fmt.Sprintf("[free-pool] %s %s catalog synced: %d models", providerName, depID, result.modelCount))
	return result
}

func sanitizeFreeProviderCatalogError(err error, key string) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if key = strings.TrimSpace(key); key != "" {
		message = strings.ReplaceAll(message, key, "[redacted]")
	}
	message = strings.Join(strings.Fields(message), " ")
	const maxErrorLength = 512
	runes := []rune(message)
	if len(runes) > maxErrorLength {
		message = string(runes[:maxErrorLength])
	}
	return message
}

func applyValidatedFreeProviderCatalog(
	deploymentID string,
	providerName string,
	key string,
	meta FreeProviderMeta,
	channel *model.Channel,
	candidate FreeProviderCatalogCandidate,
	routingModel string,
	attemptedAt time.Time,
) error {
	if channel == nil {
		return fmt.Errorf("channel is nil")
	}
	if err := InitFreeProviderCatalogStore(); err != nil {
		return err
	}
	if routingModel == "" {
		return fmt.Errorf("catalog returned no routing model")
	}
	newModels := strings.Join(freeProviderCatalogModelIDs(candidate.Models), ",")
	previousSnapshot, hasPreviousSnapshot := GetFreeProviderCatalogSnapshot(deploymentID)

	freePoolMutationMu.Lock()
	defer freePoolMutationMu.Unlock()

	configLock.RLock()
	if config == nil || config.Deployments == nil {
		configLock.RUnlock()
		return fmt.Errorf("fallback config is not initialized")
	}
	current, ok := config.Deployments[deploymentID]
	if !ok {
		configLock.RUnlock()
		return fmt.Errorf("deployment %s is not initialized", deploymentID)
	}
	providerConfig, providerConfigured := config.FreeProviders[providerName]
	if !providerConfigured || !freeProviderConfigAllowsCatalogRefresh(providerConfig, meta, key) || current.ChannelID != channel.Id {
		configLock.RUnlock()
		return errFreeProviderCatalogRefreshSuperseded
	}
	target := current
	previousAutomaticSelection := hasPreviousSnapshot &&
		strings.TrimSpace(previousSnapshot.SelectedModel) != "" &&
		strings.TrimSpace(current.RealModel) == strings.TrimSpace(previousSnapshot.SelectedModel)
	if previousAutomaticSelection || !isDeploymentRealModelOverride(current.RealModel, routingModel, providerName) {
		target.RealModel = routingModel
		target.SupportsVision = meta.SupportsVision
		target.SupportsStream = meta.SupportsStream
		target.SupportsTools = meta.SupportsTools
		target.SupportsJSON = meta.SupportsJSON
		target.ContextLength = meta.ContextLength
		if entry, found := findFreeModelCatalogEntry(candidate.Models, routingModel); found {
			target = applyFreeModelCapabilities(target, entry)
		}
	} else if entry, found := findFreeModelCatalogEntry(candidate.Models, current.RealModel); found {
		target = applyFreeModelCapabilities(target, entry)
	}
	configLock.RUnlock()

	snapshot := FreeProviderCatalogSnapshot{
		DeploymentID:  deploymentID,
		Provider:      providerName,
		Source:        candidate.Source,
		Models:        candidate.Models,
		SelectedModel: target.RealModel,
		LastAttemptAt: attemptedAt,
		LastSuccessAt: attemptedAt,
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if channel.Models != newModels {
			if err := updateChannelModelsAndAbilitiesWithDB(tx, channel, newModels); err != nil {
				return err
			}
		}
		return saveFreeProviderCatalogSuccessWithDB(tx, snapshot)
	})
	if err == nil {
		configLock.Lock()
		latest := config.Deployments[deploymentID]
		latest.RealModel = target.RealModel
		latest.SupportsVision = target.SupportsVision
		latest.SupportsStream = target.SupportsStream
		latest.SupportsTools = target.SupportsTools
		latest.SupportsJSON = target.SupportsJSON
		latest.ContextLength = target.ContextLength
		config.Deployments[deploymentID] = latest
		configLock.Unlock()
	}
	if err != nil {
		return err
	}
	channel.Models = newModels
	cacheFreeProviderCatalogSnapshot(snapshot)
	return nil
}

func freeProviderConfigAllowsCatalogRefresh(fp FreeProviderConfig, meta FreeProviderMeta, key string) bool {
	if !fp.Enabled || len(fp.Models) > 0 || !isDynamicFreeProviderCatalog(meta.ModelFetchMode) {
		return false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return meta.Keyless && len(fp.Keys) == 0
	}
	for _, configuredKey := range fp.Keys {
		if strings.TrimSpace(configuredKey) == key {
			return true
		}
	}
	return false
}
