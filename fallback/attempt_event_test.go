package fallback

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAttemptEventSuccessNotPersisted(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-success-001"
	ResetAttemptMetrics(deploymentID)
	defer ResetAttemptMetrics(deploymentID)
	event := AttemptEvent{
		RequestID:            "req-success-001",
		VirtualModel:         "test/model",
		Provider:             "test",
		DeploymentID:         deploymentID,
		RealModel:            "real-model",
		Outcome:              AttemptOutcomeSuccess,
		StatusCode:           200,
		ErrorCategory:        "none",
		DurationMs:           100,
		StreamWritten:        true,
		PlanIndex:            1,
		UpstreamAttemptIndex: 1,
	}

	// recordFallbackAttempt records success metrics before applying the
	// persistence policy.
	RecordAttemptSuccess(deploymentID, event.DurationMs)
	if err := RecordAttemptEventIfWorthy(event); err != nil {
		t.Fatalf("RecordAttemptEventIfWorthy failed: %v", err)
	}

	// Verify not persisted in DB
	events, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected success event NOT persisted, got %d events", len(events))
	}

	// Verify in-memory metrics recorded success
	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.SuccessCount != 1 {
		t.Fatalf("expected success count 1, got %d", metrics.SuccessCount)
	}
	if metrics.TotalDurationMs != 100 {
		t.Fatalf("expected total duration 100, got %d", metrics.TotalDurationMs)
	}
}

func TestRecordAttemptEventIfWorthyNormalizesTimestampBeforeTraceAndPersistence(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetRecentAttemptTrace()
	defer resetRecentAttemptTrace()

	requestID := "req-zero-created-at"
	before := time.Now().UTC()
	if err := RecordAttemptEventIfWorthy(AttemptEvent{
		RequestID:            requestID,
		VirtualModel:         "test/model",
		Provider:             "test",
		DeploymentID:         "free:test-zero-created-at",
		RealModel:            "real-model",
		Outcome:              AttemptOutcomeFailure,
		StatusCode:           500,
		ErrorCategory:        ErrorCategoryTemporary.String(),
		PlanIndex:            1,
		UpstreamAttemptIndex: 1,
	}); err != nil {
		t.Fatalf("RecordAttemptEventIfWorthy failed: %v", err)
	}
	after := time.Now().UTC()

	chains := SnapshotRecentAttemptChains(1)
	if len(chains) != 1 || len(chains[0].Steps) != 1 {
		t.Fatalf("recent chains = %#v, want one recorded step", chains)
	}
	traceCreatedAt := chains[0].Steps[0].CreatedAt
	if traceCreatedAt.IsZero() || traceCreatedAt.Before(before) || traceCreatedAt.After(after) {
		t.Fatalf("trace created_at = %s, want normalized timestamp between %s and %s", traceCreatedAt, before, after)
	}

	persisted, err := GetAttemptEventsByRequestID(requestID, 10)
	if err != nil {
		t.Fatalf("GetAttemptEventsByRequestID failed: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("persisted events = %#v, want one failure", persisted)
	}
	if !persisted[0].CreatedAt.Equal(traceCreatedAt) {
		t.Fatalf("persisted created_at = %s, trace created_at = %s; want one normalized timestamp", persisted[0].CreatedAt, traceCreatedAt)
	}
}

func TestAttemptEventFailurePersistedAndMetrics(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-failure-001"
	ResetAttemptMetrics(deploymentID)
	defer ResetAttemptMetrics(deploymentID)
	ResetErrorCategoryCounters()
	defer ResetErrorCategoryCounters()
	event := AttemptEvent{
		RequestID:            "req-failure-001",
		VirtualModel:         "test/model",
		Provider:             "test",
		DeploymentID:         deploymentID,
		RealModel:            "real-model",
		Outcome:              AttemptOutcomeFailure,
		StatusCode:           500,
		ErrorCategory:        ErrorCategoryTemporary.String(),
		DurationMs:           200,
		StreamWritten:        false,
		PlanIndex:            2,
		UpstreamAttemptIndex: 1,
	}

	// recordFallbackAttempt records failure metrics and the category counter
	// alongside the persisted diagnostic event.
	RecordAttemptFailure(deploymentID, event.DurationMs)
	RecordErrorCategoryCounter(ErrorCategoryTemporary)
	if err := RecordAttemptEventIfWorthy(event); err != nil {
		t.Fatalf("RecordAttemptEventIfWorthy failed: %v", err)
	}

	// Verify persisted in DB
	events, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 persisted event, got %d", len(events))
	}
	if events[0].Outcome != AttemptOutcomeFailure {
		t.Fatalf("expected outcome failure, got %s", events[0].Outcome)
	}
	if events[0].StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", events[0].StatusCode)
	}

	// Verify failure metrics
	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.FailureCount != 1 {
		t.Fatalf("expected failure count 1, got %d", metrics.FailureCount)
	}

	// Verify error category counter
	counters := SnapshotErrorCategoryCounters()
	if counters["temporary"] != 1 {
		t.Fatalf("expected temporary counter 1, got %d", counters["temporary"])
	}
}

