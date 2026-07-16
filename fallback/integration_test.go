package fallback

import (
	"testing"
	"time"
)

// Integration-style tests covering the three-tier gateway core: capability
// filter, quota pre-check, strategy sort, and cooldown policy. These exercise
// the real config + runtime-state paths end-to-end without a live server.

func resetRuntimeForTest() {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	runtimeStates = make(map[string]*DeploymentRuntimeState)
}

func TestIntegrationCapabilityFilterDropsNonVision(t *testing.T) {
	deps := []DeploymentConfig{
		{ID: "vision-cap", Pool: "free", SupportsVision: true, ContextLength: 32000},
		{ID: "text-only", Pool: "free", SupportsVision: false, ContextLength: 32000},
	}
	caps := RequestCapabilities{Vision: true, MaxTokens: 100}
	got := FilterByCapability(deps, caps)
	if len(got) != 1 || got[0].ID != "vision-cap" {
		t.Fatalf("expected only vision-cap, got %v", got)
	}
}

func TestIntegrationCapabilityFilterContextLength(t *testing.T) {
	deps := []DeploymentConfig{
		{ID: "small", Pool: "free", ContextLength: 8000},
		{ID: "large", Pool: "free", ContextLength: 128000},
	}
	caps := RequestCapabilities{MaxTokens: 50000}
	got := FilterByCapability(deps, caps)
	if len(got) != 1 || got[0].ID != "large" {
		t.Fatalf("expected only large-context deployment, got %v", got)
	}
}

func TestIntegrationCapabilityFilterSelectsCatalogModelForTools(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	toolsFalse := false
	toolsTrue := true
	depID := "free:kilo-e3b0c442"
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID: depID,
		Provider:     "kilo",
		Source:       ModelFetchKiloFree,
		Models: []FreeModelCatalogEntry{
			{ID: "kilo/text:free", SupportsTools: &toolsFalse},
			{ID: "kilo/tools:free", SupportsTools: &toolsTrue},
		},
		SelectedModel: "kilo/text:free",
		LastAttemptAt: now,
		LastSuccessAt: now,
	}); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	got := FilterByCapability([]DeploymentConfig{{
		ID: depID, RealModel: "kilo/text:free", Pool: "free", SupportsStream: true,
	}}, RequestCapabilities{Tools: true})
	if len(got) != 1 {
		t.Fatalf("expected one catalog-backed deployment, got %#v", got)
	}
	if got[0].RealModel != "kilo/tools:free" || !got[0].SupportsTools {
		t.Fatalf("tool request selected wrong model: %#v", got[0])
	}
}

func TestIntegrationCapabilityFilterPreservesManualCatalogOverride(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	toolsTrue := true
	depID := "free:kilo-e3b0c442"
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID:  depID,
		Provider:      "kilo",
		Source:        ModelFetchKiloFree,
		Models:        []FreeModelCatalogEntry{{ID: "kilo/tools:free", SupportsTools: &toolsTrue}},
		SelectedModel: "kilo/tools:free",
		LastAttemptAt: now,
		LastSuccessAt: now,
	}); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	got := FilterByCapability([]DeploymentConfig{{
		ID: depID, RealModel: "operator/manual-model", Pool: "free", SupportsTools: false,
	}}, RequestCapabilities{Tools: true})
	if len(got) != 0 {
		t.Fatalf("manual model override must not be silently replaced: %#v", got)
	}
}

func TestIntegrationQuotaPreCheckBlocksRPM(t *testing.T) {
	resetRuntimeForTest()
	dep := DeploymentConfig{ID: "groq", RPMLimit: 30}
	state := GetRuntimeState("groq")
	state.MinuteRequests = 30 // at limit
	if PassQuotaCheck(dep, state, 100) {
		t.Fatalf("expected RPM pre-check to block at limit")
	}
	state.MinuteRequests = 29
	if !PassQuotaCheck(dep, state, 100) {
		t.Fatalf("expected RPM pre-check to allow under limit")
	}
}

func TestIntegrationQuotaPreCheckBlocksTPD(t *testing.T) {
	resetRuntimeForTest()
	dep := DeploymentConfig{ID: "cerebras", TPDLimit: 500000}
	state := GetRuntimeState("cerebras")
	state.DayTokens = 499900
	if PassQuotaCheck(dep, state, 200) {
		t.Fatalf("expected TPD pre-check to block when 200 tokens would exceed")
	}
	if !PassQuotaCheck(dep, state, 50) {
		t.Fatalf("expected TPD pre-check to allow 50 tokens")
	}
}

