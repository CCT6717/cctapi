package fallback

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAttemptObservabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
	})

	if err := db.AutoMigrate(&AttemptEvent{}); err != nil {
		t.Fatalf("migrate attempt events: %v", err)
	}
	return db
}

func createAttemptObservabilityEvents(t *testing.T, db *gorm.DB, events []AttemptEvent) {
	t.Helper()
	for _, event := range events {
		if err := db.Create(&event).Error; err != nil {
			t.Fatalf("create attempt event: %v", err)
		}
	}
}

func TestAttemptObservabilityClassifiesOneHourFailuresAndSkips(t *testing.T) {
	db := setupAttemptObservabilityTestDB(t)
	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)

	createAttemptObservabilityEvents(t, db, []AttemptEvent{
		{CreatedAt: now.Add(-time.Hour).Add(-time.Nanosecond), Provider: "before", DeploymentID: "before", RealModel: "before", Outcome: AttemptOutcomeFailure, ErrorCategory: "temporary", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-time.Hour), Provider: "boundary", DeploymentID: "boundary", RealModel: "boundary", Outcome: AttemptOutcomeFailure, ErrorCategory: "Bearer sk-secret password raw_upstream_body content_preview injected error text", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-30 * time.Minute), Provider: "rate", DeploymentID: "rate", RealModel: "rate", Outcome: AttemptOutcomeModelRateLimited, ErrorCategory: "rate_limit", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-20 * time.Minute), Provider: "non-fallbackable", DeploymentID: "non-fallbackable", RealModel: "non-fallbackable", Outcome: AttemptOutcomeNonFallbackable, ErrorCategory: "client", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-10 * time.Minute), Provider: "capability", DeploymentID: "capability", RealModel: "capability", Outcome: AttemptOutcomeModelCapabilityFalsePositive, ErrorCategory: "model_access", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-5 * time.Minute), Provider: "skip", DeploymentID: "skip", RealModel: "skip", Outcome: AttemptOutcomeSkippedUnavailable, ErrorCategory: "temporary", UpstreamAttemptIndex: 0},
		{CreatedAt: now.Add(-4 * time.Minute), Provider: "not-real", DeploymentID: "not-real", RealModel: "not-real", Outcome: AttemptOutcomeFailure, ErrorCategory: "temporary", UpstreamAttemptIndex: 0},
		{CreatedAt: now.Add(-3 * time.Minute), Provider: "success", DeploymentID: "success", RealModel: "success", Outcome: AttemptOutcomeSuccess, ErrorCategory: "none", UpstreamAttemptIndex: 1},
	})

	snapshot, err := snapshotAttemptObservabilityAt(now)
	if err != nil {
		t.Fatalf("snapshot attempt observability: %v", err)
	}
	if snapshot.FailureWindowSeconds != int64(time.Hour/time.Second) {
		t.Fatalf("expected one-hour window, got %d seconds", snapshot.FailureWindowSeconds)
	}
	if snapshot.FailureEventCount != 4 {
		t.Fatalf("expected 4 real failures inside the boundary, got %d", snapshot.FailureEventCount)
	}
	if snapshot.SkipEventCount != 1 {
		t.Fatalf("expected 1 skip event, got %d", snapshot.SkipEventCount)
	}
	if snapshot.RecentChainScope != "process" {
		t.Fatalf("expected process recent-chain scope, got %q", snapshot.RecentChainScope)
	}
	if len(snapshot.Outcomes) != 5 {
		t.Fatalf("expected four failure outcomes and one skip outcome, got %d", len(snapshot.Outcomes))
	}
	if !containsAggregateKey(snapshot.TopDeployments, "boundary") {
		t.Fatal("expected event exactly at the one-hour boundary in deployment aggregates")
	}
	if containsAggregateKey(snapshot.TopDeployments, "before") {
		t.Fatal("did not expect event before the one-hour boundary in deployment aggregates")
	}
	for _, aggregate := range [][]AttemptAggregateItem{
		snapshot.TopDeployments,
		snapshot.TopProviders,
		snapshot.TopModels,
	} {
		if containsAggregateKey(aggregate, "skip") {
			t.Fatal("skip event must not contribute to failure aggregates")
		}
	}
	if containsAggregateKey(snapshot.ErrorCategories, "temporary") {
		t.Fatal("skip event must not contribute to error-category aggregates")
	}
	if skipOutcome, ok := findAggregateItem(snapshot.Outcomes, string(AttemptOutcomeSkippedUnavailable)); !ok || skipOutcome.Count != 1 {
		t.Fatalf("expected skipped outcome count 1, got %#v", snapshot.Outcomes)
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	for _, sentinel := range []string{"Bearer", "sk-", "password", "raw_upstream_body", "content_preview", "injected error text"} {
		if strings.Contains(string(payload), sentinel) {
			t.Fatalf("snapshot leaked unsafe value %q: %s", sentinel, payload)
		}
	}
}

