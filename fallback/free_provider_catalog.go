package fallback

import (
	"sort"
	"strings"
	"time"
)

const freeProviderCatalogStaleAfter = 12 * time.Hour

type FreeProviderCatalogStatus struct {
	Refreshable     bool       `json:"refreshable"`
	State           string     `json:"state"`
	Source          string     `json:"source"`
	ModelCount      int        `json:"model_count"`
	DeploymentCount int        `json:"deployment_count"`
	SucceededCount  int        `json:"succeeded_count"`
	FailedCount     int        `json:"failed_count"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty"`
	Stale           bool       `json:"stale"`
	LastError       string     `json:"last_error,omitempty"`
}

type FreeProviderCatalogEntry struct {
	Name              string                    `json:"name"`
	Enabled           bool                      `json:"enabled"`
	KeyCount          int                       `json:"key_count"`
	Models            []string                  `json:"models,omitempty"`
	ProviderID        string                    `json:"provider_id"`
	ChannelType       int                       `json:"channel_type"`
	DefaultBaseURL    string                    `json:"default_base_url"`
	DefaultModels     []string                  `json:"default_models,omitempty"`
	DefaultModelCount int                       `json:"default_model_count"`
	RPMLimit          int                       `json:"rpm_limit"`
	RPDLimit          int                       `json:"rpd_limit"`
	TPMLimit          int                       `json:"tpm_limit"`
	TPDLimit          int                       `json:"tpd_limit"`
	ContextLength     int                       `json:"context_length"`
	SupportsVision    bool                      `json:"supports_vision"`
	SupportsStream    bool                      `json:"supports_stream"`
	SupportsTools     bool                      `json:"supports_tools"`
	SupportsJSON      bool                      `json:"supports_json"`
	RequiresKey       bool                      `json:"requires_key"`
	Keyless           bool                      `json:"keyless"`
	ModelFetchMode    string                    `json:"model_fetch_mode"`
	Quirks            *FreeProviderQuirks       `json:"quirks,omitempty"`
	CatalogStatus     FreeProviderCatalogStatus `json:"catalog_status"`
	ModelCapabilities []FreeModelCatalogEntry   `json:"model_capabilities"`
}

func BuildFreeProviderCatalog(cfg *Config) []FreeProviderCatalogEntry {
	return buildFreeProviderCatalogAt(cfg, time.Now().UTC())
}

func buildFreeProviderCatalogAt(cfg *Config, now time.Time) []FreeProviderCatalogEntry {
	names := make([]string, 0, len(BuiltinFreeProviders))
	for name := range BuiltinFreeProviders {
		names = append(names, name)
	}
	sort.Strings(names)

	var configured map[string]FreeProviderConfig
	if cfg != nil && cfg.FreeProviders != nil {
		configured = cfg.FreeProviders
	}

	catalog := make([]FreeProviderCatalogEntry, 0, len(names))
	for _, name := range names {
		meta := BuiltinFreeProviders[name]
		var fp FreeProviderConfig
		if configured != nil {
			fp = configured[name]
		}
		rpm, rpd, tpm, tpd := ResolveFreeProviderLimits(meta, fp)
		catalogStatus, modelCapabilities := buildFreeProviderCatalogRuntimeView(name, meta, fp, now)
		catalog = append(catalog, FreeProviderCatalogEntry{
			Name:              name,
			Enabled:           fp.Enabled,
			KeyCount:          len(fp.Keys),
			Models:            append([]string{}, fp.Models...),
			ProviderID:        meta.ProviderID,
			ChannelType:       meta.ChannelType,
			DefaultBaseURL:    meta.DefaultBaseURL,
			DefaultModels:     append([]string{}, meta.DefaultModels...),
			DefaultModelCount: len(meta.DefaultModels),
			RPMLimit:          rpm,
			RPDLimit:          rpd,
			TPMLimit:          tpm,
			TPDLimit:          tpd,
			ContextLength:     meta.ContextLength,
			SupportsVision:    meta.SupportsVision,
			SupportsStream:    meta.SupportsStream,
			SupportsTools:     meta.SupportsTools,
			SupportsJSON:      meta.SupportsJSON,
			RequiresKey:       meta.RequiresKey,
			Keyless:           meta.Keyless,
			ModelFetchMode:    meta.ModelFetchMode,
			Quirks:            cloneFreeProviderQuirks(meta.Quirks),
			CatalogStatus:     catalogStatus,
			ModelCapabilities: modelCapabilities,
		})
	}
	return catalog
}

