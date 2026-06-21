package fallback

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/songquanpeng/one-api/common/logger"
)

type FreeProviderLimits struct {
	RPMLimit *int `json:"rpm_limit,omitempty"`
	RPDLimit *int `json:"rpd_limit,omitempty"`
	TPMLimit *int `json:"tpm_limit,omitempty"`
	TPDLimit *int `json:"tpd_limit,omitempty"`
}

type FreeProviderConfig struct {
	Enabled        bool                `json:"enabled"`
	Keys           []string            `json:"keys"`
	Models         []string            `json:"models,omitempty"`
	DefaultRPM     int                 `json:"default_rpm,omitempty"`
	DefaultRPD     int                 `json:"default_rpd,omitempty"`
	DefaultTPM     int                 `json:"default_tpm,omitempty"`
	DefaultTPD     int                 `json:"default_tpd,omitempty"`
	LimitsOverride *FreeProviderLimits `json:"limits_override,omitempty"`
}

type Config struct {
	Enabled       bool                          `json:"enabled"`
	VirtualModels map[string]VirtualModelConfig `json:"virtual_models"`
	Deployments   map[string]DeploymentConfig   `json:"deployments"`
	FreeProviders map[string]FreeProviderConfig `json:"free_providers,omitempty"`
	Alert         AlertConfig                   `json:"alert"`
	SmartSort     SmartSortConfig               `json:"smart_sort"`
	BlockedErrorCodes []string                  `json:"blocked_error_codes"`
}

type VirtualModelConfig struct {
	Enabled            bool     `json:"enabled"`
	Description        string   `json:"description,omitempty"`
	Strategy           string   `json:"strategy"` // quality_first / cost_first / free_first
	Pools              []string `json:"pools"`
	AllowDegradeToLow  bool     `json:"allow_degrade_to_low"`
	AllowDegradeToFree bool     `json:"allow_degrade_to_free"`
	// Legacy — populated from old-format fallback.json, ignored for new configs.
	// These get zero-values when JSON doesn't have them, so new configs are clean.
	RoutingMode      string   `json:"routing_mode,omitempty"`
	FallbackOrder    []string `json:"fallback_order,omitempty"`
	FixedDeployment  string   `json:"fixed_deployment,omitempty"`
}

type DeploymentConfig struct {
	ID                    string `json:"-"`
	Enabled               bool   `json:"enabled"`
	ChannelID             int    `json:"channel_id"`
	RealModel             string `json:"real_model"`
	Pool                  string `json:"pool"` // paid_high / cheap / local / free
	QualityTier           string `json:"quality_tier"` // high / medium / low
	CostTier              string `json:"cost_tier"`    // free / cheap / paid
	SupportsVision        bool   `json:"supports_vision"`
	SupportsStream        bool   `json:"supports_stream"`
	SupportsTools         bool   `json:"supports_tools"`
	SupportsJSON          bool   `json:"supports_json"`
	ContextLength         int    `json:"context_length"`
	Priority              int    `json:"priority"`
	Weight                int    `json:"weight"`
	MaxConcurrentRequests int    `json:"max_concurrent_requests"`
	DailyLimitTokens      int64  `json:"daily_limit_tokens"`
	QuotaMode             string `json:"quota_mode"`
	SoftLimitRatio        float64 `json:"soft_limit_ratio"`
	HardLimitRatio        float64 `json:"hard_limit_ratio"`
	RPMLimit              int    `json:"rpm_limit"`
	RPDLimit              int    `json:"rpd_limit"`
	TPMLimit              int    `json:"tpm_limit"`
	TPDLimit              int    `json:"tpd_limit"`
}

var (
	config     *Config
	configLock sync.RWMutex
)

const (
	StrategyQualityFirst = "quality_first"
	StrategyCostFirst    = "cost_first"
	StrategyFreeFirst    = "free_first"
)

