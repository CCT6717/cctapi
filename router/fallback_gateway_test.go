package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
	"github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// ensureTestDB initialises an in-memory SQLite database (once per process) and
// assigns it to model.DB so that SyncFreePool and channel-related code paths
// do not panic during handler tests.
func ensureTestDB(t *testing.T) {
	t.Helper()
	if model.DB != nil {
		return
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite for tests: %v", err)
	}
	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("failed to auto-migrate Channel table: %v", err)
	}
	model.DB = db
}

// baseValidConfigJSON is a minimal valid fallback config used as the starting
// point for PUT tests that need the full save cycle.
const baseValidConfigJSON = `{
  "enabled": true,
  "virtual_models": {
    "test/auto": {
      "enabled": true,
      "strategy": "quality_first",
      "pools": ["high"]
    }
  },
  "deployments": {
    "dep-1": {
      "enabled": true,
      "channel_id": 1,
      "real_model": "gpt-4",
      "pool": "high",
      "quality_tier": "high",
      "cost_tier": "paid"
    }
  }
}`

// baseValidConfigWithFreeProviderJSON adds a groq free_provider with a known
// key so that key-preservation tests have something to compare against.
const baseValidConfigWithFreeProviderJSON = `{
  "enabled": true,
  "virtual_models": {
    "test/auto": {
      "enabled": true,
      "strategy": "quality_first",
      "pools": ["high"]
    }
  },
  "deployments": {
    "dep-1": {
      "enabled": true,
      "channel_id": 1,
      "real_model": "gpt-4",
      "pool": "high",
      "quality_tier": "high",
      "cost_tier": "paid"
    }
  },
  "free_providers": {
    "groq": {
      "enabled": true,
      "keys": ["gsk_original_test_key_not_real_12345"]
    }
  }
}`

// setupGatewayConfigForSave creates a temp directory with data/fallback.json,
// changes CWD to it, loads the config via fallback.LoadConfig, and returns a
// cleanup function that restores the original CWD.
// It also ensures model.DB is initialised so SyncFreePool won't panic.
func setupGatewayConfigForSave(t *testing.T, configJSON string) func() {
	t.Helper()
	ensureTestDB(t)

	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "fallback.json"), []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get CWD: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	if err := fallback.LoadConfig("data/fallback.json"); err != nil {
		_ = os.Chdir(origDir)
		t.Fatalf("failed to load config: %v", err)
	}

	return func() {
		_ = os.Chdir(origDir)
	}
}

// setupGatewayConfigReadOnly writes configJSON to a temp file and loads it via
// fallback.LoadConfig. It does NOT change CWD, and is suitable for tests that
// only need fallback.GetConfig() to return a non-nil value (e.g. GET handler
// or early-return 400 cases).
func setupGatewayConfigReadOnly(t *testing.T, configJSON string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fallback.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := fallback.LoadConfig(configPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
}

// callGatewayGET invokes the getGatewayConfig handler and returns the recorder.
func callGatewayGET(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/fallback/gateway/config", nil)
	getGatewayConfig(c)
	return w
}

// callGatewayPUT invokes the updateGatewayConfig handler with the given JSON
// body string and returns the recorder.
func callGatewayPUT(t *testing.T, jsonBody string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPut, "/api/fallback/gateway/config",
		bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	updateGatewayConfig(c)
	return w
}

// parseJSON decodes the recorder body into a generic map.
func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// ---------------------------------------------------------------------------
// Test 1: GET /api/fallback/gateway/config — success
// ---------------------------------------------------------------------------

func TestGatewayGetConfig_Success(t *testing.T) {
	setupGatewayConfigReadOnly(t, baseValidConfigJSON)

	w := callGatewayGET(t)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	resp := parseJSON(t, w)
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp["data"])
	}
	if data["enabled"] != true {
		t.Fatalf("expected enabled=true in data, got %v", data["enabled"])
	}
	vms, ok := data["virtual_models"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected virtual_models map, got %T", data["virtual_models"])
	}
	if _, ok := vms["test/auto"]; !ok {
		t.Fatalf("expected virtual model test/auto in response")
	}
}

