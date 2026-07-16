package fallback

import (
	"sync"
	"testing"
	"time"
)

func TestProviderRateLimitDegradationEpisodeTransitions(t *testing.T) {
	base := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		records []time.Time
		want    ProviderRateLimitDegradationSnapshot
	}{
		{
			name:    "first observation remains level zero",
			records: []time.Time{base},
			want: ProviderRateLimitDegradationSnapshot{
				EpisodeCount: 1,
			},
		},
		{
			name: "second post cooldown episode enters level one",
			records: []time.Time{
				base,
				base.Add(time.Minute + time.Nanosecond),
			},
			want: ProviderRateLimitDegradationSnapshot{
				Active:       true,
				Level:        1,
				EpisodeCount: 2,
				Reason:       providerRateLimitDegradationReason,
			},
		},
		{
			name: "same cooldown is deduplicated",
			records: []time.Time{
				base,
				base,
			},
			want: ProviderRateLimitDegradationSnapshot{
				EpisodeCount: 1,
			},
		},
		{
			name: "observation expires after fifteen minutes",
			records: []time.Time{
				base,
				base.Add(15*time.Minute + time.Nanosecond),
			},
			want: ProviderRateLimitDegradationSnapshot{
				EpisodeCount: 1,
			},
		},
		{
			name: "degradation caps at level three",
			records: []time.Time{
				base,
				base.Add(1*time.Minute + time.Nanosecond),
				base.Add(2*time.Minute + 2*time.Nanosecond),
				base.Add(3*time.Minute + 3*time.Nanosecond),
				base.Add(4*time.Minute + 4*time.Nanosecond),
			},
			want: ProviderRateLimitDegradationSnapshot{
				Active:       true,
				Level:        3,
				EpisodeCount: 5,
				Reason:       providerRateLimitDegradationReason,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &DeploymentRuntimeState{DeploymentID: "provider-rate-limit-transitions"}
			for _, now := range tt.records {
				recordProviderRateLimitEpisodeAtLocked(state, now, now.Add(time.Minute))
			}

			got := snapshotProviderRateLimitDegradationLocked(state)
			if got.Active != tt.want.Active || got.Level != tt.want.Level || got.EpisodeCount != tt.want.EpisodeCount || got.Reason != tt.want.Reason {
				t.Fatalf("snapshot = %#v, want active=%t level=%d episodes=%d reason=%q", got, tt.want.Active, tt.want.Level, tt.want.EpisodeCount, tt.want.Reason)
			}
		})
	}
}

func TestProviderRateLimitDegradationLevelZeroSuccessResetsObservation(t *testing.T) {
	base := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	state := &DeploymentRuntimeState{DeploymentID: "provider-rate-limit-success-reset"}

	recordProviderRateLimitEpisodeAtLocked(state, base, base.Add(time.Minute))
	recordProviderRateLimitDegradationSuccessAtLocked(state, base.Add(time.Second))
	recordProviderRateLimitEpisodeAtLocked(state, base.Add(2*time.Minute), base.Add(3*time.Minute))

	got := snapshotProviderRateLimitDegradationLocked(state)
	if got.Active || got.Level != 0 || got.EpisodeCount != 1 {
		t.Fatalf("snapshot = %#v, want a fresh level-zero observation", got)
	}
}

func TestProviderRateLimitDegradationRecoversOneLevelAfterThreeSuccesses(t *testing.T) {
	base := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	state := &DeploymentRuntimeState{DeploymentID: "provider-rate-limit-success-recovery"}
	for i := 0; i < 4; i++ {
		now := base.Add(time.Duration(i) * time.Minute)
		recordProviderRateLimitEpisodeAtLocked(state, now, now.Add(time.Minute-time.Nanosecond))
	}

	for i := 1; i <= 3; i++ {
		recordProviderRateLimitDegradationSuccessAtLocked(state, base.Add(5*time.Minute+time.Duration(i)*time.Second))
	}

	got := snapshotProviderRateLimitDegradationLocked(state)
	if got.Level != 2 || got.ConsecutiveRecoverySuccesses != 0 {
		t.Fatalf("snapshot = %#v, want level 2 with cleared recovery successes", got)
	}
}

