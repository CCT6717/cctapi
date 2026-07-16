package router

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackupFallbackEditorConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fallback.json")
	oldContent := []byte(`{"enabled":true}`)
	if err := os.WriteFile(configPath, oldContent, 0644); err != nil {
		t.Fatalf("failed to write source config: %v", err)
	}

	backupPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected backup to succeed, got %v", err)
	}
	if !strings.Contains(backupPath, filepath.Join("backups", "fallback.")) {
		t.Fatalf("expected backup path under backups directory, got %s", backupPath)
	}

	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup config: %v", err)
	}
	if string(backupContent) != string(oldContent) {
		t.Fatalf("expected backup content %s, got %s", oldContent, backupContent)
	}
}

func TestBuildFreePoolSyncResponseReportsPartialFailure(t *testing.T) {
	report := fallback.FreeProviderCatalogSyncReport{
		Attempted: 2,
		Succeeded: 1,
		Failed:    1,
		Results: []fallback.FreeProviderCatalogSyncResult{
			{Provider: "kilo", Attempted: 1, Succeeded: 1, Errors: []string{}},
			{Provider: "ovh", Attempted: 1, Failed: 1, Errors: []string{"status 429"}},
		},
	}
	payload := buildFreePoolSyncResponse(report)
	if success, _ := payload["success"].(bool); success {
		t.Fatalf("partial catalog failure must not report success: %#v", payload)
	}
	data, ok := payload["data"].(gin.H)
	if !ok {
		t.Fatalf("sync response missing data object: %#v", payload)
	}
	got, ok := data["catalog_sync"].(fallback.FreeProviderCatalogSyncReport)
	if !ok || got.Failed != 1 {
		t.Fatalf("sync response missing catalog report: %#v", payload)
	}
}

func TestBuildFreePoolSyncResponseDoesNotReportSkippedRefreshAsComplete(t *testing.T) {
	report := fallback.FreeProviderCatalogSyncReport{
		Attempted: 1,
		Skipped:   1,
		Results: []fallback.FreeProviderCatalogSyncResult{
			{Provider: "kilo", Attempted: 1, Skipped: 1, Errors: []string{}},
		},
	}
	payload := buildFreePoolSyncResponse(report)
	if success, _ := payload["success"].(bool); success {
		t.Fatalf("skipped catalog refresh must not report complete success: %#v", payload)
	}
	if message, _ := payload["message"].(string); !strings.Contains(message, "skipped") {
		t.Fatalf("skipped sync response should explain the outcome: %#v", payload)
	}
}

func TestBackupFallbackEditorConfigCreatesUniquePaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fallback.json")
	if err := os.WriteFile(configPath, []byte(`{"enabled":true}`), 0644); err != nil {
		t.Fatalf("failed to write source config: %v", err)
	}

	firstPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected first backup to succeed, got %v", err)
	}
	secondPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected second backup to succeed, got %v", err)
	}
	if firstPath == secondPath {
		t.Fatalf("expected unique backup paths, got %s", firstPath)
	}
}

func TestBackupFallbackEditorConfigMissingFile(t *testing.T) {
	backupPath, err := backupFallbackEditorConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("expected missing config to be ignored, got %v", err)
	}
	if backupPath != "" {
		t.Fatalf("expected no backup path for missing config, got %s", backupPath)
	}
}