func TestAttemptObservabilityReturnsStableTopFiveAndEmptySlices(t *testing.T) {
	db := setupAttemptObservabilityTestDB(t)
	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)

	empty, err := snapshotAttemptObservabilityAt(now)
	if err != nil {
		t.Fatalf("empty snapshot: %v", err)
	}
	for _, aggregate := range [][]AttemptAggregateItem{
		empty.TopDeployments,
		empty.TopProviders,
		empty.TopModels,
		empty.ErrorCategories,
		empty.Outcomes,
	} {
		if aggregate == nil {
			t.Fatal("expected empty aggregate slices to be non-nil")
		}
	}

	createAttemptObservabilityEvents(t, db, []AttemptEvent{
		{CreatedAt: now.Add(-10 * time.Minute), Provider: "count-first", DeploymentID: "count-first", RealModel: "count-first", Outcome: AttemptOutcomeFailure, ErrorCategory: "temporary", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-9 * time.Minute), Provider: "count-first", DeploymentID: "count-first", RealModel: "count-first", Outcome: AttemptOutcomeFailure, ErrorCategory: "temporary", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-time.Minute), Provider: "z-recent", DeploymentID: "z-recent", RealModel: "z-recent", Outcome: AttemptOutcomeFailure, ErrorCategory: "temporary", UpstreamAttemptIndex: 1},
		{CreatedAt: now.Add(-2 * time.Minute), Provider: "a-older", DeploymentID: "a-older", RealModel: "a-older", Outcome: AttemptOutcomeFailure, ErrorCategory: "temporary", UpstreamAttemptIndex: 1},
	})
	for _, provider := range []string{"provider-f", "provider-e", "provider-d", "provider-c", "provider-b", "provider-a"} {
		createAttemptObservabilityEvents(t, db, []AttemptEvent{{
			CreatedAt:            now.Add(-3 * time.Minute),
			Provider:             provider,
			DeploymentID:         provider,
			RealModel:            provider,
			Outcome:              AttemptOutcomeFailure,
			ErrorCategory:        "temporary",
			UpstreamAttemptIndex: 1,
		}})
	}

	snapshot, err := snapshotAttemptObservabilityAt(now)
	if err != nil {
		t.Fatalf("snapshot attempt observability: %v", err)
	}
	if len(snapshot.TopProviders) != 5 {
		t.Fatalf("expected top-five provider limit, got %d", len(snapshot.TopProviders))
	}
	for index, expected := range []string{"count-first", "z-recent", "a-older", "provider-a", "provider-b"} {
		if snapshot.TopProviders[index].Key != expected {
			t.Fatalf("expected stable provider order at index %d to be %q, got %q", index, expected, snapshot.TopProviders[index].Key)
		}
	}
	if snapshot.TopProviders[0].Count != 2 {
		t.Fatalf("expected highest-count provider first, got count %d", snapshot.TopProviders[0].Count)
	}
}

func TestAttemptObservabilitySanitizesRecentChains(t *testing.T) {
	resetRecentAttemptTrace()
	t.Cleanup(resetRecentAttemptTrace)
	setupAttemptObservabilityTestDB(t)
	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	unsafeCategory := "Bearer sk-secret password raw_upstream_body content_preview injected error text"

	if err := RecordAttemptEventIfWorthy(AttemptEvent{
		CreatedAt:            now,
		RequestID:            "request-safe-chain",
		VirtualModel:         "free/auto",
		Provider:             "provider",
		DeploymentID:         "deployment",
		RealModel:            "model",
		Outcome:              AttemptOutcomeFailure,
		ErrorCategory:        unsafeCategory,
		UpstreamAttemptIndex: 1,
	}); err != nil {
		t.Fatalf("record unsafe attempt event: %v", err)
	}

	snapshot, err := snapshotAttemptObservabilityAt(now)
	if err != nil {
		t.Fatalf("snapshot attempt observability: %v", err)
	}
	if len(snapshot.RecentChains) != 1 || len(snapshot.RecentChains[0].Steps) != 1 {
		t.Fatalf("expected one recent chain with one step, got %#v", snapshot.RecentChains)
	}
	if snapshot.RecentChains[0].Steps[0].ErrorCategory != "unknown" {
		t.Fatalf("expected safe chain category unknown, got %q", snapshot.RecentChains[0].Steps[0].ErrorCategory)
	}

	payload, err := json.Marshal(snapshot.RecentChains)
	if err != nil {
		t.Fatalf("marshal recent chains: %v", err)
	}
	for _, sentinel := range []string{"Bearer", "sk-", "password", "raw_upstream_body", "content_preview", "injected error text"} {
		if strings.Contains(string(payload), sentinel) {
			t.Fatalf("recent chains leaked unsafe value %q: %s", sentinel, payload)
		}
	}
}

func containsAggregateKey(items []AttemptAggregateItem, key string) bool {
	_, ok := findAggregateItem(items, key)
	return ok
}

func findAggregateItem(items []AttemptAggregateItem, key string) (AttemptAggregateItem, bool) {
	for _, item := range items {
		if item.Key == key {
			return item, true
		}
	}
	return AttemptAggregateItem{}, false
}