func TestIntegrationQuotaPreCheckResetsExpiredWindows(t *testing.T) {
	resetRuntimeForTest()
	dep := DeploymentConfig{ID: "groq", RPMLimit: 1, RPDLimit: 1, TPMLimit: 100, TPDLimit: 100}
	state := GetRuntimeState("groq")
	state.MinuteRequests = 1
	state.DayRequests = 1
	state.MinuteTokens = 100
	state.DayTokens = 100
	state.LastResetMinute = time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	state.LastResetDay = truncateToDay(time.Now().AddDate(0, 0, -1))

	if !PassQuotaCheck(dep, state, 50) {
		t.Fatalf("expected expired quota windows to reset before pre-check")
	}

	snap := SnapshotRuntimeState("groq")
	if snap.MinuteRequests != 0 || snap.DayRequests != 0 || snap.MinuteTokens != 0 || snap.DayTokens != 0 {
		t.Fatalf("expected expired counters reset, got req %d/%d tokens %d/%d",
			snap.MinuteRequests, snap.DayRequests, snap.MinuteTokens, snap.DayTokens)
	}
}

func TestIntegrationRecordUsageIncrementsCounters(t *testing.T) {
	resetRuntimeForTest()
	RecordUsage("groq", 1500)
	RecordUsage("groq", 500)
	snap := SnapshotRuntimeState("groq")
	if snap.MinuteRequests != 2 || snap.MinuteTokens != 2000 {
		t.Fatalf("expected 2 reqs / 2000 tokens, got %d/%d", snap.MinuteRequests, snap.MinuteTokens)
	}
}

func TestIntegrationRateLimitScoreCapsAtTen(t *testing.T) {
	resetRuntimeForTest()
	for i := 0; i < 15; i++ {
		RecordFailure("groq", "429", true)
	}
	snap := SnapshotRuntimeState("groq")
	if snap.RateLimitScore != 10 {
		t.Fatalf("expected rate limit score capped at 10, got %d", snap.RateLimitScore)
	}
	if snap.FailureCount != 15 {
		t.Fatalf("expected 15 failures recorded, got %d", snap.FailureCount)
	}
}

func TestIntegrationFreeFirstSortPrefersHeadroom(t *testing.T) {
	t.Cleanup(func() { resetConfigForTest(nil) })
	resetRuntimeForTest()

	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			"cct/free": {Enabled: true, Strategy: StrategyFreeFirst, Pools: []string{"free"}},
		},
		Deployments: map[string]DeploymentConfig{
			"depleted": {ID: "depleted", Enabled: true, Pool: "free", RealModel: "m1", Priority: 1, DailyLimitTokens: 100000},
			"fresh":    {ID: "fresh", Enabled: true, Pool: "free", RealModel: "m2", Priority: 2, DailyLimitTokens: 100000},
		},
	})

	// Mark "depleted" with maximum rate-limit penalty so free_first avoids it.
	for i := 0; i < 10; i++ {
		RecordFailure("depleted", "429", true)
	}
	// fresh stays clean

	sorted := SortByStrategy([]DeploymentConfig{
		{ID: "depleted", Pool: "free", RealModel: "m1", Priority: 1, DailyLimitTokens: 100000},
		{ID: "fresh", Pool: "free", RealModel: "m2", Priority: 2, DailyLimitTokens: 100000},
	}, StrategyFreeFirst)

	if sorted[0].ID != "fresh" {
		t.Fatalf("expected free_first to prefer fresh deployment, got %s first (rate_limit_score depleted=%d fresh=%d)",
			sorted[0].ID,
			SnapshotRuntimeState("depleted").RateLimitScore,
			SnapshotRuntimeState("fresh").RateLimitScore)
	}
}

func TestIntegrationCostFirstSortPrefersFree(t *testing.T) {
	t.Cleanup(func() { resetConfigForTest(nil) })
	resetRuntimeForTest()

	sorted := SortByStrategy([]DeploymentConfig{
		{ID: "paid", Pool: "paid_high", RealModel: "gpt4", CostTier: "paid"},
		{ID: "free", Pool: "free", RealModel: "llama", CostTier: "free"},
		{ID: "cheap", Pool: "cheap", RealModel: "haiku", CostTier: "cheap"},
	}, StrategyCostFirst)

	if sorted[0].ID != "free" {
		t.Fatalf("expected cost_first to rank free first, got %s", sorted[0].ID)
	}
}

