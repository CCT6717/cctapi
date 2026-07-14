package fallback

import (
	"net/http"
	"testing"
	"time"
)

func setupFreeProviderModelPlanTest(t *testing.T, depID string, models []FreeModelCatalogEntry) {
	t.Helper()
	cleanupCatalog := setupFreeProviderCatalogStoreTestDB(t)
	t.Cleanup(cleanupCatalog)
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)

	if len(models) == 0 {
		return
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID:  depID,
		Provider:      "kilo",
		Source:        ModelFetchKiloFree,
		Models:        models,
		SelectedModel: models[0].ID,
		LastAttemptAt: now,
		LastSuccessAt: now,
	}); err != nil {
		t.Fatalf("saveFreeProviderCatalogSuccess: %v", err)
	}
}

func TestPrepareDeploymentModelPlanKeepsNonKiloSingleAndNonRotatableAttempt(t *testing.T) {
	dep := DeploymentConfig{ID: "manual", RealModel: "manual-model", SupportsTools: true}

	plan := PrepareDeploymentModelPlan(dep, RequestCapabilities{Tools: true})

	if plan.Rotatable || plan.CompatibleCount != 1 || plan.CoolingCount != 0 || len(plan.Attempts) != 1 {
		t.Fatalf("unexpected non-Kilo plan: %#v", plan)
	}
	attempt := plan.Attempts[0]
	if attempt.Deployment != dep || attempt.Rotatable || attempt.ModelIndex != 0 || attempt.ModelCount != 1 {
		t.Fatalf("unexpected non-Kilo attempt: %#v", attempt)
	}
}

func TestPrepareDeploymentModelPlanPromotesConfiguredKiloModelBeforeCatalogOrder(t *testing.T) {
	depID := "free:kilo-001122ff"
	setupFreeProviderModelPlanTest(t, depID, []FreeModelCatalogEntry{
		{ID: "kilo/a:free"},
		{ID: "kilo/b:free"},
		{ID: "kilo/c:free"},
	})
	dep := DeploymentConfig{ID: depID, RealModel: "kilo/b:free"}

	plan := PrepareDeploymentModelPlan(dep, RequestCapabilities{})

	if !plan.Rotatable || plan.CompatibleCount != 3 || len(plan.Attempts) != 3 {
		t.Fatalf("unexpected Kilo plan: %#v", plan)
	}
	wantModels := []string{"kilo/b:free", "kilo/a:free", "kilo/c:free"}
	for i, wantModel := range wantModels {
		attempt := plan.Attempts[i]
		if attempt.Deployment.RealModel != wantModel || attempt.ProviderName != "kilo" || !attempt.Rotatable || attempt.ModelIndex != i || attempt.ModelCount != len(wantModels) {
			t.Fatalf("attempt %d = %#v, want model %q at %d/%d", i, attempt, wantModel, i, len(wantModels))
		}
	}
	if dep.RealModel != "kilo/b:free" {
		t.Fatalf("planner mutated source deployment: %#v", dep)
	}
}

func TestKiloModelCandidatesUseProviderDefaultsForNilCatalogMetadata(t *testing.T) {
	kilo := BuiltinFreeProviders["kilo"]
	dep := DeploymentConfig{
		ID:             "free:kilo-001122ff",
		RealModel:      "kilo/configured:free",
		SupportsTools:  true,
		SupportsStream: false,
		ContextLength:  512,
	}

	candidates := kiloModelCandidates(dep, []FreeModelCatalogEntry{
		{ID: dep.RealModel},
		{ID: "kilo/alternative:free"},
	})

	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(candidates))
	}
	alternative := candidates[1]
	if alternative.RealModel != "kilo/alternative:free" ||
		alternative.SupportsTools != kilo.SupportsTools ||
		alternative.SupportsStream != kilo.SupportsStream ||
		alternative.ContextLength != kilo.ContextLength {
		t.Fatalf("nil catalog metadata inherited configured capabilities: %#v; Kilo defaults: %#v", alternative, kilo)
	}
}

func TestPrepareDeploymentModelPlanUsesKiloDefaultsForNilAlternativeMetadata(t *testing.T) {
	depID := "free:kilo-001122ff"
	configuredModel := "kilo/configured:free"
	alternativeModel := "kilo/alternative:free"
	tools := true
	stream := false
	smallContext := 512
	setupFreeProviderModelPlanTest(t, depID, []FreeModelCatalogEntry{
		{ID: configuredModel, SupportsTools: &tools, SupportsStream: &stream, ContextLength: &smallContext},
		{ID: alternativeModel},
	})
	dep := DeploymentConfig{
		ID:             depID,
		RealModel:      configuredModel,
		SupportsTools:  true,
		SupportsStream: false,
		ContextLength:  smallContext,
	}

	tests := []struct {
		name      string
		caps      RequestCapabilities
		wantModel string
	}{
		{name: "tools", caps: RequestCapabilities{Tools: true}, wantModel: configuredModel},
		{name: "stream", caps: RequestCapabilities{Stream: true}, wantModel: alternativeModel},
		{name: "context", caps: RequestCapabilities{MaxTokens: 1024}, wantModel: alternativeModel},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := PrepareDeploymentModelPlan(dep, tc.caps)
			if plan.CompatibleCount != 1 || len(plan.Attempts) != 1 || plan.Attempts[0].Deployment.RealModel != tc.wantModel {
				t.Fatalf("plan = %#v, want only %q", plan, tc.wantModel)
			}
		})
	}
}

