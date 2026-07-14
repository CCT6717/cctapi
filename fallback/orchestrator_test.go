package fallback

import (
	"testing"
	"time"
)

func resetFallbackPlanningStateForTest(t *testing.T, cfg *Config) {
	t.Helper()
	resetConfigForTest(cfg)
	t.Cleanup(func() {
		resetConfigForTest(nil)
		stickyDepMu.Lock()
		stickyDep = nil
		stickyDepMu.Unlock()
		globalHealth.mu.Lock()
		globalHealth.status = make(map[string]HealthStatus)
		globalHealth.mu.Unlock()
		resetRuntimeForTest()
	})
	stickyDepMu.Lock()
	stickyDep = nil
	stickyDepMu.Unlock()
	globalHealth.mu.Lock()
	globalHealth.status = make(map[string]HealthStatus)
	globalHealth.mu.Unlock()
	resetRuntimeForTest()
}

func TestPrepareDeploymentsPromotesPreferredDeployment(t *testing.T) {
	resetFallbackPlanningStateForTest(t, &Config{
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
			"dep-a": {Enabled: true, Pool: "paid_high", RealModel: "a", Priority: 1, QualityTier: "high"},
			"dep-b": {Enabled: true, Pool: "paid_high", RealModel: "b", Priority: 2, QualityTier: "low"},
			"dep-c": {Enabled: true, Pool: "paid_high", RealModel: "c", Priority: 3, QualityTier: "medium"},
		},
	})

	deployments, err := PrepareDeploymentsForRequest("cct/high", RequestCapabilities{})
	if err != nil {
		t.Fatalf("PrepareDeploymentsForRequest failed: %v", err)
	}
	if len(deployments) != 3 {
		t.Fatalf("expected 3 deployments, got %d", len(deployments))
	}
	if deployments[0].ID != "dep-b" {
		t.Fatalf("expected preferred deployment dep-b first, got %s", deployments[0].ID)
	}
}

func TestPrepareDeploymentsAppliesCapabilityFilter(t *testing.T) {
	resetFallbackPlanningStateForTest(t, &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/vision": {
				Enabled:  true,
				Strategy: StrategyQualityFirst,
				Pools:    []string{"mixed"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"text-only":  {Enabled: true, Pool: "mixed", RealModel: "text", Priority: 1, SupportsVision: false},
			"vision-cap": {Enabled: true, Pool: "mixed", RealModel: "vision", Priority: 2, SupportsVision: true},
		},
	})

	plan, err := PrepareDeploymentPlanForRequest("cct/vision", RequestCapabilities{Vision: true})
	if err != nil {
		t.Fatalf("PrepareDeploymentPlanForRequest failed: %v", err)
	}
	if plan.CapabilityBefore != 2 || plan.CapabilityAfter != 1 {
		t.Fatalf("expected capability filter 2 -> 1, got %d -> %d", plan.CapabilityBefore, plan.CapabilityAfter)
	}
	if len(plan.Deployments) != 1 || plan.Deployments[0].ID != "vision-cap" {
		t.Fatalf("expected only vision-cap, got %#v", plan.Deployments)
	}
}

func TestPrepareDeploymentsKeepsAIHordeForStreamRequests(t *testing.T) {
	aihorde := BuiltinFreeProviders["aihorde"]
	resetFallbackPlanningStateForTest(t, &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {
				Enabled:  true,
				Strategy: StrategyFreeFirst,
				Pools:    []string{"free"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"free:aihorde-001122ff": {
				ID:             "free:aihorde-001122ff",
				Enabled:        true,
				Pool:           "free",
				RealModel:      "aihorde/free",
				Priority:       1,
				SupportsStream: aihorde.SupportsStream,
			},
		},
	})

	plan, err := PrepareDeploymentPlanForRequest("cct/free", RequestCapabilities{Stream: true})
	if err != nil {
		t.Fatalf("PrepareDeploymentPlanForRequest failed: %v", err)
	}
	if plan.CapabilityBefore != 1 || plan.CapabilityAfter != 1 {
		t.Fatalf("expected aihorde to survive stream capability filter, got %d -> %d", plan.CapabilityBefore, plan.CapabilityAfter)
	}
	if len(plan.Deployments) != 1 || plan.Deployments[0].ID != "free:aihorde-001122ff" {
		t.Fatalf("expected aihorde deployment candidate, got %#v", plan.Deployments)
	}
}

func TestPrepareDeploymentsKeepsKiloAlternativeForCapabilityRequest(t *testing.T) {
	depID := "free:kilo-001122ff"
	resetFallbackPlanningStateForTest(t, &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {Enabled: true, Strategy: StrategyFreeFirst, Pools: []string{"free"}},
		},
		Deployments: map[string]DeploymentConfig{
			depID: {ID: depID, Enabled: true, Pool: "free", RealModel: "kilo/text:free", SupportsTools: false},
		},
	})
	cleanupCatalog := setupFreeProviderCatalogStoreTestDB(t)
	t.Cleanup(cleanupCatalog)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	tools := true
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID: depID, Provider: "kilo", Source: ModelFetchKiloFree,
		Models: []FreeModelCatalogEntry{
			{ID: "kilo/text:free", SupportsTools: boolPtr(false)},
			{ID: "kilo/tools:free", SupportsTools: &tools},
		},
		SelectedModel: "kilo/text:free", LastAttemptAt: now, LastSuccessAt: now,
	}); err != nil {
		t.Fatalf("saveFreeProviderCatalogSuccess: %v", err)
	}

	plan, err := PrepareDeploymentPlanForRequest("cct/free", RequestCapabilities{Tools: true})
	if err != nil {
		t.Fatalf("PrepareDeploymentPlanForRequest failed: %v", err)
	}
	if plan.CapabilityBefore != 1 || plan.CapabilityAfter != 1 || len(plan.Deployments) != 1 || plan.Deployments[0].ID != depID {
		t.Fatalf("Kilo alternative was dropped from deployment plan: %#v", plan)
	}
}

func TestPrepareDeploymentsPreservesStickyWhenNoPreferred(t *testing.T) {
	resetFallbackPlanningStateForTest(t, &Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/high": {
				Enabled:  true,
				Strategy: StrategyQualityFirst,
				Pools:    []string{"paid_high"},
			},
		},
		Deployments: map[string]DeploymentConfig{
			"sticky-low": {Enabled: true, Pool: "paid_high", RealModel: "low", Priority: 2, QualityTier: "low"},
			"high-q":     {Enabled: true, Pool: "paid_high", RealModel: "high", Priority: 1, QualityTier: "high"},
		},
	})
	SetStickyDeployment("cct/high", "sticky-low")

	deployments, err := PrepareDeploymentsForRequest("cct/high", RequestCapabilities{})
	if err != nil {
		t.Fatalf("PrepareDeploymentsForRequest failed: %v", err)
	}
	if len(deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(deployments))
	}
	if deployments[0].ID != "sticky-low" {
		t.Fatalf("expected sticky deployment to remain first, got %s", deployments[0].ID)
	}
}