func TestProviderRateLimitDegradationDecaysAfterTenMinutes(t *testing.T) {
	base := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	state := &DeploymentRuntimeState{DeploymentID: "provider-rate-limit-time-decay"}
	for i := 0; i < 3; i++ {
		now := base.Add(time.Duration(i) * time.Minute)
		recordProviderRateLimitEpisodeAtLocked(state, now, now.Add(time.Minute-time.Nanosecond))
	}

	decayProviderRateLimitDegradationAtLocked(state, base.Add(12*time.Minute))
	got := snapshotProviderRateLimitDegradationLocked(state)
	if got.Level != 1 {
		t.Fatalf("snapshot = %#v, want level 1 after time decay", got)
	}
	if got.NextRecoveryAt == nil || !got.NextRecoveryAt.Equal(base.Add(22*time.Minute)) {
		t.Fatalf("next recovery = %v, want %v", got.NextRecoveryAt, base.Add(22*time.Minute))
	}
}

func TestProviderRateLimitDegradationSnapshotUsesSafeReason(t *testing.T) {
	base := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	state := &DeploymentRuntimeState{DeploymentID: "provider-rate-limit-safe-reason"}
	recordProviderRateLimitEpisodeAtLocked(state, base, base.Add(time.Minute))
	recordProviderRateLimitEpisodeAtLocked(state, base.Add(time.Minute+time.Nanosecond), base.Add(2*time.Minute+time.Nanosecond))

	got := snapshotProviderRateLimitDegradationLocked(state)
	if got.Reason != "repeated rate limits" {
		t.Fatalf("reason = %q, want fixed safe reason", got.Reason)
	}
}

func TestResetProviderRateLimitDegradationResetsOnlyTarget(t *testing.T) {
	first := "provider-rate-limit-reset-first"
	second := "provider-rate-limit-reset-second"
	base := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)

	runtimeStatesMu.Lock()
	runtimeStates[first] = &DeploymentRuntimeState{DeploymentID: first, RateLimitEpisodeCount: 2, RateLimitDegradationLevel: 1, LastProviderRateLimitedAt: base}
	runtimeStates[second] = &DeploymentRuntimeState{DeploymentID: second, RateLimitEpisodeCount: 2, RateLimitDegradationLevel: 1, LastProviderRateLimitedAt: base}
	runtimeStatesMu.Unlock()
	t.Cleanup(func() {
		runtimeStatesMu.Lock()
		delete(runtimeStates, first)
		delete(runtimeStates, second)
		runtimeStatesMu.Unlock()
	})

	ResetProviderRateLimitDegradation(first)
	if got := SnapshotProviderRateLimitDegradation(first); got.Active || got.EpisodeCount != 0 {
		t.Fatalf("target snapshot = %#v, want reset", got)
	}
	if got := SnapshotProviderRateLimitDegradation(second); !got.Active || got.EpisodeCount != 2 {
		t.Fatalf("non-target snapshot = %#v, want unchanged", got)
	}
}

func TestProviderRateLimitDegradationConcurrentAccess(t *testing.T) {
	deploymentID := "provider-rate-limit-concurrent"
	ResetProviderRateLimitDegradation(deploymentID)
	t.Cleanup(func() {
		runtimeStatesMu.Lock()
		delete(runtimeStates, deploymentID)
		runtimeStatesMu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				RecordProviderRateLimitEpisode(deploymentID, time.Millisecond)
				RecordSuccess(deploymentID)
				_ = SnapshotProviderRateLimitDegradation(deploymentID)
				DecayRateLimitScores()
			}
		}()
	}
	wg.Wait()
}