// ---------------------------------------------------------------------------
// Test 2-4: Legacy field rejection (routing_mode, fallback_order, fixed_deployment)
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_LegacyFieldsRejected(t *testing.T) {
	// The legacy check happens before any file I/O, so read-only config is fine.
	setupGatewayConfigReadOnly(t, baseValidConfigJSON)

	tests := []struct {
		name       string
		legacyKey  string
		legacyVal  string
	}{
		{"routing_mode", "routing_mode", `"weighted"`},
		{"fallback_order", "fallback_order", `["dep-1"]`},
		{"fixed_deployment", "fixed_deployment", `"dep-1"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := `{
				"enabled": true,
				"virtual_models": {
					"test/auto": {
						"enabled": true,
						"` + tt.legacyKey + `": ` + tt.legacyVal + `
					}
				},
				"deployments": {
					"dep-1": {
						"enabled": true,
						"channel_id": 1,
						"real_model": "gpt-4",
						"pool": "high"
					}
				}
			}`

			w := callGatewayPUT(t, payload)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 for legacy field %s, got %d\nbody: %s",
					tt.name, w.Code, w.Body.String())
			}
			resp := parseJSON(t, w)
			if resp["success"] != false {
				t.Fatalf("expected success=false for legacy field %s", tt.name)
			}
			msg, _ := resp["message"].(string)
			if msg == "" {
				t.Fatalf("expected non-empty error message for legacy field %s", tt.name)
			}
			// The handler says "legacy field detected in v2 gateway config"
			if !contains(msg, "legacy") {
				t.Fatalf("expected message to mention 'legacy', got %q", msg)
			}
		})
	}
}

// contains is a small helper to avoid importing strings in every test.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Test 5: pools, strategy, deployment.pool preserved after save
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_PoolsStrategyDeploymentPoolPreserved(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
	defer cleanup()

	putPayload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "cost_first",
				"pools": ["high"],
				"allow_degrade_to_low": true
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high",
				"quality_tier": "high",
				"cost_tier": "paid",
				"priority": 5,
				"weight": 200
			}
		}
	}`

	w := callGatewayPUT(t, putPayload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v; message: %v", resp["success"], resp["message"])
	}

	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		t.Fatal("expected data in response")
	}

	// Verify virtual model
	vms, _ := data["virtual_models"].(map[string]interface{})
	vm, _ := vms["test/auto"].(map[string]interface{})
	if vm == nil {
		t.Fatal("expected virtual model test/auto in response data")
	}
	if vm["strategy"] != "cost_first" {
		t.Fatalf("expected strategy cost_first, got %v", vm["strategy"])
	}
	pools, _ := vm["pools"].([]interface{})
	if len(pools) != 1 || pools[0] != "high" {
		t.Fatalf("expected pools [high], got %v", pools)
	}
	if vm["allow_degrade_to_low"] != true {
		t.Fatalf("expected allow_degrade_to_low=true, got %v", vm["allow_degrade_to_low"])
	}

	// Verify deployment pool
	deps, _ := data["deployments"].(map[string]interface{})
	dep, _ := deps["dep-1"].(map[string]interface{})
	if dep == nil {
		t.Fatal("expected deployment dep-1 in response data")
	}
	if dep["pool"] != "high" {
		t.Fatalf("expected deployment pool=high, got %v", dep["pool"])
	}
	if dep["priority"] != float64(5) {
		t.Fatalf("expected priority=5, got %v", dep["priority"])
	}
	if dep["weight"] != float64(200) {
		t.Fatalf("expected weight=200, got %v", dep["weight"])
	}

	// Also verify via GET to ensure persistence
	wGet := callGatewayGET(t)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GET after PUT: expected 200, got %d", wGet.Code)
	}
	getResp := parseJSON(t, wGet)
	getData, _ := getResp["data"].(map[string]interface{})
	getVMs, _ := getData["virtual_models"].(map[string]interface{})
	getVM, _ := getVMs["test/auto"].(map[string]interface{})
	if getVM["strategy"] != "cost_first" {
		t.Fatalf("GET after PUT: expected strategy cost_first, got %v", getVM["strategy"])
	}
}

