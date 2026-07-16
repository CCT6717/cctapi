package fallback

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetFreeProviderModelRuntimeForTest() {
	freeProviderModelRuntimeMu.Lock()
	freeProviderModelRuntime = make(map[string]map[string]*freeProviderModelRuntimeEntry)
	freeProviderModelRuntimeMu.Unlock()
	freeProviderModelCapabilityFPMu.Lock()
	freeProviderModelCapabilityFP = make(map[string]map[string]*freeProviderModelCapabilityFalsePositive)
	capabilityFPDuration = 30 * time.Minute
	freeProviderModelCapabilityFPMu.Unlock()
	freeProviderModelRuntimeNow = time.Now
}

func TestFreeProviderModelCapabilityFalsePositiveExpiresAndResets(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	freeProviderModelRuntimeNow = func() time.Time { return now }

	MarkFreeProviderModelCapabilityFalsePositive("free:kilo-test", "model-a", "tools")
	if !IsFreeProviderModelCapabilityFalsePositive("free:kilo-test", "model-a", "tools") {
		t.Fatal("active tools false-positive was not recorded")
	}
	if IsFreeProviderModelCapabilityFalsePositive("free:kilo-test", "model-a", "json") {
		t.Fatal("tools false-positive affected another capability")
	}

	now = now.Add(31 * time.Minute)
	if IsFreeProviderModelCapabilityFalsePositive("free:kilo-test", "model-a", "tools") {
		t.Fatal("expired tools false-positive remained active")
	}
	freeProviderModelCapabilityFPMu.RLock()
	_, deploymentExists := freeProviderModelCapabilityFP["free:kilo-test"]
	freeProviderModelCapabilityFPMu.RUnlock()
	if deploymentExists {
		t.Fatal("expired tools false-positive was not removed from runtime state")
	}

	MarkFreeProviderModelCapabilityFalsePositive("free:kilo-test", "model-a", "tools")
	ResetFreeProviderModelCapabilityFalsePositive("free:kilo-test", "")
	if IsFreeProviderModelCapabilityFalsePositive("free:kilo-test", "model-a", "tools") {
		t.Fatal("deployment reset did not clear tools false-positive")
	}
}

func TestFreeProviderModelCapabilityFalsePositiveConcurrentAccess(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			modelID := "model-" + string(rune('a'+i%3))
			for j := 0; j < 100; j++ {
				MarkFreeProviderModelCapabilityFalsePositive("free:kilo-test", modelID, "tools")
				_ = IsFreeProviderModelCapabilityFalsePositive("free:kilo-test", modelID, "tools")
				ResetFreeProviderModelCapabilityFalsePositive("free:kilo-test", modelID)
			}
		}(i)
	}
	wg.Wait()
}