func TestAttemptEventModelRateLimitedNotPenalizeProvider(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-kilo-001"
	// Ensure deployment state exists
	if _, err := EnsureDeploymentState(deploymentID, todayString()); err != nil {
		t.Fatalf("EnsureDeploymentState failed: %v", err)
	}

	event := AttemptEvent{
		RequestID:            "req-model-rl-001",
		VirtualModel:         "test/model",
		Provider:             "kilo",
		DeploymentID:         deploymentID,
		RealModel:            "kilo/a:free",
		Outcome:              AttemptOutcomeModelRateLimited,
		StatusCode:           429,
		ErrorCategory:        ErrorCategoryRateLimit.String(),
		DurationMs:           50,
		StreamWritten:        false,
		PlanIndex:            1,
		UpstreamAttemptIndex: 1,
	}

	if err := RecordAttemptEventIfWorthy(event); err != nil {
		t.Fatalf("RecordAttemptEventIfWorthy failed: %v", err)
	}
	// Also record attempt failure and category counter (as relay.go does)
	RecordAttemptFailure(deploymentID, 50)
	RecordErrorCategoryCounter(ErrorCategoryRateLimit)

	// Verify persisted in DB
	events, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 persisted event, got %d", len(events))
	}
	if events[0].Outcome != AttemptOutcomeModelRateLimited {
		t.Fatalf("expected outcome model_rate_limited, got %s", events[0].Outcome)
	}

	// Verify attempt failure metrics recorded
	metrics := SnapshotAttemptMetrics(deploymentID)
	if metrics.FailureCount != 1 {
		t.Fatalf("expected attempt failure count 1, got %d", metrics.FailureCount)
	}

	// Verify provider NOT penalized (no deployment error count)
	_, requestCount, errorCount, err := GetDeploymentStats(deploymentID)
	if err != nil {
		t.Fatalf("GetDeploymentStats failed: %v", err)
	}
	if requestCount != 0 || errorCount != 0 {
		t.Fatalf("provider was penalized: requests=%d errors=%d, want 0/0", requestCount, errorCount)
	}
}