var ValidStrategies = []string{StrategyQualityFirst, StrategyCostFirst, StrategyFreeFirst}

func normalizeStrategy(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StrategyQualityFirst:
		return StrategyQualityFirst
	case StrategyCostFirst:
		return StrategyCostFirst
	case StrategyFreeFirst:
		return StrategyFreeFirst
	default:
		return StrategyQualityFirst
	}
}

func NormalizeStrategy(s string) string {
	return normalizeStrategy(s)
}

// loadConfigData parses JSON config data and applies normalization defaults.
// It does NOT touch the global config or mutex — safe to call without holding any lock.
func loadConfigData(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.VirtualModels == nil {
		cfg.VirtualModels = map[string]VirtualModelConfig{}
	}
	if cfg.Deployments == nil {
		cfg.Deployments = map[string]DeploymentConfig{}
	}

	// Default SmartSort config
	if cfg.SmartSort.Weights.BasePriorityPenalty <= 0 {
		defaults := DefaultSmartSortConfig()
		cfg.SmartSort.Weights = defaults.Weights
	}

	for name, vm := range cfg.VirtualModels {
		vm.Strategy = normalizeStrategy(vm.Strategy)
		if len(vm.Pools) == 0 {
			vm.Pools = []string{"default"}
		}
		cfg.VirtualModels[name] = vm
	}

	// Detect old-format config (routing_mode/fallback_order era).
	for name, vm := range cfg.VirtualModels {
		if vm.RoutingMode == "" {
			continue
		}
		logger.SysLogf("[config] legacy VM %q with routing_mode=%s — assigning synthetic pool", name, vm.RoutingMode)

		switch vm.RoutingMode {
		case "fixed":
			// fixed_deployment is handled by GetDeploymentsForVirtualModel
			// (returns just the one deployment). Still assign pool to the
			// fixed deployment so validateConfigData can verify it.
			poolName := "_fixed_" + name
			if dep, ok := cfg.Deployments[vm.FixedDeployment]; ok {
				dep.Pool = poolName
				cfg.Deployments[vm.FixedDeployment] = dep
			}
			vm.Pools = []string{poolName}
		default:
			// weighted / sequential: assign fallback_order deployments to a
			// VM-specific pool so pool-based filtering works correctly.
			poolName := "_legacy_" + name
			for _, depID := range vm.FallbackOrder {
				if dep, ok := cfg.Deployments[depID]; ok {
					dep.Pool = poolName
					cfg.Deployments[depID] = dep
				}
			}
			vm.Pools = []string{poolName}
		}
		cfg.VirtualModels[name] = vm
	}

	for id, dep := range cfg.Deployments {
		dep.ID = id
		if dep.Pool == "" {
			dep.Pool = "default"
		}
		if dep.QualityTier == "" {
			dep.QualityTier = "medium"
		}
		if dep.CostTier == "" {
			dep.CostTier = "paid"
		}
		if dep.Weight <= 0 {
			dep.Weight = 100
		}
		if dep.MaxConcurrentRequests < 0 {
			dep.MaxConcurrentRequests = 0
		}
		if dep.SoftLimitRatio <= 0 {
			dep.SoftLimitRatio = 0.95
		}
		if dep.HardLimitRatio <= 0 {
			dep.HardLimitRatio = 1.0
		}

		// Safety guard: enabled deployment with channel_id <= 0 is a broken config.
		// Auto-disable with a warning instead of silently routing nowhere.
		if dep.Enabled && dep.ChannelID <= 0 {
			logger.SysWarn(fmt.Sprintf("[config] deployment %q has enabled=true but channel_id=%d — disabling it; set a valid channel_id or keep enabled=false", id, dep.ChannelID))
			dep.Enabled = false
		}

		cfg.Deployments[id] = dep
	}

	return &cfg, nil
}

