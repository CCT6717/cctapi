package fallback

import "testing"

func resetConfigForTest(cfg *Config) {
	configLock.Lock()
	defer configLock.Unlock()
	config = cfg
}

func TestLoadConfigDataDefaultsStrategyToQualityFirst(t *testing.T) {
	cfg, err := loadConfigData([]byte(`{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"pools": ["default"]
			}
		},
		"deployments": {
			"primary": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "real",
				"pool": "default"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("loadConfigData failed: %v", err)
	}

	if got := cfg.VirtualModels["test/auto"].Strategy; got != StrategyQualityFirst {
		t.Fatalf("expected default strategy %s, got %s", StrategyQualityFirst, got)
	}
	if len(cfg.VirtualModels["test/auto"].Pools) != 1 || cfg.VirtualModels["test/auto"].Pools[0] != "default" {
		t.Fatalf("expected pools [default], got %v", cfg.VirtualModels["test/auto"].Pools)
	}
}

func TestNormalizeStrategy(t *testing.T) {
	if got := NormalizeStrategy(" cost_first "); got != StrategyCostFirst {
		t.Fatalf("expected cost_first, got %s", got)
	}
	if got := NormalizeStrategy(" free_first "); got != StrategyFreeFirst {
		t.Fatalf("expected free_first, got %s", got)
	}
	if got := NormalizeStrategy("bad-value"); got != StrategyQualityFirst {
		t.Fatalf("expected invalid strategy to default to quality_first, got %s", got)
	}
}

func TestGetDeploymentsForVirtualModelFiltersByPool(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:  true,
				Strategy: StrategyFreeFirst,
				Pools:    []string{"free"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"free-1": {Enabled: true, ChannelID: 1, RealModel: "free1", Pool: "free", Priority: 1},
			"paid-1": {Enabled: true, ChannelID: 2, RealModel: "paid1", Pool: "paid_high", Priority: 1},
			"free-2": {Enabled: false, ChannelID: 1, RealModel: "free2", Pool: "free", Priority: 2},
		},
	})

	deployments, err := GetDeploymentsForVirtualModel("cct/free")
	if err != nil {
		t.Fatalf("GetDeploymentsForVirtualModel failed: %v", err)
	}

	if len(deployments) != 1 {
		t.Fatalf("expected 1 enabled free deployment, got %d: %v", len(deployments), deployments)
	}
	if deployments[0].ID != "free-1" {
		t.Fatalf("expected free-1, got %s", deployments[0].ID)
	}
}

func TestGetDeploymentsForVirtualModelStartsWithPreferredDeployment(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/high": {
				Enabled:             true,
				Strategy:            StrategyQualityFirst,
				Pools:               []string{"paid_high"},
				PreferredDeployment: "dep-b",
			},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, ChannelID: 1, RealModel: "a", Pool: "paid_high", Priority: 1},
			"dep-b": {Enabled: true, ChannelID: 2, RealModel: "b", Pool: "paid_high", Priority: 2},
			"dep-c": {Enabled: true, ChannelID: 3, RealModel: "c", Pool: "paid_high", Priority: 3},
		},
	})

	deployments, err := GetDeploymentsForVirtualModel("cct/high")
	if err != nil {
		t.Fatalf("GetDeploymentsForVirtualModel failed: %v", err)
	}
	if got := deployments[0].ID; got != "dep-b" {
		t.Fatalf("expected preferred deployment dep-b first, got %s", got)
	}
	if len(deployments) != 3 {
		t.Fatalf("expected fallback candidates to remain available, got %d", len(deployments))
	}
}

func TestGetDeploymentsForVirtualModelFixedUsesOnlyPreferredDeployment(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/high": {
				Enabled:             true,
				Strategy:            StrategyQualityFirst,
				Pools:               []string{"paid_high"},
				RoutingMode:         RoutingModeFixed,
				PreferredDeployment: "dep-b",
			},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, ChannelID: 1, RealModel: "a", Pool: "paid_high", Priority: 1},
			"dep-b": {Enabled: true, ChannelID: 2, RealModel: "b", Pool: "paid_high", Priority: 2},
		},
	})

	deployments, err := GetDeploymentsForVirtualModel("cct/high")
	if err != nil {
		t.Fatalf("GetDeploymentsForVirtualModel failed: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ID != "dep-b" {
		t.Fatalf("expected fixed mode to return only dep-b, got %#v", deployments)
	}
}

func TestValidateConfigDataRejectsPreferredDeploymentOutsideFallbackOrder(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/high": {
				Enabled:             true,
				Strategy:            StrategyQualityFirst,
				Pools:               []string{"paid_high"},
				PreferredDeployment: "dep-b",
				FallbackOrder:       []string{"dep-a"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"dep-a": {Enabled: true, ChannelID: 1, RealModel: "a", Pool: "paid_high", Priority: 1},
			"dep-b": {Enabled: true, ChannelID: 2, RealModel: "b", Pool: "paid_high", Priority: 2},
		},
	}

	if err := validateConfigData(cfg); err == nil {
		t.Fatalf("expected validateConfigData to reject preferred deployment outside fallback_order")
	}
}

func TestValidateConfigDataRejectsEmptyPools(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	cfg := &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:  true,
				Strategy: StrategyFreeFirst,
				Pools:    []string{},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"free-1": {Enabled: true, ChannelID: 1, RealModel: "free1", Pool: "free"},
		},
	}
	if err := validateConfigData(cfg); err == nil {
		t.Fatalf("expected validateConfigData to reject empty pools")
	}
}

func TestValidateConfigDataAllowsVMWithNoDeployments(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	cfg := &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:  true,
				Strategy: StrategyFreeFirst,
				Pools:    []string{"free"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"paid-1": {Enabled: true, ChannelID: 1, RealModel: "paid1", Pool: "paid_high"},
		},
	}
	// Empty-pool VMs now log a warning instead of rejecting the save —
	// this lets users bootstrap a config (e.g. cct/low with cheap/local/free
	// before any cheap/local deployments exist).
	if err := validateConfigData(cfg); err != nil {
		t.Fatalf("expected validateConfigData to allow VM with no enabled deployments (warning only), got: %v", err)
	}
}

func TestIsCCTVirtualModel(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"cct/high", true},
		{"cct/low", true},
		{"cct/free", true},
		{"high/auto", false},
		{"cct/auto", false},
		{"gpt-4", false},
	}
	for _, c := range cases {
		if got := IsCCTVirtualModel(c.name); got != c.want {
			t.Errorf("IsCCTVirtualModel(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