func TestPrepareDeploymentModelPlanRemovesCoolingKiloModelsAfterCompatibilityCount(t *testing.T) {
	depID := "free:kilo-001122ff"
	setupFreeProviderModelPlanTest(t, depID, []FreeModelCatalogEntry{{ID: "kilo/a:free"}, {ID: "kilo/b:free"}})
	MarkFreeProviderModelRateLimited(depID, "kilo/a:free", "rate limited", RelayCooldownInput{StatusCode: http.StatusTooManyRequests})

	plan := PrepareDeploymentModelPlan(DeploymentConfig{ID: depID, RealModel: "kilo/a:free"}, RequestCapabilities{})

	if plan.CompatibleCount != 2 || plan.CoolingCount != 1 || len(plan.Attempts) != 1 {
		t.Fatalf("unexpected cooled Kilo plan: %#v", plan)
	}
	attempt := plan.Attempts[0]
	if attempt.Deployment.RealModel != "kilo/b:free" || attempt.ModelIndex != 0 || attempt.ModelCount != 1 || attempt.CompatibleCount != 2 || attempt.CoolingCount != 1 {
		t.Fatalf("unexpected cooled Kilo attempt: %#v", attempt)
	}
}

func TestPrepareDeploymentModelPlanExcludesKiloModelsWithoutRequestedCapabilities(t *testing.T) {
	trueValue := true
	falseValue := false
	shortContext := 1024
	longContext := 8192
	tests := []struct {
		name      string
		caps      RequestCapabilities
		models    []FreeModelCatalogEntry
		wantModel string
	}{
		{
			name: "tools", caps: RequestCapabilities{Tools: true}, wantModel: "kilo/tools:free",
			models: []FreeModelCatalogEntry{{ID: "kilo/text:free", SupportsTools: &falseValue}, {ID: "kilo/tools:free", SupportsTools: &trueValue}},
		},
		{
			name: "JSON", caps: RequestCapabilities{JSON: true}, wantModel: "kilo/json:free",
			models: []FreeModelCatalogEntry{{ID: "kilo/text:free", SupportsJSON: &falseValue}, {ID: "kilo/json:free", SupportsJSON: &trueValue}},
		},
		{
			name: "vision", caps: RequestCapabilities{Vision: true}, wantModel: "kilo/vision:free",
			models: []FreeModelCatalogEntry{{ID: "kilo/text:free", SupportsVision: &falseValue}, {ID: "kilo/vision:free", SupportsVision: &trueValue}},
		},
		{
			name: "context", caps: RequestCapabilities{MaxTokens: 2048}, wantModel: "kilo/long:free",
			models: []FreeModelCatalogEntry{{ID: "kilo/short:free", ContextLength: &shortContext}, {ID: "kilo/long:free", ContextLength: &longContext}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depID := "free:kilo-001122ff"
			setupFreeProviderModelPlanTest(t, depID, tc.models)

			plan := PrepareDeploymentModelPlan(DeploymentConfig{ID: depID, RealModel: tc.models[0].ID}, tc.caps)

			if plan.CompatibleCount != 1 || len(plan.Attempts) != 1 || plan.Attempts[0].Deployment.RealModel != tc.wantModel {
				t.Fatalf("capability plan = %#v, want only %q", plan, tc.wantModel)
			}
		})
	}
}

func TestPrepareDeploymentModelPlanFallsBackToConfiguredKiloModelWithoutCatalog(t *testing.T) {
	depID := "free:kilo-001122ff"
	setupFreeProviderModelPlanTest(t, depID, nil)
	dep := DeploymentConfig{ID: depID, RealModel: "kilo/configured:free", SupportsJSON: true}

	plan := PrepareDeploymentModelPlan(dep, RequestCapabilities{JSON: true})

	if plan.Rotatable || plan.CompatibleCount != 1 || plan.CoolingCount != 0 || len(plan.Attempts) != 1 {
		t.Fatalf("unexpected catalog-less plan: %#v", plan)
	}
	attempt := plan.Attempts[0]
	if attempt.Deployment != dep || attempt.ProviderName != "kilo" || attempt.Rotatable || attempt.ModelIndex != 0 || attempt.ModelCount != 1 {
		t.Fatalf("unexpected catalog-less attempt: %#v", attempt)
	}
}

func TestDeploymentModelAttemptReportsRemainingModels(t *testing.T) {
	tests := []struct {
		name      string
		attempt   DeploymentModelAttempt
		hasNext   bool
		remaining int
	}{
		{
			name:      "first of three rotatable models",
			attempt:   DeploymentModelAttempt{Rotatable: true, ModelIndex: 0, ModelCount: 3},
			hasNext:   true,
			remaining: 2,
		},
		{
			name:      "last rotatable model",
			attempt:   DeploymentModelAttempt{Rotatable: true, ModelIndex: 2, ModelCount: 3},
			remaining: 0,
		},
		{
			name:      "single non-rotatable model",
			attempt:   DeploymentModelAttempt{ModelIndex: 0, ModelCount: 1},
			remaining: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attempt.HasNextModel(); got != tc.hasNext {
				t.Fatalf("HasNextModel() = %v, want %v", got, tc.hasNext)
			}
			if got := tc.attempt.RemainingModelAttempts(); got != tc.remaining {
				t.Fatalf("RemainingModelAttempts() = %d, want %d", got, tc.remaining)
			}
		})
	}
}