func TestAttemptEventSkipIndices(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-skip-001"

	// Skip events: upstreamAttemptIndex = 0
	skip1 := AttemptEvent{
		RequestID: "req-skip-001", VirtualModel: "test/model",
		Provider: "test", DeploymentID: deploymentID, RealModel: "real-1",
		Outcome: AttemptOutcomeSkippedUnavailable, StatusCode: 503,
		ErrorCategory: "temporary", DurationMs: 0, StreamWritten: false,
		PlanIndex: 1, UpstreamAttemptIndex: 0,
	}
	skip2 := AttemptEvent{
		RequestID: "req-skip-001", VirtualModel: "test/model",
		Provider: "test", DeploymentID: deploymentID, RealModel: "real-2",
		Outcome: AttemptOutcomeSkippedQuota, StatusCode: 429,
		ErrorCategory: "rate_limit", DurationMs: 0, StreamWritten: false,
		PlanIndex: 2, UpstreamAttemptIndex: 0,
	}
	// Real upstream call: upstreamAttemptIndex = 1
	real1 := AttemptEvent{
		RequestID: "req-skip-001", VirtualModel: "test/model",
		Provider: "test", DeploymentID: deploymentID, RealModel: "real-3",
		Outcome: AttemptOutcomeFailure, StatusCode: 500,
		ErrorCategory: "temporary", DurationMs: 150, StreamWritten: false,
		PlanIndex: 3, UpstreamAttemptIndex: 1,
	}

	for _, e := range []AttemptEvent{skip1, skip2, real1} {
		if err := RecordAttemptEventIfWorthy(e); err != nil {
			t.Fatalf("RecordAttemptEventIfWorthy failed: %v", err)
		}
	}

	events, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 persisted events, got %d", len(events))
	}

	// Verify indices: events are returned in reverse time order
	// We need to check all three have correct plan indices
	planIndices := make(map[int]bool)
	upstreamIndices := make(map[int]bool)
	for _, e := range events {
		planIndices[e.PlanIndex] = true
		upstreamIndices[e.UpstreamAttemptIndex] = true
	}
	if !planIndices[1] || !planIndices[2] || !planIndices[3] {
		t.Fatalf("expected plan indices 1,2,3, got %v", planIndices)
	}
	// upstreamAttemptIndex: 0,0,1
	if !upstreamIndices[0] || !upstreamIndices[1] {
		t.Fatalf("expected upstream indices 0 and 1, got %v", upstreamIndices)
	}

	// Verify no conflict: skip indices (0) and real indices (1) coexist
	skipCount := 0
	realCount := 0
	for _, e := range events {
		if e.UpstreamAttemptIndex == 0 {
			skipCount++
		} else {
			realCount++
		}
	}
	if skipCount != 2 || realCount != 1 {
		t.Fatalf("expected 2 skips + 1 real, got %d skips + %d real", skipCount, realCount)
	}
}

