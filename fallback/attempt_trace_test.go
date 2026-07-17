package fallback

import (
	"sync"
	"testing"
	"time"
)

func TestRecentAttemptChainsOrderAndTerminalSuccess(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetRecentAttemptTrace()
	defer resetRecentAttemptTrace()

	base := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.UTC)
	events := []AttemptEvent{
		{CreatedAt: base, RequestID: "req-kilo", VirtualModel: "free/auto", Provider: "kilo", DeploymentID: "kilo", RealModel: "kilo/a", Outcome: AttemptOutcomeModelRateLimited, StatusCode: 429, ErrorCategory: "rate_limit", DurationMs: 10, PlanIndex: 1, UpstreamAttemptIndex: 1},
		{CreatedAt: base.Add(time.Second), RequestID: "req-kilo", VirtualModel: "free/auto", Provider: "kilo", DeploymentID: "kilo", RealModel: "kilo/b", Outcome: AttemptOutcomeSuccess, StatusCode: 200, DurationMs: 20, PlanIndex: 2, UpstreamAttemptIndex: 2},
		{CreatedAt: base.Add(2 * time.Second), RequestID: "req-later", VirtualModel: "free/auto", Provider: "other", DeploymentID: "other", RealModel: "other/a", Outcome: AttemptOutcomeFailure, StatusCode: 500, ErrorCategory: "temporary", DurationMs: 30, PlanIndex: 1, UpstreamAttemptIndex: 1},
	}
	for _, event := range events {
		if err := RecordAttemptEventIfWorthy(event); err != nil {
			t.Fatalf("RecordAttemptEventIfWorthy failed: %v", err)
		}
	}

	persisted, err := GetAttemptEvents(100)
	if err != nil {
		t.Fatalf("GetAttemptEvents failed: %v", err)
	}
	for _, event := range persisted {
		if event.Outcome == AttemptOutcomeSuccess {
			t.Fatal("success event must not be persisted to SQLite")
		}
	}

	chains := SnapshotRecentAttemptChains(20)
	if len(chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(chains))
	}
	if chains[0].RequestID != "req-later" || chains[1].RequestID != "req-kilo" {
		t.Fatalf("expected newest-first chain order req-later, req-kilo; got %q, %q", chains[0].RequestID, chains[1].RequestID)
	}
	if len(chains[1].Steps) != 2 {
		t.Fatalf("expected 2 steps in req-kilo chain, got %d", len(chains[1].Steps))
	}
	if chains[1].Steps[0].RealModel != "kilo/a" || chains[1].Steps[1].RealModel != "kilo/b" {
		t.Fatalf("expected kilo/a then kilo/b, got %q then %q", chains[1].Steps[0].RealModel, chains[1].Steps[1].RealModel)
	}
	if chains[1].Steps[1].Outcome != AttemptOutcomeSuccess {
		t.Fatalf("expected terminal success, got %s", chains[1].Steps[1].Outcome)
	}
}

func TestRecentAttemptChainsSortStepsByRouteOrder(t *testing.T) {
	base := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		events []AttemptEvent
		want   []string
	}{
		{
			name: "plan index wins over reversed timestamps",
			events: []AttemptEvent{
				{ID: 2, CreatedAt: base, RequestID: "req-plan-order", RealModel: "plan-two", PlanIndex: 2, UpstreamAttemptIndex: 1},
				{ID: 1, CreatedAt: base.Add(time.Second), RequestID: "req-plan-order", RealModel: "plan-one", PlanIndex: 1, UpstreamAttemptIndex: 1},
			},
			want: []string{"plan-one", "plan-two"},
		},
		{
			name: "upstream attempt index wins when timestamps are equal",
			events: []AttemptEvent{
				{ID: 2, CreatedAt: base, RequestID: "req-upstream-order", RealModel: "upstream-two", PlanIndex: 1, UpstreamAttemptIndex: 2},
				{ID: 1, CreatedAt: base, RequestID: "req-upstream-order", RealModel: "upstream-one", PlanIndex: 1, UpstreamAttemptIndex: 1},
			},
			want: []string{"upstream-one", "upstream-two"},
		},
		{
			name: "exact ties preserve insertion order",
			events: []AttemptEvent{
				{CreatedAt: base, RequestID: "req-stable-tie", RealModel: "first", PlanIndex: 1, UpstreamAttemptIndex: 1},
				{CreatedAt: base, RequestID: "req-stable-tie", RealModel: "second", PlanIndex: 1, UpstreamAttemptIndex: 1},
			},
			want: []string{"first", "second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRecentAttemptTrace()
			defer resetRecentAttemptTrace()
			for _, event := range tt.events {
				recordRecentAttempt(event)
			}

			chains := SnapshotRecentAttemptChains(1)
			if len(chains) != 1 || len(chains[0].Steps) != len(tt.want) {
				t.Fatalf("recent chains = %#v, want one chain with %d steps", chains, len(tt.want))
			}
			for index, wantModel := range tt.want {
				if got := chains[0].Steps[index].RealModel; got != wantModel {
					t.Fatalf("step %d model = %q, want %q; steps=%#v", index, got, wantModel, chains[0].Steps)
				}
			}
		})
	}
}