func TestFreeProviderModelRuntimeRateLimitDurations(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	freeProviderModelRuntimeNow = func() time.Time { return now }

	tests := []struct {
		name       string
		retryAfter *int
		want       time.Duration
	}{
		{name: "retry after", retryAfter: runtimeIntPtr(120), want: 120 * time.Second},
		{name: "retry after is clamped", retryAfter: runtimeIntPtr(301), want: 300 * time.Second},
		{name: "default cooldown", want: 60 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFreeProviderModelRuntimeForTest()
			got := MarkFreeProviderModelRateLimited("free:kilo-test", tc.name, "rate limited", RelayCooldownInput{
				Category: ErrorCategoryRateLimit, StatusCode: http.StatusTooManyRequests,
				RetryAfterSeconds: tc.retryAfter, Attempt: 1,
			})
			if got != tc.want {
				t.Fatalf("cooldown = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestFreeProviderModelRuntimeRateLimitExpires(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	freeProviderModelRuntimeNow = func() time.Time { return now }

	retryAfter := 120
	got := MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", "rate limited", RelayCooldownInput{
		Category: ErrorCategoryRateLimit, StatusCode: http.StatusTooManyRequests,
		RetryAfterSeconds: &retryAfter, Attempt: 1,
	})
	if got != 120*time.Second || !IsFreeProviderModelCooling("free:kilo-test", "model-a") {
		t.Fatalf("unexpected cooldown: %s", got)
	}
	now = now.Add(121 * time.Second)
	if IsFreeProviderModelCooling("free:kilo-test", "model-a") {
		t.Fatal("expired model cooldown remained active")
	}
}

func TestFreeProviderModelRuntimeSanitizesRateLimitReason(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)

	tests := []string{
		"  upstream\t429   Bearer sk-secret-token-value  ",
		"api_key=secret-value; retry after 30 seconds",
		`{"error":"sk-secret-value"}`,
		"x-api-key: secret-value",
	}
	for _, reason := range tests {
		MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", reason, RelayCooldownInput{})
		snapshot := SnapshotFreeProviderModelRuntime("free:kilo-test")
		if len(snapshot.Models) != 1 {
			t.Fatalf("unexpected snapshot: %+v", snapshot)
		}
		got := snapshot.Models[0].Reason
		if got != "rate limited" {
			t.Fatalf("rate-limit reason = %q, want fixed safe category", got)
		}
		resetFreeProviderModelRuntimeForTest()
	}

	longReason := strings.Repeat(" noisy reason ", 32)
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", longReason, RelayCooldownInput{})
	got := SnapshotFreeProviderModelRuntime("free:kilo-test").Models[0].Reason
	if got != "rate limited" {
		t.Fatalf("long rate-limit reason = %q, want fixed safe category", got)
	}
}

func TestFreeProviderModelRuntimeSnapshotPrunesExpiredUnsuccessfulModels(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	freeProviderModelRuntimeNow = func() time.Time { return now }

	MarkFreeProviderModelRateLimited("free:kilo-test", "model-expired", "rate limited", RelayCooldownInput{})
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-success", "rate limited", RelayCooldownInput{})
	RecordFreeProviderModelSuccess("free:kilo-test", "model-success")
	now = now.Add(61 * time.Second)

	snapshot := SnapshotFreeProviderModelRuntime("free:kilo-test")
	if len(snapshot.Models) != 1 || snapshot.Models[0].ModelID != "model-success" {
		t.Fatalf("unexpected pruned snapshot: %+v", snapshot)
	}
	if snapshot.LastSuccessfulModel != "model-success" || snapshot.Models[0].SuccessCount != 1 {
		t.Fatalf("successful model history was not preserved: %+v", snapshot)
	}
}

func TestFreeProviderModelRuntimeSnapshotRemovesEmptyDeploymentAfterPruning(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	freeProviderModelRuntimeNow = func() time.Time { return now }
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-expired", "rate limited", RelayCooldownInput{})
	now = now.Add(61 * time.Second)

	snapshot := SnapshotFreeProviderModelRuntime("free:kilo-test")
	if len(snapshot.Models) != 0 {
		t.Fatalf("expired model remained in snapshot: %+v", snapshot)
	}
	freeProviderModelRuntimeMu.RLock()
	_, exists := freeProviderModelRuntime["free:kilo-test"]
	freeProviderModelRuntimeMu.RUnlock()
	if exists {
		t.Fatal("empty deployment runtime was not removed")
	}
}

func TestFreeProviderModelRuntimeSuccessResetsCooldown(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", "rate limited", RelayCooldownInput{})
	RecordFreeProviderModelSuccess("free:kilo-test", "model-a")

	snapshot := SnapshotFreeProviderModelRuntime("free:kilo-test")
	if len(snapshot.Models) != 1 || snapshot.Models[0].CooldownActive {
		t.Fatalf("success did not reset cooldown: %+v", snapshot)
	}
	if snapshot.Models[0].Consecutive429Count != 0 || snapshot.Models[0].SuccessCount != 1 || snapshot.Models[0].FailureCount != 1 {
		t.Fatalf("unexpected counters: %+v", snapshot.Models[0])
	}
	if snapshot.LastSuccessfulModel != "model-a" {
		t.Fatalf("last successful model = %q", snapshot.LastSuccessfulModel)
	}
}

func TestFreeProviderModelRuntimeSnapshotIsSortedAndDeepCopied(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	freeProviderModelRuntimeNow = func() time.Time { return now }
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-b", "first", RelayCooldownInput{})
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", "second", RelayCooldownInput{})

	snapshot := SnapshotFreeProviderModelRuntime("free:kilo-test")
	if len(snapshot.Models) != 2 || snapshot.Models[0].ModelID != "model-a" || snapshot.Models[1].ModelID != "model-b" {
		t.Fatalf("models are not sorted: %+v", snapshot.Models)
	}
	*snapshot.Models[0].CooldownUntil = now.Add(24 * time.Hour)
	snapshot.Models[0].Reason = "mutated"
	snapshot.Models = snapshot.Models[:1]

	again := SnapshotFreeProviderModelRuntime("free:kilo-test")
	if len(again.Models) != 2 || again.Models[0].Reason == "mutated" || again.Models[0].CooldownUntil.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("snapshot leaked mutable state: %+v", again)
	}
}

func TestFreeProviderModelRuntimeDeploymentReset(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	MarkFreeProviderModelRateLimited("free:kilo-test", "model-a", "rate limited", RelayCooldownInput{})
	MarkFreeProviderModelRateLimited("free:other", "model-b", "rate limited", RelayCooldownInput{})

	ResetFreeProviderModelRuntime("free:kilo-test")
	if got := SnapshotFreeProviderModelRuntime("free:kilo-test"); len(got.Models) != 0 {
		t.Fatalf("deployment runtime was not reset: %+v", got)
	}
	if !IsFreeProviderModelCooling("free:other", "model-b") {
		t.Fatal("reset affected another deployment")
	}
}

func TestFreeProviderModelRuntimeConcurrentReadersAndWriters(t *testing.T) {
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)
	freeProviderModelRuntimeNow = func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			modelID := "model-" + string(rune('a'+i%3))
			for j := 0; j < 100; j++ {
				MarkFreeProviderModelRateLimited("free:kilo-test", modelID, "rate limited", RelayCooldownInput{})
				RecordFreeProviderModelSuccess("free:kilo-test", modelID)
				_ = IsFreeProviderModelCooling("free:kilo-test", modelID)
				_ = SnapshotFreeProviderModelRuntime("free:kilo-test")
			}
		}(i)
	}
	wg.Wait()
}

func runtimeIntPtr(value int) *int { return &value }