func LoadConfig(path string) error {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			config = &Config{
				Enabled:       false,
				VirtualModels: map[string]VirtualModelConfig{},
				Deployments:   map[string]DeploymentConfig{},
			}
			return nil
		}
		return err
	}

	newCfg, err := loadConfigData(data)
	if err != nil {
		return err
	}

	config = newCfg
	return nil
}

func GetConfig() *Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

func IsEnabled() bool {
	configLock.RLock()
	defer configLock.RUnlock()
	return config != nil && config.Enabled
}

func IsVirtualModel(modelName string) bool {
	configLock.RLock()
	defer configLock.RUnlock()

	if config == nil || !config.Enabled {
		return false
	}

	vm, ok := config.VirtualModels[modelName]
	return ok && vm.Enabled
}

func GetVirtualModel(modelName string) (*VirtualModelConfig, bool) {
	configLock.RLock()
	defer configLock.RUnlock()

	if config == nil || !config.Enabled {
		return nil, false
	}

	vm, ok := config.VirtualModels[modelName]
	return &vm, ok
}

// GetAllVirtualModelNames returns a list of all enabled virtual model names
func GetAllVirtualModelNames() []string {
	configLock.RLock()
	defer configLock.RUnlock()

	if config == nil || !config.Enabled {
		return nil
	}

	names := make([]string, 0)
	for name, vm := range config.VirtualModels {
		if vm.Enabled {
			names = append(names, name)
		}
	}
	return names
}

func GetDeployment(id string) (*DeploymentConfig, bool) {
	configLock.RLock()
	defer configLock.RUnlock()

	if config == nil {
		return nil, false
	}

	dep, ok := config.Deployments[id]
	return &dep, ok
}

func GetDeploymentsForVirtualModel(modelName string) ([]DeploymentConfig, error) {
	configLock.RLock()

	if config == nil || !config.Enabled {
		configLock.RUnlock()
		return nil, fmt.Errorf("fallback config is not enabled")
	}

	vm, ok := config.VirtualModels[modelName]
	if !ok || !vm.Enabled {
		configLock.RUnlock()
		return nil, fmt.Errorf("virtual model not found or disabled: %s", modelName)
	}

	// Legacy fixed mode: return only the fixed_deployment, bypass pool/strategy.
	if vm.RoutingMode == "fixed" && vm.FixedDeployment != "" {
		if dep, ok := config.Deployments[vm.FixedDeployment]; ok && dep.Enabled {
			dep.ID = vm.FixedDeployment
			configLock.RUnlock()
			return []DeploymentConfig{dep}, nil
		}
		configLock.RUnlock()
		return nil, fmt.Errorf("legacy fixed deployment %s not found or disabled for VM %s",
			vm.FixedDeployment, modelName)
	}

	pools := vm.Pools
	deployments := make([]DeploymentConfig, 0)
	for depID, dep := range config.Deployments {
		if !dep.Enabled {
			continue
		}
		for _, p := range pools {
			if dep.Pool == p {
				dep.ID = depID
				deployments = append(deployments, dep)
				break
			}
		}
	}
	smartSortEnabled := config.SmartSort.Enabled
	configLock.RUnlock()

	if len(deployments) == 0 {
		return nil, fmt.Errorf("no enabled deployments found for virtual model: %s", modelName)
	}

	if smartSortEnabled {
		today := todayString()
		configLock.RLock()
		weights := config.SmartSort.Weights
		configLock.RUnlock()
		sort.SliceStable(deployments, func(i, j int) bool {
			return getDeploymentScore(deployments[i], today, weights) > getDeploymentScore(deployments[j], today, weights)
		})
	} else {
		sort.SliceStable(deployments, func(i, j int) bool {
			return deployments[i].Priority < deployments[j].Priority
		})
	}

	return deployments, nil
}

// getDeploymentScore calculates the smart score for a deployment
func getDeploymentScore(dep DeploymentConfig, date string, weights ScoreWeights) float64 {
	state, err := GetDeploymentState(dep.ID, date)
	if err != nil {
		// No history yet — use static priority
		return float64(100 - (dep.Priority-1)*int(weights.BasePriorityPenalty))
	}

	return CalculateScore(dep, state, weights)
}