// ---------------------------------------------------------------------------
// Test 1 (full save): v2 gateway config save success
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_SaveSuccess(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
	defer cleanup()

	putPayload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "quality_first",
				"pools": ["high"]
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high",
				"quality_tier": "high",
				"cost_tier": "paid"
			}
		}
	}`

	w := callGatewayPUT(t, putPayload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d\nbody: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v; message: %v", resp["success"], resp["message"])
	}
	if resp["message"] != "gateway config saved" {
		t.Fatalf("expected message 'gateway config saved', got %v", resp["message"])
	}
	if resp["data"] == nil {
		t.Fatal("expected data in response")
	}
}

// ---------------------------------------------------------------------------
// Test 6: key_masked not written back as real key
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_MaskedKeyNotWrittenBack(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigWithFreeProviderJSON)
	defer cleanup()

	originalKeys := fallback.GetConfig().FreeProviders["groq"].Keys
	if len(originalKeys) == 0 {
		t.Fatal("expected initial config to have groq keys")
	}
	originalKey := originalKeys[0]

	// Send a masked key (contains *) — the handler should skip it and preserve
	// the original.
	putPayload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "quality_first",
				"pools": ["high"]
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high"
			}
		},
		"free_providers": {
			"groq": {
				"enabled": true,
				"keys": ["gsk_****_masked_not_real"]
			}
		}
	}`

	w := callGatewayPUT(t, putPayload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d\nbody: %s", w.Code, w.Body.String())
	}

	// Verify the original key was preserved (not overwritten with masked value)
	cfg := fallback.GetConfig()
	groqFP, ok := cfg.FreeProviders["groq"]
	if !ok {
		t.Fatal("expected groq free_provider in config after save")
	}
	if len(groqFP.Keys) != 1 {
		t.Fatalf("expected 1 key after masked-key save, got %d", len(groqFP.Keys))
	}
	if groqFP.Keys[0] != originalKey {
		t.Fatalf("expected original key %q to be preserved, got %q", originalKey, groqFP.Keys[0])
	}

	// Also check the response: key_count should still be 1
	resp := parseJSON(t, w)
	data, _ := resp["data"].(map[string]interface{})
	fps, _ := data["free_providers"].(map[string]interface{})
	groq, _ := fps["groq"].(map[string]interface{})
	if groq["key_count"] != float64(1) {
		t.Fatalf("expected key_count=1 in response, got %v", groq["key_count"])
	}
}

// ---------------------------------------------------------------------------
// Test 7: empty key preserves old key
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_EmptyKeyPreservesOld(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigWithFreeProviderJSON)
	defer cleanup()

	originalKeys := fallback.GetConfig().FreeProviders["groq"].Keys
	if len(originalKeys) == 0 {
		t.Fatal("expected initial config to have groq keys")
	}
	originalKey := originalKeys[0]

	// Send free_provider without keys field (omitempty) — existing keys kept.
	putPayload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "quality_first",
				"pools": ["high"]
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high"
			}
		},
		"free_providers": {
			"groq": {
				"enabled": true
			}
		}
	}`

	w := callGatewayPUT(t, putPayload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d\nbody: %s", w.Code, w.Body.String())
	}

	cfg := fallback.GetConfig()
	groqFP := cfg.FreeProviders["groq"]
	if len(groqFP.Keys) != 1 || groqFP.Keys[0] != originalKey {
		t.Fatalf("expected original key %q preserved when empty keys sent, got %v",
			originalKey, groqFP.Keys)
	}
}

// ---------------------------------------------------------------------------
// Test 8: new key can update
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_NewKeyCanUpdate(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigWithFreeProviderJSON)
	defer cleanup()

	newKey := "gsk_brand_new_real_key_99999999"
	putPayload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "quality_first",
				"pools": ["high"]
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high"
			}
		},
		"free_providers": {
			"groq": {
				"enabled": true,
				"keys": ["` + newKey + `"]
			}
		}
	}`

	w := callGatewayPUT(t, putPayload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d\nbody: %s", w.Code, w.Body.String())
	}

	cfg := fallback.GetConfig()
	groqFP := cfg.FreeProviders["groq"]
	if len(groqFP.Keys) != 1 {
		t.Fatalf("expected 1 key after update, got %d", len(groqFP.Keys))
	}
	if groqFP.Keys[0] != newKey {
		t.Fatalf("expected key to be updated to %q, got %q", newKey, groqFP.Keys[0])
	}
}

