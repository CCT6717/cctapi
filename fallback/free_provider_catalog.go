package fallback

import "sort"

type FreeProviderCatalogEntry struct {
	Name              string              `json:"name"`
	Enabled           bool                `json:"enabled"`
	KeyCount          int                 `json:"key_count"`
	Models            []string            `json:"models,omitempty"`
	ProviderID        string              `json:"provider_id"`
	ChannelType       int                 `json:"channel_type"`
	DefaultBaseURL    string              `json:"default_base_url"`
	DefaultModels     []string            `json:"default_models,omitempty"`
	DefaultModelCount int                 `json:"default_model_count"`
	RPMLimit          int                 `json:"rpm_limit"`
	RPDLimit          int                 `json:"rpd_limit"`
	TPMLimit          int                 `json:"tpm_limit"`
	TPDLimit          int                 `json:"tpd_limit"`
	ContextLength     int                 `json:"context_length"`
	SupportsVision    bool                `json:"supports_vision"`
	SupportsStream    bool                `json:"supports_stream"`
	SupportsTools     bool                `json:"supports_tools"`
	SupportsJSON      bool                `json:"supports_json"`
	RequiresKey       bool                `json:"requires_key"`
	Keyless           bool                `json:"keyless"`
	ModelFetchMode    string              `json:"model_fetch_mode"`
	Quirks            *FreeProviderQuirks `json:"quirks,omitempty"`
}

func BuildFreeProviderCatalog(cfg *Config) []FreeProviderCatalogEntry {
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
		rpm, rpd, tpm, tpd := ApplyLimitsOverride(meta.DefaultRPM, meta.DefaultRPD, meta.DefaultTPM, meta.DefaultTPD, fp.LimitsOverride)
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
		})
	}
	return catalog
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