func TestRecentAttemptTraceBoundsAndLimits(t *testing.T) {
	resetRecentAttemptTrace()
	defer resetRecentAttemptTrace()

	base := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.UTC)
	for i := 0; i < recentAttemptEventLimit+1; i++ {
		recordRecentAttempt(AttemptEvent{CreatedAt: base.Add(time.Duration(i) * time.Second), RequestID: "req-bounds", VirtualModel: "free/auto", Outcome: AttemptOutcomeFailure})
	}

	recentAttemptTrace.RLock()
	retained := len(recentAttemptTrace.events)
	oldest := recentAttemptTrace.events[0].CreatedAt
	recentAttemptTrace.RUnlock()
	if retained != recentAttemptEventLimit {
		t.Fatalf("expected %d retained events, got %d", recentAttemptEventLimit, retained)
	}
	if !oldest.Equal(base.Add(time.Second)) {
		t.Fatalf("expected oldest retained event at %s, got %s", base.Add(time.Second), oldest)
	}
	if got := len(SnapshotRecentAttemptChains(20)[0].Steps); got != recentAttemptStepLimit {
		t.Fatalf("expected %d retained steps per chain, got %d", recentAttemptStepLimit, got)
	}

	resetRecentAttemptTrace()
	for i := 0; i < recentAttemptChainLimit+5; i++ {
		recordRecentAttempt(AttemptEvent{CreatedAt: base.Add(time.Duration(i) * time.Second), RequestID: string(rune('a' + i)), VirtualModel: "free/auto", Outcome: AttemptOutcomeFailure})
	}
	if got := len(SnapshotRecentAttemptChains(0)); got != recentAttemptChainLimit {
		t.Fatalf("expected default limit %d, got %d", recentAttemptChainLimit, got)
	}
	if got := len(SnapshotRecentAttemptChains(200)); got != recentAttemptChainLimit {
		t.Fatalf("expected capped limit %d, got %d", recentAttemptChainLimit, got)
	}
}

func TestRecentAttemptChainsSnapshotIsDeepCopy(t *testing.T) {
	resetRecentAttemptTrace()
	defer resetRecentAttemptTrace()

	recordRecentAttempt(AttemptEvent{CreatedAt: time.Now().UTC(), RequestID: "req-copy", VirtualModel: "free/auto", Provider: "kilo", Outcome: AttemptOutcomeSuccess})
	first := SnapshotRecentAttemptChains(1)
	first[0].Steps[0].Provider = "mutated"

	second := SnapshotRecentAttemptChains(1)
	if second[0].Steps[0].Provider != "kilo" {
		t.Fatalf("snapshot mutation leaked into trace: got %q", second[0].Steps[0].Provider)
	}
}

func TestRecentAttemptTraceConcurrentRecordSnapshotAndReset(t *testing.T) {
	resetRecentAttemptTrace()
	defer resetRecentAttemptTrace()

	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				recordRecentAttempt(AttemptEvent{CreatedAt: time.Now().UTC(), RequestID: "req-concurrent", VirtualModel: "free/auto", Provider: "kilo", Outcome: AttemptOutcomeFailure, PlanIndex: worker, UpstreamAttemptIndex: i})
			}
		}(worker)
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = SnapshotRecentAttemptChains(20)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			resetRecentAttemptTrace()
		}
	}()
	wg.Wait()
}