func GetFirstDeploymentForVirtualModel(modelName string) (*DeploymentConfig, error) {
	deployments, err := GetDeploymentsForVirtualModel(modelName)
	if err != nil {
		return nil, err
	}

	if len(deployments) == 0 {
		return nil, fmt.Errorf("no deployments found for virtual model: %s", modelName)
	}

	// Return a copy to avoid issues with slice element lifetime
	dep := deployments[0]
	return &dep, nil
}

func IsCCTVirtualModel(modelName string) bool {
	return modelName == "cct/high" || modelName == "cct/low" || modelName == "cct/free"
}

// validateConfigData checks that a config is semantically valid.
// It does NOT touch the global config or mutex.
func validateConfigData(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if !cfg.Enabled {
		return fmt.Errorf("fallback is not enabled")
	}

	if len(cfg.VirtualModels) == 0 {
		return fmt.Errorf("no virtual models configured")
	}

	for modelName, vm := range cfg.VirtualModels {
		if !vm.Enabled {
			continue
		}

		if len(vm.Pools) == 0 {
			return fmt.Errorf("virtual model %s has empty pools", modelName)
		}

		// Verify at least one deployment exists for these pools
		hasDeployment := false
		for _, dep := range cfg.Deployments {
			if !dep.Enabled {
				continue
			}
			for _, p := range vm.Pools {
				if dep.Pool == p {
					hasDeployment = true
					break
				}
			}
			if hasDeployment {
				break
			}
		}
		if !hasDeployment {
			return fmt.Errorf("virtual model %s has no enabled deployments in pools %v", modelName, vm.Pools)
		}
	}

	// Validate free_providers limits_override (reject negative values)
	for name, fp := range cfg.FreeProviders {
		if !fp.Enabled {
			continue
		}
		if fp.LimitsOverride != nil {
			if err := ValidateFreeProviderLimits(fp.LimitsOverride); err != nil {
				return fmt.Errorf("free_provider %q limits_override: %w", name, err)
			}
		}
	}

	// Reject enabled deployments with invalid channel_id (safety guard)
	for id, dep := range cfg.Deployments {
		if dep.Enabled && dep.ChannelID <= 0 {
			return fmt.Errorf("deployment %q has enabled=true but invalid channel_id=%d", id, dep.ChannelID)
		}
	}

	return nil
}

func ValidateConfig() error {
	configLock.RLock()
	defer configLock.RUnlock()
	return validateConfigData(config)
}

func ReloadConfig(path string) error {
	// Step 1: Read the file (no lock needed)
	data, err := os.ReadFile(path)
	if err != nil {
		logger.SysError(fmt.Sprintf("[config] failed to read config file %s: %v", path, err))
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Step 2: Parse and normalize into a TEMPORARY variable (no lock needed)
	newCfg, err := loadConfigData(data)
	if err != nil {
		logger.SysError(fmt.Sprintf("[config] failed to parse config file %s: %v", path, err))
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Step 3: Sync free pool BEFORE validation — auto-deployments must exist
	// for pool-based virtual models (e.g. cct/free with pools=["free"]) to
	// pass validateConfigData.
	if err := SyncFreePool(newCfg); err != nil {
		logger.SysError(fmt.Sprintf("[config] failed to sync free pool: %v", err))
	}

	// Step 4: Validate the new config before swapping
	if err := validateConfigData(newCfg); err != nil {
		logger.SysError(fmt.Sprintf("[config] validation failed for %s, keeping old config: %v", path, err))
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Step 5: Swap under write lock
	configLock.Lock()
	config = newCfg
	configLock.Unlock()

	logger.SysLog(fmt.Sprintf("[config] configuration reloaded successfully from %s", path))
	return nil
}