func TestIntegrationQualityFirstSortPrefersHigh(t *testing.T) {
	resetRuntimeForTest()
	sorted := SortByStrategy([]DeploymentConfig{
		{ID: "low-q", Pool: "free", RealModel: "m1", QualityTier: "low"},
		{ID: "high-q", Pool: "paid_high", RealModel: "m2", QualityTier: "high"},
		{ID: "med-q", Pool: "cheap", RealModel: "m3", QualityTier: "medium"},
	}, StrategyQualityFirst)

	if sorted[0].ID != "high-q" {
		t.Fatalf("expected quality_first to rank high first, got %s", sorted[0].ID)
	}
}

func setProviderRateLimitDegradationLevelForTest(deploymentID string, level int) {
	runtimeStatesMu.Lock()
	defer runtimeStatesMu.Unlock()
	runtimeStates[deploymentID] = &DeploymentRuntimeState{
		DeploymentID:              deploymentID,
		RateLimitDegradationLevel: level,
	}
}

func TestIntegrationProviderDegradationPenaltyLevels(t *testing.T) {
	resetRuntimeForTest()
	t.Cleanup(resetRuntimeForTest)

	for _, tt := range []struct {
		name  string
		level int
		want  float64
	}{
		{name: "healthy", level: 0, want: 0},
		{name: "level one", level: 1, want: 25},
		{name: "level two", level: 2, want: 50},
		{name: "level three", level: 3, want: 75},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deploymentID := "provider-degradation-" + tt.name
			setProviderRateLimitDegradationLevelForTest(deploymentID, tt.level)

			if got := providerRateLimitDegradationScorePenalty(deploymentID); got != tt.want {
				t.Fatalf("penalty = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntegrationProviderDegradationDemotesHealthyPeersAcrossStrategies(t *testing.T) {
	resetRuntimeForTest()
	t.Cleanup(resetRuntimeForTest)

	for _, tt := range []struct {
		name     string
		strategy string
		dep      DeploymentConfig
	}{
		{
			name:     "quality first",
			strategy: StrategyQualityFirst,
			dep:      DeploymentConfig{Pool: "paid_high", QualityTier: "high", CostTier: "paid"},
		},
		{
			name:     "cost first",
			strategy: StrategyCostFirst,
			dep:      DeploymentConfig{Pool: "free", QualityTier: "medium", CostTier: "free"},
		},
		{
			name:     "free first",
			strategy: StrategyFreeFirst,
			dep:      DeploymentConfig{Pool: "free", QualityTier: "medium", CostTier: "free"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			healthy := tt.dep
			healthy.ID = "healthy-" + tt.strategy
			healthy.RealModel = "healthy-model"

			degraded := tt.dep
			degraded.ID = "degraded-" + tt.strategy
			degraded.RealModel = "degraded-model"
			setProviderRateLimitDegradationLevelForTest(degraded.ID, 1)

			sorted := SortByStrategy([]DeploymentConfig{degraded, healthy}, tt.strategy)
			if len(sorted) != 2 {
				t.Fatalf("candidate count = %d, want 2", len(sorted))
			}
			if sorted[0].ID != healthy.ID {
				t.Fatalf("first candidate = %s, want healthy peer %s", sorted[0].ID, healthy.ID)
			}
			if sorted[1].ID != degraded.ID {
				t.Fatalf("degraded candidate must remain eligible, got %#v", sorted)
			}
		})
	}
}

func TestIntegrationProviderDegradationPenalizesCalculateScore(t *testing.T) {
	resetRuntimeForTest()
	t.Cleanup(resetRuntimeForTest)

	healthy := DeploymentConfig{ID: "healthy-admin-score", Priority: 1}
	degraded := DeploymentConfig{ID: "degraded-admin-score", Priority: 1}
	setProviderRateLimitDegradationLevelForTest(degraded.ID, 1)

	healthyScore := CalculateScore(healthy, nil, DefaultScoreWeights())
	degradedScore := CalculateScore(degraded, nil, DefaultScoreWeights())
	if healthyScore-degradedScore != 25 {
		t.Fatalf("admin score difference = %v, want 25", healthyScore-degradedScore)
	}
}

func TestIntegrationProviderDegradationPenalizesCalculateScoreWithHistory(t *testing.T) {
	resetRuntimeForTest()
	t.Cleanup(resetRuntimeForTest)

	healthy := DeploymentConfig{ID: "healthy-admin-score-history", Priority: 1}
	degraded := DeploymentConfig{ID: "degraded-admin-score-history", Priority: 1}
	state := &DeploymentState{RequestCount: 10, SuccessCount: 8, ErrorCount: 2}
	setProviderRateLimitDegradationLevelForTest(degraded.ID, 1)

	healthyScore := CalculateScore(healthy, state, DefaultScoreWeights())
	degradedScore := CalculateScore(degraded, state, DefaultScoreWeights())
	if healthyScore-degradedScore != 25 {
		t.Fatalf("historical admin score difference = %v, want 25", healthyScore-degradedScore)
	}
}

func TestIntegrationPreferredDeploymentPromotesDegradedCandidateAfterAutomaticSorting(t *testing.T) {
	t.Cleanup(func() { resetConfigForTest(nil) })
	resetRuntimeForTest()
	t.Cleanup(resetRuntimeForTest)

	const virtualModel = "cct/high"
	const preferredID = "preferred-degraded"
	resetConfigForTest(&Config{
		Enabled: true,
		VirtualModels: map[string]VirtualModelConfig{
			virtualModel: {
				Enabled:             true,
				Strategy:            StrategyQualityFirst,
				Pools:               []string{"paid_high"},
				PreferredDeployment: preferredID,
			},
		},
		Deployments: map[string]DeploymentConfig{
			"healthy":     {ID: "healthy", Enabled: true, Pool: "paid_high", RealModel: "healthy-model", QualityTier: "high"},
			preferredID: {ID: preferredID, Enabled: true, Pool: "paid_high", RealModel: "preferred-model", QualityTier: "high"},
		},
	})
	setProviderRateLimitDegradationLevelForTest(preferredID, 1)

	plan, err := PrepareDeploymentPlanForRequest(virtualModel, RequestCapabilities{})
	if err != nil {
		t.Fatalf("prepare deployment plan: %v", err)
	}
	if len(plan.Deployments) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(plan.Deployments))
	}
	if plan.Deployments[0].ID != preferredID {
		t.Fatalf("first candidate = %s, want preferred deployment %s", plan.Deployments[0].ID, preferredID)
	}
}

func TestIntegrationCooldownPolicyDurations(t *testing.T) {
	// Verify the policy durations are sensible without touching the DB-backed
	// cooldown state machine (which requires a live DB connection).
	if DefaultCooldownPolicy.RateLimitShort != 60*1e9 {
		t.Fatalf("expected rate-limit short cooldown 60s, got %v", DefaultCooldownPolicy.RateLimitShort)
	}
	if DefaultCooldownPolicy.RateLimitDay != 24*60*60*1e9 {
		t.Fatalf("expected rate-limit day cooldown 24h, got %v", DefaultCooldownPolicy.RateLimitDay)
	}
	if DefaultCooldownPolicy.TemporaryShort != 30*1e9 {
		t.Fatalf("expected temporary short cooldown 30s, got %v", DefaultCooldownPolicy.TemporaryShort)
	}
}

func TestIntegrationApplyCooldownNoPanicsWithoutDB(t *testing.T) {
	// Without a DB, ApplyCooldown must not panic; it silently returns 0.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ApplyCooldown panicked without DB: %v", r)
		}
	}()
	_ = ApplyCooldown("no-db-dep", "quota", ErrorCategoryQuota)
	_ = ApplyCooldown("no-db-dep", "rate", ErrorCategoryRateLimit)
	_ = ApplyCooldown("no-db-dep", "temp", ErrorCategoryTemporary)
}

func TestIntegrationIsCCTVirtualModelRecognisesThreeTiers(t *testing.T) {
	for _, m := range []string{"cct/high", "cct/low", "cct/free"} {
		if !IsCCTVirtualModel(m) {
			t.Errorf("expected %s to be a CCT virtual model", m)
		}
	}
	for _, m := range []string{"high/auto", "cct/auto", "gpt-4"} {
		if IsCCTVirtualModel(m) {
			t.Errorf("expected %s NOT to be a CCT virtual model", m)
		}
	}
}

func TestIntegrationHealthStatusDefaultsToUnknown(t *testing.T) {
	if got := GetHealthStatus("never-checked"); got != HealthUnknown {
		t.Fatalf("expected unknown for never-checked deployment, got %s", got)
	}
	if !IsDeploymentHealthy("never-checked") {
		t.Fatalf("unknown deployments should be treated as healthy (allowed to route)")
	}
}