func TestAttemptEventQueryLimits(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-limit-001"
	// Insert 10 events
	for i := 0; i < 10; i++ {
		event := AttemptEvent{
			RequestID:            fmt.Sprintf("req-limit-%d", i),
			VirtualModel:         "test/model",
			Provider:             "test",
			DeploymentID:         deploymentID,
			RealModel:            "real-model",
			Outcome:              AttemptOutcomeFailure,
			StatusCode:           500,
			ErrorCategory:        "temporary",
			DurationMs:           int64(i * 10),
			StreamWritten:        false,
			PlanIndex:            i + 1,
			UpstreamAttemptIndex: 1,
		}
		if err := RecordAttemptEvent(event); err != nil {
			t.Fatalf("RecordAttemptEvent failed: %v", err)
		}
	}

	// limit=0 should default to 100
	events, err := GetAttemptEvents(0)
	if err != nil {
		t.Fatalf("GetAttemptEvents(0) failed: %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("GetAttemptEvents(0): expected 10 events, got %d", len(events))
	}

	// limit=50 should return 50 (but we only have 10)
	events, err = GetAttemptEvents(50)
	if err != nil {
		t.Fatalf("GetAttemptEvents(50) failed: %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("GetAttemptEvents(50): expected 10 events, got %d", len(events))
	}

	// limit=600 should cap to 500
	// We only have 10, so should return 10
	// But let's verify the limit logic by checking there's no error
	events, err = GetAttemptEvents(600)
	if err != nil {
		t.Fatalf("GetAttemptEvents(600) failed: %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("GetAttemptEvents(600): expected 10 events, got %d", len(events))
	}

	// Verify by request ID limit
	eventsByReq, err := GetAttemptEventsByRequestID("req-limit-0", 0)
	if err != nil {
		t.Fatalf("GetAttemptEventsByRequestID failed: %v", err)
	}
	if len(eventsByReq) != 1 {
		t.Fatalf("expected 1 event by request ID, got %d", len(eventsByReq))
	}

	// Verify by deployment ID limit
	eventsByDep, err := GetAttemptEventsByDeploymentID(deploymentID, 5)
	if err != nil {
		t.Fatalf("GetAttemptEventsByDeploymentID failed: %v", err)
	}
	if len(eventsByDep) != 5 {
		t.Fatalf("expected 5 events by deployment ID, got %d", len(eventsByDep))
	}
}

func TestAttemptEventRetentionCleanup(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-retention-001"
	now := time.Now().UTC()

	// Insert old event (beyond retention)
	oldEvent := AttemptEvent{
		RequestID:            "req-old-001",
		VirtualModel:         "test/model",
		Provider:             "test",
		DeploymentID:         deploymentID,
		RealModel:            "real-model",
		Outcome:              AttemptOutcomeFailure,
		StatusCode:           500,
		ErrorCategory:        "temporary",
		DurationMs:           100,
		StreamWritten:        false,
		PlanIndex:            1,
		UpstreamAttemptIndex: 1,
		CreatedAt:            now.Add(-AttemptEventRetention - time.Hour),
	}
	if err := RecordAttemptEvent(oldEvent); err != nil {
		t.Fatalf("RecordAttemptEvent old failed: %v", err)
	}

	// Insert recent event (within retention)
	recentEvent := AttemptEvent{
		RequestID:            "req-recent-001",
		VirtualModel:         "test/model",
		Provider:             "test",
		DeploymentID:         deploymentID,
		RealModel:            "real-model",
		Outcome:              AttemptOutcomeFailure,
		StatusCode:           500,
		ErrorCategory:        "temporary",
		DurationMs:           100,
		StreamWritten:        false,
		PlanIndex:            2,
		UpstreamAttemptIndex: 1,
		CreatedAt:            now.Add(-time.Hour),
	}
	if err := RecordAttemptEvent(recentEvent); err != nil {
		t.Fatalf("RecordAttemptEvent recent failed: %v", err)
	}

	// The existing application history cleanup loop must own attempt-event
	// retention instead of starting another background goroutine.
	runCleanup()

	// Verify only recent event remains
	events, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 remaining event, got %d", len(events))
	}
	if events[0].RequestID != "req-recent-001" {
		t.Fatalf("expected recent event to remain, got %s", events[0].RequestID)
	}
}

func TestAttemptEventStoreReinitializesOnDatabaseSwitch(t *testing.T) {
	originalDB := model.DB
	defer func() {
		model.DB = originalDB
	}()

	// First DB
	db1, err := gorm.Open(sqlite.Open("file:test_event_1?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open first DB: %v", err)
	}
	model.DB = db1

	if err := InitAttemptEventStore(); err != nil {
		t.Fatalf("InitAttemptEventStore on first DB failed: %v", err)
	}

	// Verify table exists on first DB
	if !db1.Migrator().HasTable(&AttemptEvent{}) {
		t.Fatal("expected attempt_events table on first DB")
	}

	// Second DB
	db2, err := gorm.Open(sqlite.Open("file:test_event_2?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open second DB: %v", err)
	}
	model.DB = db2

	// Switching the active database must trigger migration automatically.
	if err := InitAttemptEventStore(); err != nil {
		t.Fatalf("InitAttemptEventStore on second DB failed: %v", err)
	}

	// Verify table exists on second DB
	if !db2.Migrator().HasTable(&AttemptEvent{}) {
		t.Fatal("expected attempt_events table on second DB")
	}

	// Verify first DB still has its table (not corrupted by second init)
	if !db1.Migrator().HasTable(&AttemptEvent{}) {
		t.Fatal("first DB table should still exist")
	}
}

func TestAttemptEventConcurrentRecordNoRace(t *testing.T) {
	cleanupDB := setupAttemptEventConcurrentTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-concurrent-001"
	const numGoroutines = 20
	const eventsPerGoroutine = 50

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := AttemptEvent{
					RequestID:            fmt.Sprintf("req-concurrent-%d-%d", id, j),
					VirtualModel:         "test/model",
					Provider:             "test",
					DeploymentID:         deploymentID,
					RealModel:            "real-model",
					Outcome:              AttemptOutcomeFailure,
					StatusCode:           500,
					ErrorCategory:        "temporary",
					DurationMs:           int64(j),
					StreamWritten:        false,
					PlanIndex:            j + 1,
					UpstreamAttemptIndex: 1,
				}
				if err := RecordAttemptEvent(event); err != nil {
					t.Errorf("RecordAttemptEvent failed: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()

	var persisted int64
	if err := model.DB.Model(&AttemptEvent{}).Count(&persisted).Error; err != nil {
		t.Fatalf("count attempt events: %v", err)
	}
	expected := int64(numGoroutines * eventsPerGoroutine)
	if persisted != expected {
		t.Fatalf("expected %d persisted events, got %d", expected, persisted)
	}

	events, err := GetAttemptEvents(1000)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 500 {
		t.Fatalf("expected capped query result of 500 events, got %d", len(events))
	}
}

func setupAttemptEventConcurrentTestDB(t *testing.T) func() {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open concurrent attempt-event DB: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get concurrent attempt-event SQL DB: %v", err)
	}
	// SQLite :memory: is connection-local. Keep one connection so concurrent
	// callers share the migrated schema while the race detector still observes
	// concurrent access to the Go state around it.
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	return func() {
		model.DB = originalDB
		_ = sqlDB.Close()
	}
}

func TestAttemptEventNoSensitiveData(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:test-sensitive-001"
	event := AttemptEvent{
		RequestID:            "req-sensitive-001",
		VirtualModel:         "test/model",
		Provider:             "test",
		DeploymentID:         deploymentID,
		RealModel:            "real-model",
		Outcome:              AttemptOutcomeFailure,
		StatusCode:           500,
		ErrorCategory:        "temporary",
		DurationMs:           100,
		StreamWritten:        false,
		PlanIndex:            1,
		UpstreamAttemptIndex: 1,
	}

	if err := RecordAttemptEvent(event); err != nil {
		t.Fatalf("RecordAttemptEvent failed: %v", err)
	}

	events, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]

	// Verify no API key or token in any field
	sensitiveStrings := []string{"sk-", "api_key", "token", "secret", "password", "credential"}
	for _, s := range sensitiveStrings {
		if containsAny(t, e.RequestID, s) || containsAny(t, e.VirtualModel, s) ||
			containsAny(t, e.Provider, s) || containsAny(t, e.DeploymentID, s) ||
			containsAny(t, e.RealModel, s) || containsAny(t, string(e.Outcome), s) ||
			containsAny(t, e.ErrorCategory, s) {
			t.Fatalf("field contains sensitive string %q", s)
		}
	}

	// Verify raw error message is not stored (ErrorCategory is a fixed category, not raw message)
	if e.ErrorCategory != "temporary" && e.ErrorCategory != "none" &&
		e.ErrorCategory != "client" && e.ErrorCategory != "quota" &&
		e.ErrorCategory != "rate_limit" && e.ErrorCategory != "model_access" {
		t.Fatalf("error category %q is not a fixed allowed category", e.ErrorCategory)
	}
}

func containsAny(t *testing.T, s, substr string) bool {
	t.Helper()
	return len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)
}
