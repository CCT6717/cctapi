package fallback

import (
	"errors"
	"testing"
	"time"

	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/gorm"
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

func TestResetDeploymentStateClearsOnlyTargetModelRuntime(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	targetDeploymentID := "free:kilo-001122ff"
	otherDeploymentID := "free:kilo-aabbccdd"
	ResetProviderRateLimitDegradation(targetDeploymentID)
	ResetProviderRateLimitDegradation(otherDeploymentID)
	t.Cleanup(func() {
		ResetProviderRateLimitDegradation(targetDeploymentID)
		ResetProviderRateLimitDegradation(otherDeploymentID)
	})
	MarkFreeProviderModelRateLimited(targetDeploymentID, "model-a", "rate limited", RelayCooldownInput{})
	MarkFreeProviderModelRateLimited(otherDeploymentID, "model-b", "rate limited", RelayCooldownInput{})
	MarkFreeProviderModelCapabilityFalsePositive(targetDeploymentID, "model-a", "tools")
	MarkFreeProviderModelCapabilityFalsePositive(otherDeploymentID, "model-b", "tools")
	RecordProviderRateLimitEpisode(targetDeploymentID, 0)
	time.Sleep(time.Millisecond)
	RecordProviderRateLimitEpisode(targetDeploymentID, 0)
	RecordProviderRateLimitEpisode(otherDeploymentID, 0)
	time.Sleep(time.Millisecond)
	RecordProviderRateLimitEpisode(otherDeploymentID, 0)

	if err := ResetDeploymentState(targetDeploymentID); err != nil {
		t.Fatalf("ResetDeploymentState failed: %v", err)
	}

	if got := SnapshotFreeProviderModelRuntime(targetDeploymentID); got.ActiveCooldownCount != 0 || len(got.Models) != 0 {
		t.Fatalf("target model runtime was not reset: %+v", got)
	}
	if !IsFreeProviderModelCooling(otherDeploymentID, "model-b") {
		t.Fatal("reset affected another deployment model runtime")
	}
	if IsFreeProviderModelCapabilityFalsePositive(targetDeploymentID, "model-a", "tools") {
		t.Fatal("target capability false-positive was not reset")
	}
	if !IsFreeProviderModelCapabilityFalsePositive(otherDeploymentID, "model-b", "tools") {
		t.Fatal("reset affected another deployment capability false-positive")
	}
	if got := SnapshotProviderRateLimitDegradation(targetDeploymentID); got.Active || got.EpisodeCount != 0 {
		t.Fatalf("target provider degradation was not reset: %#v", got)
	}
	if got := SnapshotProviderRateLimitDegradation(otherDeploymentID); !got.Active || got.EpisodeCount != 2 {
		t.Fatalf("reset affected another deployment provider degradation: %#v", got)
	}
}

func TestResetDeploymentStateKeepsModelRuntimeWhenPersistentResetFails(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetFreeProviderModelRuntimeForTest()
	t.Cleanup(resetFreeProviderModelRuntimeForTest)

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:kilo-reset-failure"
	ResetProviderRateLimitDegradation(deploymentID)
	t.Cleanup(func() { ResetProviderRateLimitDegradation(deploymentID) })
	MarkFreeProviderModelRateLimited(deploymentID, "model-a", "rate limited", RelayCooldownInput{})
	MarkFreeProviderModelCapabilityFalsePositive(deploymentID, "model-a", "tools")
	RecordProviderRateLimitEpisode(deploymentID, 0)
	time.Sleep(time.Millisecond)
	RecordProviderRateLimitEpisode(deploymentID, 0)
	if _, err := EnsureDeploymentState(deploymentID, todayString()); err != nil {
		t.Fatalf("EnsureDeploymentState failed: %v", err)
	}
	if _, err := EnsureDeploymentCooldownState(deploymentID); err != nil {
		t.Fatalf("EnsureDeploymentCooldownState failed: %v", err)
	}

	callbackName := "test:fail_deployment_cooldown_reset"
	if err := dbmodel.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "deployment_cooldown_states" {
			tx.AddError(errors.New("forced persistent cooldown reset failure"))
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}

	if err := ResetDeploymentState(deploymentID); err == nil {
		t.Fatal("ResetDeploymentState succeeded despite forced persistent failure")
	}
	if !IsFreeProviderModelCooling(deploymentID, "model-a") {
		t.Fatal("failed persistent reset cleared model runtime")
	}
	if !IsFreeProviderModelCapabilityFalsePositive(deploymentID, "model-a", "tools") {
		t.Fatal("failed persistent reset cleared capability false-positive")
	}
	if got := SnapshotProviderRateLimitDegradation(deploymentID); !got.Active || got.EpisodeCount != 2 {
		t.Fatalf("failed persistent reset cleared provider degradation: %#v", got)
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

func TestMarkInvalidClearsStickyAndSetsCooldown(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	resetStickyStateForTest(t)

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "free:invalid-auth"
	SetStickyDeployment("cct/free", deploymentID)

	if err := MarkInvalid(deploymentID, "invalid api key"); err != nil {
		t.Fatalf("MarkInvalid failed: %v", err)
	}

	if got := GetStickyDeployment("cct/free"); got != "" {
		t.Fatalf("expected invalid deployment to lose sticky routing, got %q", got)
	}

	cooldownUntil, reason, err := GetDeploymentCooldown(deploymentID)
	if err != nil {
		t.Fatalf("GetDeploymentCooldown failed: %v", err)
	}
	if reason != "invalid api key" {
		t.Fatalf("expected cooldown reason to be saved, got %q", reason)
	}
	if cooldownUntil == nil || cooldownUntil.Before(time.Now().Add(23*time.Hour)) {
		t.Fatalf("expected near-24h cooldown, got until=%v", cooldownUntil)
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
