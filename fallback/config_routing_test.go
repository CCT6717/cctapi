package fallback

import "testing"

func resetConfigForTest(cfg *Config) {
	configLock.Lock()
	defer configLock.Unlock()
	config = cfg
}

func TestLoadConfigDataDefaultsRoutingModeToWeighted(t *testing.T) {
	cfg, err := loadConfigData([]byte(`{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"fallback_order": ["primary"]
			}
		},
		"deployments": {
			"primary": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "real"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("loadConfigData failed: %v", err)
	}

	if got := cfg.VirtualModels["test/auto"].RoutingMode; got != RoutingModeWeighted {
		t.Fatalf("expected default routing mode %s, got %s", RoutingModeWeighted, got)
	}
}

func TestNormalizeRoutingMode(t *testing.T) {
	if got := NormalizeRoutingMode(" sequential "); got != RoutingModeSequential {
		t.Fatalf("expected sequential, got %s", got)
	}
	if got := NormalizeRoutingMode(" fixed "); got != RoutingModeFixed {
		t.Fatalf("expected fixed, got %s", got)
	}
	if got := NormalizeRoutingMode("bad-value"); got != RoutingModeWeighted {
		t.Fatalf("expected invalid routing mode to default to weighted, got %s", got)
	}
}

func TestLoadConfigDataPreservesFixedDeployment(t *testing.T) {
	cfg, err := loadConfigData([]byte(`{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"routing_mode": "fixed",
				"fixed_deployment": " primary ",
				"fallback_order": ["primary"]
			}
		},
		"deployments": {
			"primary": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "real"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("loadConfigData failed: %v", err)
	}

	vm := cfg.VirtualModels["test/auto"]
	if vm.RoutingMode != RoutingModeFixed {
		t.Fatalf("expected routing mode %s, got %s", RoutingModeFixed, vm.RoutingMode)
	}
	if vm.FixedDeployment != "primary" {
		t.Fatalf("expected fixed deployment to be trimmed to primary, got %q", vm.FixedDeployment)
	}
}

func TestGetDeploymentsForVirtualModelSequentialPreservesFallbackOrder(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"test/auto": {
				Enabled:       true,
				RoutingMode:   RoutingModeSequential,
				FallbackOrder: []string{"second", "first", "third"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"first":  {Enabled: true, ChannelID: 1, RealModel: "first", Priority: 1},
			"second": {Enabled: true, ChannelID: 1, RealModel: "second", Priority: 2},
			"third":  {Enabled: true, ChannelID: 1, RealModel: "third", Priority: 3},
		},
	})

	deployments, err := GetDeploymentsForVirtualModel("test/auto")
	if err != nil {
		t.Fatalf("GetDeploymentsForVirtualModel failed: %v", err)
	}

	expected := []string{"second", "first", "third"}
	for i, deploymentID := range expected {
		if deployments[i].ID != deploymentID {
			t.Fatalf("expected index %d to be %s, got %s", i, deploymentID, deployments[i].ID)
		}
	}
}

func TestGetDeploymentsForVirtualModelFixedReturnsOnlyFixedDeployment(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"test/auto": {
				Enabled:         true,
				RoutingMode:     RoutingModeFixed,
				FixedDeployment: "third",
				FallbackOrder:   []string{"first", "second", "third"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"first":  {Enabled: true, ChannelID: 1, RealModel: "first", Priority: 1},
			"second": {Enabled: true, ChannelID: 1, RealModel: "second", Priority: 2},
			"third":  {Enabled: true, ChannelID: 1, RealModel: "third", Priority: 3},
		},
	})

	deployments, err := GetDeploymentsForVirtualModel("test/auto")
	if err != nil {
		t.Fatalf("GetDeploymentsForVirtualModel failed: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected one fixed deployment, got %d: %v", len(deployments), deployments)
	}
	if deployments[0].ID != "third" {
		t.Fatalf("expected fixed deployment third, got %s", deployments[0].ID)
	}
}

func TestGetDeploymentsForVirtualModelFixedFallsBackToFirstAvailableDeployment(t *testing.T) {
	t.Cleanup(func() {
		resetConfigForTest(nil)
	})
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"test/auto": {
				Enabled:         true,
				RoutingMode:     RoutingModeFixed,
				FixedDeployment: "missing",
				FallbackOrder:   []string{"disabled", "first", "second"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"disabled": {Enabled: false, ChannelID: 1, RealModel: "disabled", Priority: 1},
			"first":    {Enabled: true, ChannelID: 1, RealModel: "first", Priority: 2},
			"second":   {Enabled: true, ChannelID: 1, RealModel: "second", Priority: 3},
		},
	})

	deployments, err := GetDeploymentsForVirtualModel("test/auto")
	if err != nil {
		t.Fatalf("GetDeploymentsForVirtualModel failed: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected fallback to a single first available deployment, got %d: %v", len(deployments), deployments)
	}
	if deployments[0].ID != "first" {
		t.Fatalf("expected first available deployment first, got %s", deployments[0].ID)
	}
}

func TestValidateConfigDataRejectsInvalidFixedDeployment(t *testing.T) {
	baseConfig := func() *Config {
		return &Config{
			Enabled: true,
			VirtualModels: map[string]VirtualModelConfig{
				"test/auto": {
					Enabled:         true,
					RoutingMode:     RoutingModeFixed,
					FixedDeployment: "first",
					FallbackOrder:   []string{"first", "second"},
				},
			},
			Deployments: map[string]DeploymentConfig{
				"first":  {Enabled: true, ChannelID: 1, RealModel: "first"},
				"second": {Enabled: true, ChannelID: 1, RealModel: "second"},
			},
		}
	}

	tests := []struct {
		name string
		edit func(*Config)
	}{
		{
			name: "empty fixed deployment",
			edit: func(cfg *Config) {
				vm := cfg.VirtualModels["test/auto"]
				vm.FixedDeployment = ""
				cfg.VirtualModels["test/auto"] = vm
			},
		},
		{
			name: "fixed deployment outside fallback order",
			edit: func(cfg *Config) {
				vm := cfg.VirtualModels["test/auto"]
				vm.FixedDeployment = "missing"
				cfg.VirtualModels["test/auto"] = vm
			},
		},
		{
			name: "disabled fixed deployment",
			edit: func(cfg *Config) {
				dep := cfg.Deployments["first"]
				dep.Enabled = false
				cfg.Deployments["first"] = dep
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			tt.edit(cfg)
			if err := validateConfigData(cfg); err == nil {
				t.Fatalf("expected validateConfigData to reject %s", tt.name)
			}
		})
	}
}
