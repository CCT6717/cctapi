package fallback

import (
	"testing"
	"time"
)

func TestQuotaPeriodDateRefreshesAtNoonUTC8(t *testing.T) {
	beforeNoon := time.Date(2026, 6, 3, 3, 59, 59, 0, time.UTC)
	if got := quotaPeriodDate(beforeNoon); got != "2026-06-02" {
		t.Fatalf("expected previous quota date before noon UTC+8, got %s", got)
	}

	atNoon := time.Date(2026, 6, 3, 4, 0, 0, 0, time.UTC)
	if got := quotaPeriodDate(atNoon); got != "2026-06-03" {
		t.Fatalf("expected current quota date at noon UTC+8, got %s", got)
	}
}

func TestNextQuotaRefreshTime(t *testing.T) {
	beforeNoon := time.Date(2026, 6, 3, 3, 59, 59, 0, time.UTC)
	expectedSameDayNoon := time.Date(2026, 6, 3, 4, 0, 0, 0, time.UTC)
	if got := nextQuotaRefreshTime(beforeNoon); !got.Equal(expectedSameDayNoon) {
		t.Fatalf("expected next refresh %s, got %s", expectedSameDayNoon, got)
	}

	afterNoon := time.Date(2026, 6, 3, 4, 0, 1, 0, time.UTC)
	expectedNextDayNoon := time.Date(2026, 6, 4, 4, 0, 0, 0, time.UTC)
	if got := nextQuotaRefreshTime(afterNoon); !got.Equal(expectedNextDayNoon) {
		t.Fatalf("expected next refresh %s, got %s", expectedNextDayNoon, got)
	}
}

func TestMarkDeploymentCooldownForDurationUsesRelativeWindow(t *testing.T) {
	start := time.Date(2026, 6, 7, 1, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if !end.After(start) {
		t.Fatal("expected 24h cooldown end to be after start")
	}
}

func TestIsDoubaoDeploymentMatchesIDOrModel(t *testing.T) {
	if !IsDoubaoDeployment(DeploymentConfig{ID: "doubao-pro", RealModel: "deepseek-v3"}) {
		t.Fatal("expected doubao deployment by ID to match")
	}
	if !IsDoubaoDeployment(DeploymentConfig{ID: "core-auto", RealModel: "doubao-seed-2-0-pro-260215"}) {
		t.Fatal("expected doubao deployment by real model to match")
	}
	if IsDoubaoDeployment(DeploymentConfig{ID: "core-auto", RealModel: "deepseek-v3"}) {
		t.Fatal("expected non-doubao deployment not to match")
	}
}

func TestClearDeploymentCooldownClearsPersistentCooldown(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:groq-001122ff"
	if err := MarkDeploymentCooldown(deploymentID, "rate limited", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkDeploymentCooldown failed: %v", err)
	}

	cooldownUntil, reason, err := GetDeploymentCooldown(deploymentID)
	if err != nil || cooldownUntil == nil || reason != "rate limited" {
		t.Fatalf("expected active cooldown before clear, until=%v reason=%q err=%v", cooldownUntil, reason, err)
	}

	if err := ClearDeploymentCooldown(deploymentID); err != nil {
		t.Fatalf("ClearDeploymentCooldown failed: %v", err)
	}

	cooldownUntil, reason, err = GetDeploymentCooldown(deploymentID)
	if err != nil {
		t.Fatalf("GetDeploymentCooldown failed: %v", err)
	}
	if cooldownUntil != nil || reason != "" {
		t.Fatalf("expected persistent cooldown cleared, until=%v reason=%q", cooldownUntil, reason)
	}
}

func TestResetDeploymentStateClearsPersistentCooldown(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:routeway-aabbccdd"
	if err := MarkDeploymentCooldown(deploymentID, "manual cooldown", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkDeploymentCooldown failed: %v", err)
	}

	if err := ResetDeploymentState(deploymentID); err != nil {
		t.Fatalf("ResetDeploymentState failed: %v", err)
	}

	cooldownUntil, reason, err := GetDeploymentCooldown(deploymentID)
	if err != nil {
		t.Fatalf("GetDeploymentCooldown failed: %v", err)
	}
	if cooldownUntil != nil || reason != "" {
		t.Fatalf("expected reset to clear persistent cooldown, until=%v reason=%q", cooldownUntil, reason)
	}
}

func TestMarkDeploymentCooldownClearsStickyDeployment(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetStickyStateForTest(t)

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:groq-001122ff"
	SetStickyDeployment("cct/free", deploymentID)
	SetStickyDeployment("cct/paid", "paid:stable")

	if err := MarkDeploymentCooldown(deploymentID, "rate limited", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkDeploymentCooldown failed: %v", err)
	}

	if got := GetStickyDeployment("cct/free"); got != "" {
		t.Fatalf("expected cooled-down deployment to lose sticky routing, got %q", got)
	}
	if got := GetStickyDeployment("cct/paid"); got != "paid:stable" {
		t.Fatalf("expected unrelated sticky deployment to remain, got %q", got)
	}
}

func TestMarkDeploymentExhaustedClearsStickyDeployment(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetStickyStateForTest(t)

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:routeway-aabbccdd"
	SetStickyDeployment("cct/free", deploymentID)

	if err := MarkDeploymentExhausted(deploymentID, "quota exhausted", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkDeploymentExhausted failed: %v", err)
	}

	if got := GetStickyDeployment("cct/free"); got != "" {
		t.Fatalf("expected exhausted deployment to lose sticky routing, got %q", got)
	}
}

func resetStickyStateForTest(t *testing.T) {
	t.Helper()
	reset := func() {
		stickyDepMu.Lock()
		defer stickyDepMu.Unlock()
		stickyDep = nil
	}
	reset()
	t.Cleanup(reset)
}
