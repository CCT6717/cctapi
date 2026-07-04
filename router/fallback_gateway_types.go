package router

import "github.com/songquanpeng/one-api/fallback"

// v2 gateway config response types

type gatewayV2Config struct {
	Enabled             bool                                `json:"enabled"`
	VirtualModels       map[string]gatewayV2VirtualModel    `json:"virtual_models"`
	Deployments         map[string]gatewayV2Deployment      `json:"deployments"`
	FreeProviders       map[string]gatewayV2FreeProvider    `json:"free_providers"`
	FreeProviderCatalog []fallback.FreeProviderCatalogEntry `json:"free_provider_catalog"`
}

type gatewayV2VirtualModel struct {
	Enabled             bool     `json:"enabled"`
	Strategy            string   `json:"strategy"`
	Pools               []string `json:"pools"`
	RoutingMode         string   `json:"routing_mode,omitempty"`
	PreferredDeployment string   `json:"preferred_deployment,omitempty"`
	FallbackOrder       []string `json:"fallback_order,omitempty"`
	AllowDegradeToLow   bool     `json:"allow_degrade_to_low"`
	AllowDegradeToFree  bool     `json:"allow_degrade_to_free"`
}

type gatewayV2Deployment struct {
	Enabled          bool    `json:"enabled"`
	ChannelID        int     `json:"channel_id"`
	RealModel        string  `json:"real_model"`
	Pool             string  `json:"pool"`
	QualityTier      string  `json:"quality_tier"`
	CostTier         string  `json:"cost_tier"`
	QuotaMode        string  `json:"quota_mode"`
	SupportsStream   bool    `json:"supports_stream"`
	SupportsVision   bool    `json:"supports_vision"`
	SupportsTools    bool    `json:"supports_tools"`
	SupportsJSON     bool    `json:"supports_json"`
	ContextLength    int     `json:"context_length"`
	RPMLimit         int     `json:"rpm_limit"`
	RPDLimit         int     `json:"rpd_limit"`
	TPMLimit         int     `json:"tpm_limit"`
	TPDLimit         int     `json:"tpd_limit"`
	Priority         int     `json:"priority"`
	Weight           int     `json:"weight"`
	DailyLimitTokens int64   `json:"daily_limit_tokens"`
	SoftLimitRatio   float64 `json:"soft_limit_ratio"`
	HardLimitRatio   float64 `json:"hard_limit_ratio"`
}

type gatewayV2FreeProvider struct {
	Enabled        bool                         `json:"enabled"`
	KeyCount       int                          `json:"key_count"`
	Models         []string                     `json:"models,omitempty"`
	ProviderID     string                       `json:"provider_id,omitempty"`
	ChannelType    int                          `json:"channel_type,omitempty"`
	DefaultBaseURL string                       `json:"default_base_url,omitempty"`
	DefaultModels  []string                     `json:"default_models,omitempty"`
	DefaultRPM     int                          `json:"default_rpm,omitempty"`
	DefaultRPD     int                          `json:"default_rpd,omitempty"`
	DefaultTPM     int                          `json:"default_tpm,omitempty"`
	DefaultTPD     int                          `json:"default_tpd,omitempty"`
	ContextLength  int                          `json:"context_length,omitempty"`
	SupportsVision bool                         `json:"supports_vision"`
	SupportsStream bool                         `json:"supports_stream"`
	SupportsTools  bool                         `json:"supports_tools"`
	SupportsJSON   bool                         `json:"supports_json"`
	RequiresKey    bool                         `json:"requires_key"`
	Keyless        bool                         `json:"keyless"`
	ModelFetchMode string                       `json:"model_fetch_mode,omitempty"`
	Quirks         *fallback.FreeProviderQuirks `json:"quirks,omitempty"`
	LimitsOverride *gatewayV2LimitsOverride     `json:"limits_override,omitempty"`
}

type gatewayV2LimitsOverride struct {
	RPMLimit *int `json:"rpm_limit,omitempty"`
	RPDLimit *int `json:"rpd_limit,omitempty"`
	TPMLimit *int `json:"tpm_limit,omitempty"`
	TPDLimit *int `json:"tpd_limit,omitempty"`
}

// v2 gateway config request types (PUT)

type gatewayV2ConfigInput struct {
	Enabled       bool                                  `json:"enabled"`
	VirtualModels map[string]gatewayV2VirtualModel      `json:"virtual_models"`
	Deployments   map[string]gatewayV2Deployment        `json:"deployments"`
	FreeProviders map[string]gatewayV2FreeProviderInput `json:"free_providers"`
}

type gatewayV2FreeProviderInput struct {
	Enabled        bool                     `json:"enabled"`
	Keys           []string                 `json:"keys,omitempty"`
	Models         []string                 `json:"models,omitempty"`
	LimitsOverride *gatewayV2LimitsOverride `json:"limits_override,omitempty"`
}
