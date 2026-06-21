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

type Config struct {
	Enabled       bool                          `json:"enabled"`
	VirtualModels map[string]VirtualModelConfig `json:"virtual_models"`
	Deployments   map[string]DeploymentConfig   `json:"deployments"`
	Alert         AlertConfig                   `json:"alert"`
	SmartSort     SmartSortConfig               `json:"smart_sort"`
	BlockedErrorCodes []string                  `json:"blocked_error_codes"`
}

type VirtualModelConfig struct {
	Enabled         bool     `json:"enabled"`
	Description     string   `json:"description"`
	RoutingMode     string   `json:"routing_mode"`
	FixedDeployment string   `json:"fixed_deployment,omitempty"`
	FallbackOrder   []string `json:"fallback_order"`
}

type DeploymentConfig struct {
	ID                    string  `json:"-"`
	Enabled               bool    `json:"enabled"`
	ChannelID             int     `json:"channel_id"`
	RealModel             string  `json:"real_model"`
	Priority              int     `json:"priority"`
	Weight                int     `json:"weight"`
	MaxConcurrentRequests int     `json:"max_concurrent_requests"`
	DailyLimitTokens      int64   `json:"daily_limit_tokens"`
	QuotaMode             string  `json:"quota_mode"`
	SoftLimitRatio        float64 `json:"soft_limit_ratio"`
	HardLimitRatio        float64 `json:"hard_limit_ratio"`
	MaxContext            int     `json:"max_context"`
	MinContext            int     `json:"min_context"`
}

var (
	config     *Config
	configLock sync.RWMutex
)

const (
	RoutingModeWeighted   = "weighted"
	RoutingModeSequential = "sequential"
	RoutingModeFixed      = "fixed"
)

func normalizeRoutingMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case RoutingModeFixed:
		return RoutingModeFixed
	case RoutingModeSequential:
		return RoutingModeSequential
	case RoutingModeWeighted:
		return RoutingModeWeighted
	default:
		return RoutingModeWeighted
	}
}

func NormalizeRoutingMode(mode string) string {
	return normalizeRoutingMode(mode)
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
		vm.RoutingMode = normalizeRoutingMode(vm.RoutingMode)
		vm.FixedDeployment = strings.TrimSpace(vm.FixedDeployment)
		cfg.VirtualModels[name] = vm
	}

	for id, dep := range cfg.Deployments {
		dep.ID = id
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

func GetRoutingModeForVirtualModel(modelName string) string {
	configLock.RLock()
	defer configLock.RUnlock()

	if config == nil || !config.Enabled {
		return RoutingModeWeighted
	}

	vm, ok := config.VirtualModels[modelName]
	if !ok || !vm.Enabled {
		return RoutingModeWeighted
	}

	return normalizeRoutingMode(vm.RoutingMode)
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

	deployments := make([]DeploymentConfig, 0)

	for _, deploymentID := range vm.FallbackOrder {
		dep, ok := config.Deployments[deploymentID]
		if !ok {
			continue
		}
		if !dep.Enabled {
			continue
		}
		dep.ID = deploymentID
		deployments = append(deployments, dep)
	}

	routingMode := normalizeRoutingMode(vm.RoutingMode)
	smartSortEnabled := config.SmartSort.Enabled
	scoreWeights := config.SmartSort.Weights
	configLock.RUnlock()

	if len(deployments) == 0 {
		return nil, fmt.Errorf("no enabled deployments found for virtual model: %s", modelName)
	}

	if routingMode == RoutingModeFixed {
		if vm.FixedDeployment != "" {
			for _, dep := range deployments {
				if dep.ID == vm.FixedDeployment {
					return []DeploymentConfig{dep}, nil
				}
			}
		}
		return deployments[:1], nil
	}

	if routingMode == RoutingModeSequential {
		return deployments, nil
	}

	if smartSortEnabled {
		// Smart sorting: order by dynamic score (highest first)
		today := todayString()
		sort.SliceStable(deployments, func(i, j int) bool {
			return getDeploymentScore(deployments[i], today, scoreWeights) > getDeploymentScore(deployments[j], today, scoreWeights)
		})
	} else {
		// Static sorting: order by configured priority (lowest number first)
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

		if len(vm.FallbackOrder) == 0 {
			return fmt.Errorf("virtual model %s has empty fallback_order", modelName)
		}

		fallbackOrder := make(map[string]bool, len(vm.FallbackOrder))
		for _, deploymentID := range vm.FallbackOrder {
			dep, ok := cfg.Deployments[deploymentID]
			if !ok {
				return fmt.Errorf("virtual model %s references unknown deployment: %s", modelName, deploymentID)
			}
			if dep.Enabled {
				fallbackOrder[deploymentID] = true
			} else {
				fallbackOrder[deploymentID] = false
			}
		}

		if normalizeRoutingMode(vm.RoutingMode) == RoutingModeFixed {
			if vm.FixedDeployment == "" {
				return fmt.Errorf("fixed virtual model %s has empty fixed_deployment", modelName)
			}
			fixedEnabled, ok := fallbackOrder[vm.FixedDeployment]
			if !ok {
				return fmt.Errorf("fixed deployment %s must be in fallback_order for virtual model %s", vm.FixedDeployment, modelName)
			}
			if !fixedEnabled {
				return fmt.Errorf("fixed deployment %s for virtual model %s is disabled", vm.FixedDeployment, modelName)
			}
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

	// Step 3: Validate the new config before swapping
	if err := validateConfigData(newCfg); err != nil {
		logger.SysError(fmt.Sprintf("[config] validation failed for %s, keeping old config: %v", path, err))
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Step 4: Swap under write lock
	configLock.Lock()
	config = newCfg
	configLock.Unlock()

	logger.SysLog(fmt.Sprintf("[config] configuration reloaded successfully from %s", path))
	return nil
}