func TestBackupFallbackEditorConfigSanitizesFreeProviderKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fallback.json")
	rawKey := "gsk_backup_secret_not_real_12345"
	config := `{
  "enabled": true,
  "free_providers": {
    "groq": {
      "enabled": true,
      "keys": ["` + rawKey + `"]
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write source config: %v", err)
	}

	backupPath, err := backupFallbackEditorConfig(configPath)
	if err != nil {
		t.Fatalf("expected backup to succeed, got %v", err)
	}

	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup config: %v", err)
	}
	if strings.Contains(string(backupContent), rawKey) {
		t.Fatalf("backup must not contain raw free provider key: %s", backupContent)
	}
	if !strings.Contains(string(backupContent), `"keys": []`) {
		t.Fatalf("expected sanitized backup to remove stored keys, got %s", backupContent)
	}
	if !strings.Contains(string(backupContent), fallback.SafeKeyHash(rawKey)) {
		t.Fatalf("expected sanitized backup to retain non-secret key hash, got %s", backupContent)
	}
}

func TestTriggerDeploymentHealthCheckReturnsRuntimeErrorDetails(t *testing.T) {
	originalDB := dbmodel.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Channel{}); err != nil {
		t.Fatalf("failed to migrate channel table: %v", err)
	}
	dbmodel.DB = db
	defer func() {
		_ = fallback.LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
		dbmodel.DB = originalDB
	}()

	deploymentID := "free:pollinations-" + fallback.SafeKeyHash("") + "-handler-missing-channel"
	configPath := filepath.Join(t.TempDir(), "fallback.json")
	config := `{
  "enabled": true,
  "virtual_models": {
    "cct/free": {
      "enabled": true,
      "strategy": "free_first",
      "pools": ["free"]
    }
  },
  "deployments": {
    "` + deploymentID + `": {
      "enabled": true,
      "channel_id": 404,
      "real_model": "openai-fast",
      "pool": "free"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write fallback config: %v", err)
	}
	if err := fallback.LoadConfig(configPath); err != nil {
		t.Fatalf("failed to load fallback config: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: deploymentID}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/fallback/deployments/"+deploymentID+"/health-check", nil)

	triggerDeploymentHealthCheck(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"runtime"`) {
		t.Fatalf("expected runtime snapshot in health-check response, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"last_error"`) || !strings.Contains(w.Body.String(), "health check channel") {
		t.Fatalf("expected last_error details in health-check response, got %s", w.Body.String())
	}
}

func TestBuildFallbackRuntimeStatusRowsIncludesRuntimeVisibilityFields(t *testing.T) {
	originalDB := dbmodel.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	dbmodel.DB = db
	defer func() {
		dbmodel.DB = originalDB
		fallback.ClearStickyDeployment("cct/free")
	}()
	if err := fallback.InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	coolingDeploymentID := "free:groq-runtime-visible"
	stickyDeploymentID := "free:routeway-runtime-visible"
	fallback.RecordFailure(coolingDeploymentID, "provider returned 429", true)
	if err := fallback.RecordDeploymentError(coolingDeploymentID, errors.New("provider returned 429")); err != nil {
		t.Fatalf("RecordDeploymentError failed: %v", err)
	}
	if err := fallback.MarkDeploymentCooldown(coolingDeploymentID, "rate limited", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkDeploymentCooldown failed: %v", err)
	}
	fallback.SetStickyDeployment("cct/free", stickyDeploymentID)

	rows := buildFallbackRuntimeStatusRows(&fallback.Config{
		Enabled: true,
		VirtualModels: map[string]fallback.VirtualModelConfig{
			"cct/free": {Enabled: true, Pools: []string{"free"}},
		},
		Deployments: map[string]fallback.DeploymentConfig{
			coolingDeploymentID: {
				Enabled:   true,
				Pool:      "free",
				RealModel: "llama-free",
			},
			stickyDeploymentID: {
				Enabled:   true,
				Pool:      "free",
				RealModel: "routeway-free",
			},
		},
	})

	coolingRow := findRuntimeStatusRow(t, rows, coolingDeploymentID)
	if coolingRow["last_error"] != "provider returned 429" {
		t.Fatalf("expected runtime last_error, got %#v", coolingRow["last_error"])
	}
	if coolingRow["state_last_error_message"] != "provider returned 429" {
		t.Fatalf("expected persistent last error message, got %#v", coolingRow["state_last_error_message"])
	}
	if coolingRow["state_last_error_code"] != "unknown" {
		t.Fatalf("expected persistent last error code, got %#v", coolingRow["state_last_error_code"])
	}
	if coolingRow["cooldown_reason"] != "rate limited" {
		t.Fatalf("expected cooldown reason, got %#v", coolingRow["cooldown_reason"])
	}
	if coolingRow["cooldown_active"] != true {
		t.Fatalf("expected active cooldown, got %#v", coolingRow["cooldown_active"])
	}
	if coolingRow["cooldown_until"] == nil {
		t.Fatal("expected cooldown_until to be populated")
	}

	stickyRow := findRuntimeStatusRow(t, rows, stickyDeploymentID)
	if stickyRow["is_sticky"] != true {
		t.Fatalf("expected sticky deployment flag, got %#v", stickyRow["is_sticky"])
	}
	stickyVirtualModels, ok := stickyRow["sticky_virtual_models"].([]string)
	if !ok || len(stickyVirtualModels) != 1 || stickyVirtualModels[0] != "cct/free" {
		t.Fatalf("expected sticky virtual model list, got %#v", stickyRow["sticky_virtual_models"])
	}
}

func findRuntimeStatusRow(t *testing.T, rows []map[string]interface{}, deploymentID string) map[string]interface{} {
	t.Helper()
	for _, row := range rows {
		if row["deployment_id"] == deploymentID {
			return row
		}
	}
	t.Fatalf("runtime status row %s not found in %#v", deploymentID, rows)
	return nil
}

func TestSplitFallbackEditorChannelModels(t *testing.T) {
	models := splitFallbackEditorChannelModels(" deepseek-v3,deepseek-reasoner,, deepseek-v3 , claude-3-5-sonnet ")

	expected := []string{"deepseek-v3", "deepseek-reasoner", "claude-3-5-sonnet"}
	if len(models) != len(expected) {
		t.Fatalf("expected %d models, got %d: %v", len(expected), len(models), models)
	}
	for i := range expected {
		if models[i] != expected[i] {
			t.Fatalf("expected model %d to be %s, got %s", i, expected[i], models[i])
		}
	}
}

func TestBuildFallbackConfigFromEditorPreservesStrategyAndPools(t *testing.T) {
	payload := fallbackEditorConfig{
		Enabled: true,
	}
	virtualModels := []fallbackEditorVirtualModel{
		{
			Name:               "cct/free",
			Enabled:            true,
			Description:        "free pool virtual model",
			Strategy:           "free_first",
			Pools:              []string{"free"},
			AllowDegradeToLow:  false,
			AllowDegradeToFree: false,
		},
	}
	deployments := []fallbackEditorDeployment{
		{ID: "groq-free", Enabled: true, ChannelID: 1, RealModel: "llama-3.1-8b-instant", Pool: "free", CostTier: "free"},
	}

	cfg := buildFallbackConfigFromEditor(payload, virtualModels, deployments)

	vm := cfg.VirtualModels["cct/free"]
	if vm.Strategy != fallback.StrategyFreeFirst {
		t.Fatalf("expected strategy free_first, got %s", vm.Strategy)
	}
	if len(vm.Pools) != 1 || vm.Pools[0] != "free" {
		t.Fatalf("expected pools [free], got %v", vm.Pools)
	}
	dep := cfg.Deployments["groq-free"]
	if dep.Pool != "free" || dep.CostTier != "free" {
		t.Fatalf("expected deployment pool=free cost_tier=free, got pool=%s cost_tier=%s", dep.Pool, dep.CostTier)
	}
}

func TestBuildFallbackConfigFromEditorPreservesUnmanagedFreeProviders(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "fallback.json")
	if err := os.WriteFile(configPath, []byte(`{
  "enabled": true,
  "virtual_models": {},
  "deployments": {},
  "free_providers": {
    "groq": {
      "enabled": true,
      "keys": ["stored-secret-key"],
      "models": ["llama-3.1-8b-instant"],
      "limits_override": {"rpm_limit": 11}
    }
  },
  "blocked_error_codes": ["insufficient_quota"],
  "alert": {"check_interval_sec": 300},
  "smart_sort": {"enabled": false, "weights": {"base_priority_penalty": 10}}
}`), 0644); err != nil {
		t.Fatalf("failed to write fallback config: %v", err)
	}
	if err := fallback.LoadConfig(configPath); err != nil {
		t.Fatalf("failed to load fallback config: %v", err)
	}
	t.Cleanup(func() {
		_ = fallback.LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	})

	payload := fallbackEditorConfig{Enabled: true}
	virtualModels := []fallbackEditorVirtualModel{
		{Name: "cct/free", Enabled: true, Strategy: "free_first", Pools: []string{"free"}},
	}
	deployments := []fallbackEditorDeployment{
		{ID: "groq-free", Enabled: true, ChannelID: 1, RealModel: "llama-3.1-8b-instant", Pool: "free", CostTier: "free"},
	}

	cfg := buildFallbackConfigFromEditor(payload, virtualModels, deployments)

	groq, ok := cfg.FreeProviders["groq"]
	if !ok {
		t.Fatalf("expected existing groq free provider to be preserved")
	}
	if len(groq.Keys) != 1 || groq.Keys[0] != "stored-secret-key" {
		t.Fatalf("expected stored free provider key to be preserved, got %v", groq.Keys)
	}
	if groq.LimitsOverride == nil || groq.LimitsOverride.RPMLimit == nil || *groq.LimitsOverride.RPMLimit != 11 {
		t.Fatalf("expected stored limits override to be preserved, got %#v", groq.LimitsOverride)
	}
	if len(cfg.BlockedErrorCodes) != 1 || cfg.BlockedErrorCodes[0] != "insufficient_quota" {
		t.Fatalf("expected blocked error codes to be preserved, got %v", cfg.BlockedErrorCodes)
	}
}

func TestNormalizeFallbackEditorPayloadRejectsInvalidPoolConfig(t *testing.T) {
	basePayload := func() fallbackEditorConfig {
		return fallbackEditorConfig{
			Enabled: true,
			VirtualModels: []fallbackEditorVirtualModel{
				{
					Name:     "cct/free",
					Enabled:  true,
					Strategy: "free_first",
					Pools:    []string{"free"},
				},
			},
			Deployments: []fallbackEditorDeployment{
				{ID: "groq-free", Enabled: true, ChannelID: 1, RealModel: "llama-3.1-8b-instant", Pool: "free"},
				{ID: "paid-1", Enabled: true, ChannelID: 2, RealModel: "gpt-4", Pool: "paid_high"},
			},
		}
	}

	tests := []struct {
		name string
		edit func(*fallbackEditorConfig)
	}{
		{
			name: "empty pools",
			edit: func(payload *fallbackEditorConfig) {
				payload.VirtualModels[0].Pools = nil
			},
		},
		{
			name: "pool with no deployments",
			edit: func(payload *fallbackEditorConfig) {
				payload.VirtualModels[0].Pools = []string{"nonexistent"}
			},
		},
		{
			name: "pool with only disabled deployments",
			edit: func(payload *fallbackEditorConfig) {
				payload.Deployments[0].Enabled = false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := basePayload()
			tt.edit(&payload)
			if _, _, err := normalizeFallbackEditorPayload(payload); err == nil {
				t.Fatalf("expected normalizeFallbackEditorPayload to reject %s", tt.name)
			}
		})
	}
}

func TestBuildFallbackEditorConfigIncludesStrategyAndPools(t *testing.T) {
	cfg := &fallback.Config{
		Enabled: true,
		VirtualModels: map[string]fallback.VirtualModelConfig{
			"cct/free": {
				Enabled:  true,
				Strategy: fallback.StrategyFreeFirst,
				Pools:    []string{"free"},
			},
		},
		Deployments: map[string]fallback.DeploymentConfig{
			"groq-free": {Enabled: true, ChannelID: 0, RealModel: "llama-3.1-8b-instant", Pool: "free", CostTier: "free"},
		},
	}

	editorCfg := buildFallbackEditorConfig(cfg)

	if len(editorCfg.VirtualModels) != 1 {
		t.Fatalf("expected one virtual model, got %d", len(editorCfg.VirtualModels))
	}
	vm := editorCfg.VirtualModels[0]
	if vm.Strategy != fallback.StrategyFreeFirst {
		t.Fatalf("expected strategy free_first, got %s", vm.Strategy)
	}
	if len(vm.Pools) != 1 || vm.Pools[0] != "free" {
		t.Fatalf("expected pools [free], got %v", vm.Pools)
	}
}

func TestMaskSecretKey_Empty(t *testing.T) {
	masked := maskSecretKey("")
	if masked != "" {
		t.Fatalf("expected empty, got %q", masked)
	}
}

func TestMaskSecretKey_ShortKey(t *testing.T) {
	masked := maskSecretKey("abc123")
	if masked != "********" {
		t.Fatalf("expected 8 asterisks for short key, got %q", masked)
	}
}

func TestMaskSecretKey_DoesNotLeakOriginal(t *testing.T) {
	original := "sk-or-v1-TEST_PLACEHOLDER_NOT_REAL"
	masked := maskSecretKey(original)

	if masked == original {
		t.Fatal("masked key must not equal original")
	}
	if strings.Contains(masked, original) {
		t.Fatal("masked key must not contain original")
	}

	if !strings.HasPrefix(masked, original[:4]) {
		t.Fatalf("expected masked key to start with %q, got %q", original[:4], masked)
	}
	if !strings.HasSuffix(masked, original[len(original)-4:]) {
		t.Fatalf("expected masked key to end with %q, got %q", original[len(original)-4:], masked)
	}

	if !strings.Contains(masked, "****") {
		t.Fatal("masked key must contain **** in the middle")
	}
}

func TestMaskSecretKey_Length(t *testing.T) {
	original := "sk-or-v1-TEST_PLACEHOLDER_NOT_REAL"
	masked := maskSecretKey(original)
	if len(masked) != len(original) {
		t.Fatalf("expected masked length %d, got %d", len(original), len(masked))
	}
}

func TestNormalizeFallbackEditorPayloadNormalizesRoutingMode(t *testing.T) {
	payload := fallbackEditorConfig{
		Enabled: true,
		VirtualModels: []fallbackEditorVirtualModel{
			{
				Name:                "cct/high",
				Enabled:             true,
				Strategy:            "quality_first",
				Pools:               []string{"paid_high"},
				RoutingMode:         "Fixed",
				PreferredDeployment: "dep-a",
			},
		},
		Deployments: []fallbackEditorDeployment{
			{ID: "dep-a", Enabled: true, ChannelID: 1, RealModel: "gpt-4", Pool: "paid_high"},
		},
	}
	vms, _, err := normalizeFallbackEditorPayload(payload)
	if err != nil {
		t.Fatalf("expected normalize to succeed, got %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	if vms[0].RoutingMode != "fixed" {
		t.Fatalf("expected RoutingMode 'fixed' after normalization, got %q", vms[0].RoutingMode)
	}
}

func TestNormalizeFallbackEditorPayloadDefaultsRoutingModeToFallback(t *testing.T) {
	payload := fallbackEditorConfig{
		Enabled: true,
		VirtualModels: []fallbackEditorVirtualModel{
			{
				Name:     "cct/high",
				Enabled:  true,
				Strategy: "quality_first",
				Pools:    []string{"paid_high"},
			},
		},
		Deployments: []fallbackEditorDeployment{
			{ID: "dep-a", Enabled: true, ChannelID: 1, RealModel: "gpt-4", Pool: "paid_high"},
		},
	}
	vms, _, err := normalizeFallbackEditorPayload(payload)
	if err != nil {
		t.Fatalf("expected normalize to succeed, got %v", err)
	}
	if vms[0].RoutingMode != "fallback" {
		t.Fatalf("expected RoutingMode 'fallback' when omitted, got %q", vms[0].RoutingMode)
	}
}

func TestNormalizeFallbackEditorPayloadRejectsPreferredDeploymentOutsidePools(t *testing.T) {
	payload := fallbackEditorConfig{
		Enabled: true,
		VirtualModels: []fallbackEditorVirtualModel{
			{
				Name:                "cct/high",
				Enabled:             true,
				Strategy:            "quality_first",
				Pools:               []string{"paid_high"},
				RoutingMode:         "fixed",
				PreferredDeployment: "dep-b",
			},
		},
		Deployments: []fallbackEditorDeployment{
			{ID: "dep-a", Enabled: true, ChannelID: 1, RealModel: "gpt-4", Pool: "paid_high"},
			{ID: "dep-b", Enabled: true, ChannelID: 2, RealModel: "claude-3", Pool: "paid_low"},
		},
	}
	_, _, err := normalizeFallbackEditorPayload(payload)
	if err == nil {
		t.Fatal("expected preferred deployment outside pools to be rejected")
	}
	if !strings.Contains(err.Error(), "not in fallback order or pools") {
		t.Fatalf("expected pool membership error, got %v", err)
	}
}

func TestBuildFallbackEditorChannel_NoFullKey(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:   1,
		Name: "test-channel",
		Type: 1,
		Key:  "sk-or-v1-TEST_PLACEHOLDER_NOT_REAL",
	}
	baseURL := "https://api.example.com"
	channel.BaseURL = &baseURL

	result := buildFallbackEditorChannel(channel)

	if result.KeyMasked == channel.Key {
		t.Fatal("buildFallbackEditorChannel must not return full key")
	}
	if strings.Contains(result.KeyMasked, channel.Key) {
		t.Fatal("response must not contain full key")
	}

	if !result.HasKey {
		t.Fatal("expected HasKey = true when channel has a key")
	}

	if result.KeyMasked == "" {
		t.Fatal("key_masked must not be empty when channel has a key")
	}
	if strings.Contains(result.KeyMasked, channel.Key) {
		t.Fatal("key_masked must not contain full key")
	}
	if !strings.Contains(result.KeyMasked, "****") {
		t.Fatal("key_masked must contain masking asterisks")
	}
}

func TestBuildFallbackRuntimeStatusRowsIncludesModelRuntime(t *testing.T) {
	deploymentID := "free:kilo-model-runtime"
	t.Cleanup(func() {
		fallback.ResetFreeProviderModelRuntime(deploymentID)
		fallback.ResetFreeProviderModelCapabilityFalsePositive(deploymentID, "")
	})

	retryAfter := 120
	fallback.MarkFreeProviderModelRateLimited(deploymentID, "kilo/model-a:free", "rate limited", fallback.RelayCooldownInput{
		Category: fallback.ErrorCategoryRateLimit, StatusCode: http.StatusTooManyRequests,
		RetryAfterSeconds: &retryAfter, Attempt: 1,
	})
	fallback.MarkFreeProviderModelCapabilityFalsePositive(deploymentID, "kilo/model-b:free", "tools")

	rows := buildFallbackRuntimeStatusRows(&fallback.Config{
		Enabled: true,
		Deployments: map[string]fallback.DeploymentConfig{
			deploymentID: {
				Enabled:   true,
				Pool:      "free",
				RealModel: "kilo/model-a:free",
			},
		},
	})

	row := findRuntimeStatusRow(t, rows, deploymentID)
	modelRuntime, ok := row["model_runtime"].(fallback.FreeProviderModelRuntimeDiagnostics)
	if !ok {
		t.Fatalf("expected model_runtime field, got %#v", row["model_runtime"])
	}
	if modelRuntime.ActiveCooldownCount != 1 || len(modelRuntime.Models) != 1 {
		t.Fatalf("unexpected model runtime: %#v", modelRuntime)
	}
	if modelRuntime.ActiveCapabilityFalsePositiveCount != 1 || len(modelRuntime.CapabilityFalsePositives) != 1 {
		t.Fatalf("unexpected capability diagnostics: %#v", modelRuntime)
	}
	capabilityRuntime := modelRuntime.CapabilityFalsePositives[0]
	if capabilityRuntime.ModelID != "kilo/model-b:free" || capabilityRuntime.Capability != "tools" || capabilityRuntime.Reason != "invalid tool arguments" || capabilityRuntime.ExpiresAt == nil {
		t.Fatalf("unexpected capability false-positive: %#v", capabilityRuntime)
	}

	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(raw), "sk-secret") || strings.Contains(string(raw), "Bearer") || strings.Contains(string(raw), "raw_upstream_body") {
		t.Fatalf("runtime row leaked sensitive material: %s", string(raw))
	}
}

func TestBuildFallbackEditorChannel_NoKey(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:   2,
		Name: "no-key-channel",
		Type: 1,
		Key:  "",
	}
	baseURL := "https://api.example.com"
	channel.BaseURL = &baseURL

	result := buildFallbackEditorChannel(channel)

	if result.KeyMasked != "" {
		t.Fatalf("expected empty key_masked for channel without key, got %q", result.KeyMasked)
	}
	if result.HasKey {
		t.Fatal("expected HasKey = false when channel has no key")
	}
}