func buildFreeProviderCatalogRuntimeView(
	providerName string,
	meta FreeProviderMeta,
	fp FreeProviderConfig,
	now time.Time,
) (FreeProviderCatalogStatus, []FreeModelCatalogEntry) {
	status := FreeProviderCatalogStatus{Source: meta.ModelFetchMode}
	if len(fp.Models) > 0 {
		status.State = "configured"
		status.Source = "configured"
		status.ModelCount = len(fp.Models)
		return status, []FreeModelCatalogEntry{}
	}
	if !isDynamicFreeProviderCatalog(meta.ModelFetchMode) {
		status.State = "static"
		status.ModelCount = len(meta.DefaultModels)
		return status, []FreeModelCatalogEntry{}
	}
	if !fp.Enabled {
		status.State = "disabled"
		return status, []FreeModelCatalogEntry{}
	}

	keys := append([]string{}, fp.Keys...)
	if len(keys) == 0 && meta.Keyless {
		keys = []string{""}
	}
	deploymentIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" && !meta.Keyless {
			continue
		}
		deploymentIDs = append(deploymentIDs, deploymentID(providerName, SafeKeyHash(key)))
	}
	if len(deploymentIDs) == 0 {
		status.State = "not_ready"
		return status, []FreeModelCatalogEntry{}
	}
	status.Refreshable = true
	status.DeploymentCount = len(deploymentIDs)

	modelsByID := map[string]FreeModelCatalogEntry{}
	missingSuccess := false
	var oldestSuccess time.Time
	var latestAttempt time.Time
	var latestErrorAt time.Time
	for _, depID := range deploymentIDs {
		snapshot, ok := GetFreeProviderCatalogSnapshot(depID)
		if !ok {
			missingSuccess = true
			continue
		}
		if snapshot.LastAttemptAt.After(latestAttempt) {
			latestAttempt = snapshot.LastAttemptAt
		}
		if snapshot.LastSuccessAt.IsZero() {
			missingSuccess = true
		} else if oldestSuccess.IsZero() || snapshot.LastSuccessAt.Before(oldestSuccess) {
			oldestSuccess = snapshot.LastSuccessAt
		}
		if !snapshot.LastSuccessAt.IsZero() {
			status.SucceededCount++
		}
		if snapshot.LastError != "" && (latestErrorAt.IsZero() || snapshot.LastAttemptAt.After(latestErrorAt)) {
			latestErrorAt = snapshot.LastAttemptAt
			status.LastError = snapshot.LastError
		}
		if snapshot.LastError != "" {
			status.FailedCount++
		}
		for _, entry := range snapshot.Models {
			if existing, exists := modelsByID[entry.ID]; exists {
				modelsByID[entry.ID] = mergeFreeModelCatalogEntries(existing, entry)
			} else {
				modelsByID[entry.ID] = cloneFreeModelCatalogEntry(entry)
			}
		}
	}
	if !latestAttempt.IsZero() {
		value := latestAttempt.UTC()
		status.LastAttemptAt = &value
	}
	if !oldestSuccess.IsZero() {
		value := oldestSuccess.UTC()
		status.LastSuccessAt = &value
	}
	switch {
	case status.LastError != "":
		status.State = "failed"
		status.Stale = true
	case !oldestSuccess.IsZero() && now.Sub(oldestSuccess) > freeProviderCatalogStaleAfter:
		status.State = "stale"
		status.Stale = true
	case missingSuccess || oldestSuccess.IsZero():
		status.State = "not_refreshed"
	default:
		status.State = "current"
	}

	modelIDs := make([]string, 0, len(modelsByID))
	for modelID := range modelsByID {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	modelCapabilities := make([]FreeModelCatalogEntry, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		modelCapabilities = append(modelCapabilities, cloneFreeModelCatalogEntry(modelsByID[modelID]))
	}
	status.ModelCount = len(modelCapabilities)
	return status, modelCapabilities
}

func mergeFreeModelCatalogEntries(left, right FreeModelCatalogEntry) FreeModelCatalogEntry {
	merged := FreeModelCatalogEntry{ID: left.ID}
	merged.SupportsStream = mergeCatalogBool(left.SupportsStream, right.SupportsStream)
	merged.SupportsTools = mergeCatalogBool(left.SupportsTools, right.SupportsTools)
	merged.SupportsJSON = mergeCatalogBool(left.SupportsJSON, right.SupportsJSON)
	merged.SupportsVision = mergeCatalogBool(left.SupportsVision, right.SupportsVision)
	merged.ContextLength = mergeCatalogInt(left.ContextLength, right.ContextLength)
	return merged
}

func mergeCatalogBool(left, right *bool) *bool {
	if left == nil || right == nil || *left != *right {
		return nil
	}
	return cloneBoolPtr(left)
}

func mergeCatalogInt(left, right *int) *int {
	if left == nil || right == nil || *left != *right {
		return nil
	}
	return cloneIntPtr(left)
}

func cloneFreeProviderQuirks(src *FreeProviderQuirks) *FreeProviderQuirks {
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