// ---------------------------------------------------------------------------
// Test 9: limits_override negative → 400
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_NegativeLimitsOverrideRejected(t *testing.T) {
	// The limits check happens before file I/O, so read-only config suffices.
	setupGatewayConfigReadOnly(t, baseValidConfigJSON)

	neg := -1
	tests := []struct {
		name  string
		field string
	}{
		{"negative rpm_limit", "rpm_limit"},
		{"negative rpd_limit", "rpd_limit"},
		{"negative tpm_limit", "tpm_limit"},
		{"negative tpd_limit", "tpd_limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"enabled": true,
				"virtual_models": map[string]interface{}{
					"test/auto": map[string]interface{}{
						"enabled":  true,
						"strategy": "quality_first",
						"pools":    []string{"high"},
					},
				},
				"deployments": map[string]interface{}{
					"dep-1": map[string]interface{}{
						"enabled":    true,
						"channel_id": 1,
						"real_model": "gpt-4",
						"pool":       "high",
					},
				},
				"free_providers": map[string]interface{}{
					"groq": map[string]interface{}{
						"enabled": true,
						"limits_override": map[string]interface{}{
							tt.field: neg,
						},
					},
				},
			}
			body, _ := json.Marshal(payload)
			w := callGatewayPUT(t, string(body))

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d\nbody: %s",
					tt.name, w.Code, w.Body.String())
			}
			resp := parseJSON(t, w)
			if resp["success"] != false {
				t.Fatalf("expected success=false for %s", tt.name)
			}
			msg, _ := resp["message"].(string)
			if !searchString(msg, "must be >= 0") {
				t.Fatalf("expected message to mention 'must be >= 0', got %q", msg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 10: valid limits_override → passes (zero and positive values)
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_ValidLimitsOverrideAccepted(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
	defer cleanup()

	tests := []struct {
		name     string
		override map[string]interface{}
	}{
		{
			"zero values (unlimited)",
			map[string]interface{}{
				"rpm_limit": 0,
				"rpd_limit": 0,
				"tpm_limit": 0,
				"tpd_limit": 0,
			},
		},
		{
			"positive values",
			map[string]interface{}{
				"rpm_limit": 30,
				"rpd_limit": 500,
				"tpm_limit": 6000,
				"tpd_limit": 100000,
			},
		},
		{
			"partial override (only rpm)",
			map[string]interface{}{
				"rpm_limit": 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Re-setup for each sub-test so each starts from a clean state.
			subCleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
			defer subCleanup()

			payload := map[string]interface{}{
				"enabled": true,
				"virtual_models": map[string]interface{}{
					"test/auto": map[string]interface{}{
						"enabled":  true,
						"strategy": "quality_first",
						"pools":    []string{"high"},
					},
				},
				"deployments": map[string]interface{}{
					"dep-1": map[string]interface{}{
						"enabled":    true,
						"channel_id": 1,
						"real_model": "gpt-4",
						"pool":       "high",
					},
				},
				"free_providers": map[string]interface{}{
					"groq": map[string]interface{}{
						"enabled":         true,
						"limits_override": tt.override,
					},
				},
			}
			body, _ := json.Marshal(payload)
			w := callGatewayPUT(t, string(body))

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d\nbody: %s",
					tt.name, w.Code, w.Body.String())
			}
			resp := parseJSON(t, w)
			if resp["success"] != true {
				t.Fatalf("expected success=true for %s, message: %v",
					tt.name, resp["message"])
			}

			// Verify limits_override was persisted
			data, _ := resp["data"].(map[string]interface{})
			fps, _ := data["free_providers"].(map[string]interface{})
			groq, _ := fps["groq"].(map[string]interface{})
			if groq == nil {
				t.Fatal("expected groq in free_providers response")
			}
			limitsOverride, _ := groq["limits_override"].(map[string]interface{})
			if limitsOverride == nil {
				t.Fatal("expected limits_override in groq response")
			}
			for k, expectedVal := range tt.override {
				gotVal, ok := limitsOverride[k]
				if !ok {
					t.Fatalf("expected %s in limits_override response", k)
				}
				// JSON numbers decode as float64
				if gotVal != float64(expectedVal.(int)) {
					t.Fatalf("expected %s=%v, got %v", k, expectedVal, gotVal)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Bonus: containsLegacyFields unit tests (pure function, no HTTP)
// ---------------------------------------------------------------------------

func TestContainsLegacyFields(t *testing.T) {
	tests := []struct {
		name string
		json string
		want bool
	}{
		{
			"clean payload",
			`{"enabled":true,"virtual_models":{"test/auto":{"strategy":"quality_first"}}}`,
			false,
		},
		{
			"routing_mode at top level",
			`{"routing_mode":"weighted"}`,
			true,
		},
		{
			"fallback_order nested",
			`{"virtual_models":{"test/auto":{"fallback_order":["dep-1"]}}}`,
			true,
		},
		{
			"fixed_deployment deeply nested",
			`{"a":{"b":{"c":{"fixed_deployment":"dep-1"}}}}`,
			true,
		},
		{
			"legacy field in array element",
			`{"items":[{"routing_mode":"fixed"}]}`,
			true,
		},
		{
			"no legacy fields in complex payload",
			`{"enabled":true,"deployments":{"d1":{"pool":"high"}},"free_providers":{"groq":{"enabled":true}}}`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw interface{}
			if err := json.Unmarshal([]byte(tt.json), &raw); err != nil {
				t.Fatalf("failed to parse test JSON: %v", err)
			}
			got := containsLegacyFields(raw)
			if got != tt.want {
				t.Fatalf("containsLegacyFields(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Bonus: buildGatewayV2Config unit test
// ---------------------------------------------------------------------------

func TestBuildGatewayV2Config_ProjectsCorrectly(t *testing.T) {
	rpmOverride := 25
	cfg := &fallback.Config{
		Enabled: true,
		VirtualModels: map[string]fallback.VirtualModelConfig{
			"cct/auto": {
				Enabled:            true,
				Strategy:           fallback.StrategyCostFirst,
				Pools:              []string{"high", "cheap"},
				AllowDegradeToLow:  true,
				AllowDegradeToFree: false,
			},
		},
		Deployments: map[string]fallback.DeploymentConfig{
			"dep-a": {
				Enabled:   true,
				ChannelID: 10,
				RealModel: "gpt-4o",
				Pool:      "high",
				Priority:  1,
				Weight:    150,
			},
		},
		FreeProviders: map[string]fallback.FreeProviderConfig{
			"groq": {
				Enabled: true,
				Keys:    []string{"key-1", "key-2"},
				LimitsOverride: &fallback.FreeProviderLimits{
					RPMLimit: &rpmOverride,
				},
			},
		},
	}

	v2 := buildGatewayV2Config(cfg)

	if !v2.Enabled {
		t.Fatal("expected enabled=true")
	}

	// Virtual model
	vm, ok := v2.VirtualModels["cct/auto"]
	if !ok {
		t.Fatal("expected virtual model cct/auto")
	}
	if vm.Strategy != "cost_first" {
		t.Fatalf("expected strategy cost_first, got %s", vm.Strategy)
	}
	if len(vm.Pools) != 2 || vm.Pools[0] != "high" || vm.Pools[1] != "cheap" {
		t.Fatalf("expected pools [high cheap], got %v", vm.Pools)
	}
	if !vm.AllowDegradeToLow {
		t.Fatal("expected allow_degrade_to_low=true")
	}

	// Deployment
	dep, ok := v2.Deployments["dep-a"]
	if !ok {
		t.Fatal("expected deployment dep-a")
	}
	if dep.Pool != "high" {
		t.Fatalf("expected pool=high, got %s", dep.Pool)
	}
	if dep.Weight != 150 {
		t.Fatalf("expected weight=150, got %d", dep.Weight)
	}

	// Free provider
	fp, ok := v2.FreeProviders["groq"]
	if !ok {
		t.Fatal("expected free provider groq")
	}
	if fp.KeyCount != 2 {
		t.Fatalf("expected key_count=2, got %d", fp.KeyCount)
	}
	if fp.LimitsOverride == nil || fp.LimitsOverride.RPMLimit == nil {
		t.Fatal("expected limits_override.rpm_limit")
	}
	if *fp.LimitsOverride.RPMLimit != 25 {
		t.Fatalf("expected rpm_limit=25, got %d", *fp.LimitsOverride.RPMLimit)
	}
}

// ---------------------------------------------------------------------------
// Bonus: toFreeProviderLimits unit test
// ---------------------------------------------------------------------------

func TestToFreeProviderLimits(t *testing.T) {
	// nil input → nil output
	if got := toFreeProviderLimits(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}

	rpm := 10
	tpd := 500
	input := &gatewayV2LimitsOverride{
		RPMLimit: &rpm,
		TPDLimit: &tpd,
	}
	got := toFreeProviderLimits(input)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.RPMLimit == nil || *got.RPMLimit != 10 {
		t.Fatalf("expected rpm_limit=10, got %v", got.RPMLimit)
	}
	if got.RPDLimit != nil {
		t.Fatalf("expected rpd_limit=nil, got %v", got.RPDLimit)
	}
	if got.TPDLimit == nil || *got.TPDLimit != 500 {
		t.Fatalf("expected tpd_limit=500, got %v", got.TPDLimit)
	}
}

// ---------------------------------------------------------------------------
// Edge: invalid JSON body → 400
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_InvalidJSON(t *testing.T) {
	setupGatewayConfigReadOnly(t, baseValidConfigJSON)

	w := callGatewayPUT(t, `{not valid json`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
	resp := parseJSON(t, w)
	if resp["success"] != false {
		t.Fatal("expected success=false for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// Edge: legacy field nested inside deployment → 400
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_LegacyFieldInDeployment(t *testing.T) {
	setupGatewayConfigReadOnly(t, baseValidConfigJSON)

	// Even if a legacy field appears deep inside a deployment object, it must
	// be rejected because containsLegacyFields recurses into all objects.
	payload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "quality_first",
				"pools": ["high"]
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high",
				"fixed_deployment": "dep-2"
			}
		}
	}`

	w := callGatewayPUT(t, payload)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for legacy field in deployment, got %d\nbody: %s",
			w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Edge: deployment weight defaults to 100 when zero
// ---------------------------------------------------------------------------

func TestGatewayUpdateConfig_DeploymentWeightDefault(t *testing.T) {
	cleanup := setupGatewayConfigForSave(t, baseValidConfigJSON)
	defer cleanup()

	putPayload := `{
		"enabled": true,
		"virtual_models": {
			"test/auto": {
				"enabled": true,
				"strategy": "quality_first",
				"pools": ["high"]
			}
		},
		"deployments": {
			"dep-1": {
				"enabled": true,
				"channel_id": 1,
				"real_model": "gpt-4",
				"pool": "high",
				"weight": 0
			}
		}
	}`

	w := callGatewayPUT(t, putPayload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}

	cfg := fallback.GetConfig()
	dep := cfg.Deployments["dep-1"]
	if dep.Weight != 100 {
		t.Fatalf("expected default weight=100, got %d", dep.Weight)
	}
}
