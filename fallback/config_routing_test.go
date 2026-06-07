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
	if got := NormalizeRoutingMode("bad-value"); got != RoutingModeWeighted {
		t.Fatalf("expected invalid routing mode to default to weighted, got %s", got)
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
