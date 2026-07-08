package fallback

import (
	"net/http"
	"testing"
	"time"
)

func TestCalculateRelayCooldownDurationUsesRetryAfterWithCap(t *testing.T) {
	retryAfter := 600
	got := CalculateRelayCooldownDuration(RelayCooldownInput{
		Category:          ErrorCategoryRateLimit,
		StatusCode:        http.StatusTooManyRequests,
		RetryAfterSeconds: &retryAfter,
		Attempt:           1,
	})

	if got != 300*time.Second {
		t.Fatalf("expected Retry-After to be capped at 300s, got %s", got)
	}
}

func TestCalculateRelayCooldownDurationUsesGatewayBackoff(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "first attempt", attempt: 1, want: 60 * time.Second},
		{name: "second attempt", attempt: 2, want: 120 * time.Second},
		{name: "fourth attempt capped", attempt: 4, want: 300 * time.Second},
		{name: "zero attempt treated as first", attempt: 0, want: 60 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateRelayCooldownDuration(RelayCooldownInput{
				Category:   ErrorCategoryTemporary,
				StatusCode: http.StatusServiceUnavailable,
				Attempt:    tc.attempt,
			})
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestApplyRelayCooldownMarksRateLimitCooldown(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	retryAfter := 120
	before := time.Now()
	duration, err := ApplyRelayCooldown("free:groq-001122ff", "rate limited", RelayCooldownInput{
		Category:          ErrorCategoryRateLimit,
		StatusCode:        http.StatusTooManyRequests,
		RetryAfterSeconds: &retryAfter,
		Attempt:           1,
	})
	if err != nil {
		t.Fatalf("ApplyRelayCooldown failed: %v", err)
	}
	if duration != 120*time.Second {
		t.Fatalf("expected 120s cooldown, got %s", duration)
	}

	cooldownUntil, reason, err := GetDeploymentCooldown("free:groq-001122ff")
	if err != nil {
		t.Fatalf("GetDeploymentCooldown failed: %v", err)
	}
	if reason != "rate limited" {
		t.Fatalf("expected reason to be saved, got %q", reason)
	}
	if cooldownUntil == nil || cooldownUntil.Before(before.Add(119*time.Second)) || cooldownUntil.After(before.Add(121*time.Second)) {
		t.Fatalf("unexpected cooldown_until: %v", cooldownUntil)
	}
}

func TestApplyRelayCooldownMarksQuotaExhausted(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	duration, err := ApplyRelayCooldown("free:routeway-aabbccdd", "quota exhausted", RelayCooldownInput{
		Category:   ErrorCategoryQuota,
		StatusCode: http.StatusPaymentRequired,
		Attempt:    1,
	})
	if err != nil {
		t.Fatalf("ApplyRelayCooldown failed: %v", err)
	}
	if duration <= 0 {
		t.Fatalf("expected positive exhausted duration, got %s", duration)
	}

	state, err := GetDeploymentState("free:routeway-aabbccdd", todayString())
	if err != nil {
		t.Fatalf("GetDeploymentState failed: %v", err)
	}
	if state.ExhaustedUntil == nil {
		t.Fatal("expected exhausted_until to be set")
	}
	if state.LastErrorCode != "exhausted" || state.LastErrorMessage == "" {
		t.Fatalf("expected exhausted error metadata, got code=%q message=%q", state.LastErrorCode, state.LastErrorMessage)
	}
}

func TestApplyRelayCooldownMarksAuthFailuresAsInvalid(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	defer resetStickyStateForTest(t)

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	for _, tc := range []struct {
		name       string
		statusCode int
		reason     string
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, reason: "invalid key"},
		{name: "forbidden", statusCode: http.StatusForbidden, reason: "permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deploymentID := "free:" + tc.name + "-test"
			SetStickyDeployment("cct/free", deploymentID)
			before := time.Now()
			duration, err := ApplyRelayCooldown(deploymentID, tc.reason, RelayCooldownInput{
				Category:   ErrorCategoryModelAccess,
				StatusCode: tc.statusCode,
				Attempt:    1,
			})
			if err != nil {
				t.Fatalf("ApplyRelayCooldown failed: %v", err)
			}
			if duration <= 0 || duration > 24*time.Hour+time.Second {
				t.Fatalf("expected 24h cooldown, got %s", duration)
			}

			cooldownUntil, reason, err := GetDeploymentCooldown(deploymentID)
			if err != nil {
				t.Fatalf("GetDeploymentCooldown failed: %v", err)
			}
			if reason != tc.reason {
				t.Fatalf("expected reason to be saved, got %q", reason)
			}
			if cooldownUntil == nil || cooldownUntil.Before(before.Add(23*time.Hour+58*time.Minute)) || cooldownUntil.After(before.Add(24*time.Hour+2*time.Minute)) {
				t.Fatalf("unexpected cooldown_until: %v", cooldownUntil)
			}

			if got := GetStickyDeployment("cct/free"); got != "" {
				t.Fatalf("expected auth failure to clear sticky, got %q", got)
			}
		})
	}
}

func TestApplyRelayCooldownKeepsNonAuthModelAccessWithoutStateChange(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	duration, err := ApplyRelayCooldown("free:model-miss", "model not found", RelayCooldownInput{
		Category:   ErrorCategoryModelAccess,
		StatusCode: http.StatusNotFound,
		Attempt:    1,
	})
	if err != nil {
		t.Fatalf("ApplyRelayCooldown failed: %v", err)
	}
	if duration != 0 {
		t.Fatalf("expected zero duration for non-auth model access failures, got %s", duration)
	}

	cooldownUntil, reason, err := GetDeploymentCooldown("free:model-miss")
	if err != nil {
		t.Fatalf("GetDeploymentCooldown failed: %v", err)
	}
	if cooldownUntil != nil || reason != "" {
		t.Fatalf("expected no cooldown state for non-auth model access failure, got until=%v reason=%q", cooldownUntil, reason)
	}
}
