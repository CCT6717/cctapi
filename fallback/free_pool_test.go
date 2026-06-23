package fallback

import (
	"strings"
	"testing"

	"github.com/songquanpeng/one-api/relay/channeltype"
)

func TestBuiltinFreeProviderRegistry_OpenRouter(t *testing.T) {
	meta, ok := BuiltinFreeProviders["openrouter"]
	if !ok {
		t.Fatal("expected openrouter in BuiltinFreeProviders")
	}
	if meta.ChannelType != channeltype.OpenRouter {
		t.Errorf("expected channel type %d, got %d", channeltype.OpenRouter, meta.ChannelType)
	}
	if meta.DefaultBaseURL != "https://openrouter.ai/api" {
		t.Errorf("unexpected base URL: %s", meta.DefaultBaseURL)
	}
	if len(meta.DefaultModels) != 1 || meta.DefaultModels[0] != "openrouter/free" {
		t.Errorf("unexpected default models: %v", meta.DefaultModels)
	}
	if meta.DefaultRPM <= 0 {
		t.Errorf("expected DefaultRPM > 0, got %d", meta.DefaultRPM)
	}
}

func TestBuiltinFreeProviderRegistry_Groq(t *testing.T) {
	meta, ok := BuiltinFreeProviders["groq"]
	if !ok {
		t.Fatal("expected groq in BuiltinFreeProviders")
	}
	if meta.ChannelType != channeltype.Groq {
		t.Errorf("expected channel type %d, got %d", channeltype.Groq, meta.ChannelType)
	}
	if meta.DefaultBaseURL != "https://api.groq.com/openai" {
		t.Errorf("unexpected base URL: %s", meta.DefaultBaseURL)
	}
	if len(meta.DefaultModels) == 0 {
		t.Error("expected non-empty DefaultModels")
	}
	if !meta.SupportsTools {
		t.Error("expected groq to support tools")
	}
	if !meta.SupportsJSON {
		t.Error("expected groq to support JSON mode")
	}
}

// TestBuiltinFreeProviderRegistry_AllHaveRealModel verifies every provider in
// BuiltinFreeProviders has a non-empty real_model (DefaultModels[0]).
// Providers with empty DefaultModels (dynamic fetch) are skipped.
func TestBuiltinFreeProviderRegistry_AllHaveRealModel(t *testing.T) {
	for name, meta := range BuiltinFreeProviders {
		if len(meta.DefaultModels) == 0 {
			continue // dynamic fetch, no static default
		}
		if meta.DefaultModels[0] == "" {
			t.Errorf("provider %q: DefaultModels[0] is empty string", name)
		}
	}
}

// TestBuiltinFreeProviderRegistry_AllLimitsNonNegative verifies all limit
// defaults are non-negative. Zero is valid (unlimited).
func TestBuiltinFreeProviderRegistry_AllLimitsNonNegative(t *testing.T) {
	for name, meta := range BuiltinFreeProviders {
		if meta.DefaultRPM < 0 {
			t.Errorf("provider %q: DefaultRPM=%d is negative", name, meta.DefaultRPM)
		}
		if meta.DefaultRPD < 0 {
			t.Errorf("provider %q: DefaultRPD=%d is negative", name, meta.DefaultRPD)
		}
		if meta.DefaultTPM < 0 {
			t.Errorf("provider %q: DefaultTPM=%d is negative", name, meta.DefaultTPM)
		}
		if meta.DefaultTPD < 0 {
			t.Errorf("provider %q: DefaultTPD=%d is negative", name, meta.DefaultTPD)
		}
	}
}

// TestBuiltinFreeProviderRegistry_DeploymentPoolFields verifies the deployment
// config constructed by SyncFreePool always has pool=free, cost_tier=free, and
// quota_mode=free — these are hardcoded in SyncFreePool and must not drift.
func TestBuiltinFreeProviderRegistry_DeploymentPoolFields(t *testing.T) {
	for name, meta := range BuiltinFreeProviders {
		if len(meta.DefaultModels) == 0 {
			continue // dynamic fetch, no static default model
		}
		realModel := meta.DefaultModels[0]
		dep := DeploymentConfig{
			RealModel:             realModel,
			Pool:                  "free",
			QualityTier:           "medium",
			CostTier:              "free",
			SupportsVision:        meta.SupportsVision,
			SupportsStream:        meta.SupportsStream,
			SupportsTools:         meta.SupportsTools,
			SupportsJSON:          meta.SupportsJSON,
			ContextLength:         meta.ContextLength,
			Priority:              10,
			Weight:                100,
			MaxConcurrentRequests: 5,
			QuotaMode:             "free",
			SoftLimitRatio:        0.95,
			HardLimitRatio:        1.0,
			RPMLimit:              meta.DefaultRPM,
			RPDLimit:              meta.DefaultRPD,
			TPMLimit:              meta.DefaultTPM,
			TPDLimit:              meta.DefaultTPD,
		}
		if dep.Pool != "free" {
			t.Errorf("provider %q: Pool=%q, want \"free\"", name, dep.Pool)
		}
		if dep.CostTier != "free" {
			t.Errorf("provider %q: CostTier=%q, want \"free\"", name, dep.CostTier)
		}
		if dep.QuotaMode != "free" {
			t.Errorf("provider %q: QuotaMode=%q, want \"free\"", name, dep.QuotaMode)
		}
	}
}

// TestBuiltinFreeProviderRegistry_DisabledProviderNoDeployment checks that
// computeExpectedAutoResources returns zero results when the only provider
// is disabled. Note: SyncFreePool also skips disabled providers at
// `if !fp.Enabled || len(fp.Keys) == 0 { continue }`.
func TestBuiltinFreeProviderRegistry_DisabledProviderNoDeployment(t *testing.T) {
	cfg := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {Enabled: false, Keys: []string{"sk-groq-test"}},
		},
	}
	channels, deployments := computeExpectedAutoResources(cfg)
	if len(channels) != 0 {
		t.Errorf("expected 0 channels for disabled provider, got %d", len(channels))
	}
	if len(deployments) != 0 {
		t.Errorf("expected 0 deployments for disabled provider, got %d", len(deployments))
	}
}

func TestValidateFreeProviderName(t *testing.T) {
	if err := ValidateFreeProviderName("openrouter"); err != nil {
		t.Errorf("expected no error for openrouter, got: %v", err)
	}
	if err := ValidateFreeProviderName("groq"); err != nil {
		t.Errorf("expected no error for groq, got: %v", err)
	}
	if err := ValidateFreeProviderName("nonexistent"); err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestIsAutoDeploymentID_OldIndex(t *testing.T) {
	// Old format: integer index
	if !IsAutoDeploymentID("free:openrouter-0") {
		t.Error("expected free:openrouter-0 to be auto")
	}
	if !IsAutoDeploymentID("free:groq-12") {
		t.Error("expected free:groq-12 to be auto")
	}
	// User-created free:* IDs → NOT auto
	if IsAutoDeploymentID("free:my-custom") {
		t.Error("expected free:my-custom NOT to be auto")
	}
	if IsAutoDeploymentID("free:custom-abc") {
		t.Error("expected free:custom-abc NOT to be auto")
	}
	if IsAutoDeploymentID("free:-0") {
		t.Error("expected free:-0 NOT to be auto")
	}
	// Non-free prefix → NOT auto
	if IsAutoDeploymentID("cct/gemini") {
		t.Error("expected cct/gemini NOT to be auto")
	}
	if IsAutoDeploymentID("free:unknown-0") {
		t.Error("expected free:unknown-0 NOT to be auto (unknown provider)")
	}
}

func TestIsAutoDeploymentID_HashFormat(t *testing.T) {
	// New format: 8-char hex hash
	if !IsAutoDeploymentID("free:openrouter-a1b2c3d4") {
		t.Error("expected free:openrouter-a1b2c3d4 to be auto (hash format)")
	}
	if !IsAutoDeploymentID("free:groq-001122ff") {
		t.Error("expected free:groq-001122ff to be auto (hash format)")
	}
	if !IsAutoDeploymentID("free:openrouter-deadbeef") {
		t.Error("expected free:openrouter-deadbeef to be auto (hash format)")
	}
	// Wrong hash length → not auto
	if IsAutoDeploymentID("free:openrouter-abc") {
		t.Error("expected free:openrouter-abc NOT to be auto (too short)")
	}
	if IsAutoDeploymentID("free:openrouter-a1b2c3d4e5") {
		t.Error("expected free:openrouter-a1b2c3d4e5 NOT to be auto (too long)")
	}
	// Invalid hex → not auto
	if IsAutoDeploymentID("free:openrouter-zzzzzzzz") {
		t.Error("expected free:openrouter-zzzzzzzz NOT to be auto (invalid hex)")
	}
}

func TestIsAutoChannelName_OldIndex(t *testing.T) {
	// Old format: integer index
	if !IsAutoChannelName("[CCT Auto] openrouter-0") {
		t.Error("expected [CCT Auto] openrouter-0 to be auto")
	}
	if !IsAutoChannelName("[CCT Auto] groq-5") {
		t.Error("expected [CCT Auto] groq-5 to be auto")
	}
	// User-created or malformed → NOT auto
	if IsAutoChannelName("[CCT Auto] custom-channel") {
		t.Error("expected [CCT Auto] custom-channel NOT to be auto")
	}
	if IsAutoChannelName("[CCT Auto] unknown-0") {
		t.Error("expected [CCT Auto] unknown-0 NOT to be auto (unknown provider)")
	}
	if IsAutoChannelName("custom-channel") {
		t.Error("expected custom-channel NOT to be auto")
	}
	if IsAutoChannelName("[CCT Auto] -0") {
		t.Error("expected [CCT Auto] -0 NOT to be auto")
	}
}

func TestIsAutoChannelName_HashFormat(t *testing.T) {
	// New format: 8-char hex hash
	if !IsAutoChannelName("[CCT Auto] openrouter-a1b2c3d4") {
		t.Error("expected [CCT Auto] openrouter-a1b2c3d4 to be auto (hash format)")
	}
	if !IsAutoChannelName("[CCT Auto] groq-001122ff") {
		t.Error("expected [CCT Auto] groq-001122ff to be auto (hash format)")
	}
	// Wrong hash length → not auto
	if IsAutoChannelName("[CCT Auto] openrouter-abc") {
		t.Error("expected [CCT Auto] openrouter-abc NOT to be auto (too short)")
	}
	// Invalid hex → not auto
	if IsAutoChannelName("[CCT Auto] openrouter-zzzzzzzz") {
		t.Error("expected [CCT Auto] openrouter-zzzzzzzz NOT to be auto (invalid hex)")
	}
}

func TestChannelName(t *testing.T) {
	got := channelName("openrouter", "a1b2c3d4")
	want := "[CCT Auto] openrouter-a1b2c3d4"
	if got != want {
		t.Errorf("channelName() = %q, want %q", got, want)
	}
}

func TestDeploymentID(t *testing.T) {
	got := deploymentID("groq", "001122ff")
	want := "free:groq-001122ff"
	if got != want {
		t.Errorf("deploymentID() = %q, want %q", got, want)
	}
}

func TestSafeKeyHash_Stable(t *testing.T) {
	key := "sk-or-v1-test-key-12345"
	h1 := SafeKeyHash(key)
	h2 := SafeKeyHash(key)
	if h1 != h2 {
		t.Errorf("SafeKeyHash should be stable, got %q != %q", h1, h2)
	}
}

func TestSafeKeyHash_DifferentKeys(t *testing.T) {
	h1 := SafeKeyHash("key-one")
	h2 := SafeKeyHash("key-two")
	if h1 == h2 {
		t.Error("SafeKeyHash should produce different hashes for different keys")
	}
}

func TestSafeKeyHash_NotExposeKey(t *testing.T) {
	key := "sk-or-v1-secret-api-key-44599c8700066e2a8d4fe6d6dbc40179"
	hash := SafeKeyHash(key)
	if strings.Contains(hash, "sk-or") || strings.Contains(hash, "secret") {
		t.Errorf("SafeKeyHash should not expose original key, got %q", hash)
	}
	if len(hash) != 8 {
		t.Errorf("SafeKeyHash should return 8 chars, got %d: %q", len(hash), hash)
	}
	// Must be valid hex
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("SafeKeyHash should be hex, got invalid char %c in %q", c, hash)
		}
	}
}

func TestSafeKeyHash_KeyReorderStability(t *testing.T) {
	// Simulating array index-based naming vs hash-based naming:
	// Old: keys[0] → "-0", keys[1] → "-1"
	// New: keys[0] → SafeKeyHash(keys[0]), keys[1] → SafeKeyHash(keys[1])
	// Verify that removing a key from the middle doesn't change remaining hashes
	keys := []string{"key-aaa", "key-bbb", "key-ccc"}
	hashes := make([]string, len(keys))
	for i, k := range keys {
		hashes[i] = SafeKeyHash(k)
	}

	// All three should be different
	if hashes[0] == hashes[1] || hashes[1] == hashes[2] || hashes[0] == hashes[2] {
		t.Error("expected all three keys to produce different hashes")
	}

	// Simulate removing key-aaa (index 0): old would shift bbb→0 and ccc→1
	// New: hashes of remaining keys stay unchanged
	remainingKeys := keys[1:] // ["key-bbb", "key-ccc"]
	for i, k := range remainingKeys {
		h := SafeKeyHash(k)
		if h != hashes[i+1] {
			t.Errorf("key %q hash changed after removal of another key: was %q, now %q", k, hashes[i+1], h)
		}
	}
}

func TestApplyLimitsOverride_NilOverride(t *testing.T) {
	rpm, rpd, tpm, tpd := ApplyLimitsOverride(20, 500, 4000, 100000, nil)
	if rpm != 20 || rpd != 500 || tpm != 4000 || tpd != 100000 {
		t.Errorf("nil override should not change values, got (%d,%d,%d,%d)", rpm, rpd, tpm, tpd)
	}
}

func TestApplyLimitsOverride_SingleField(t *testing.T) {
	override := &FreeProviderLimits{RPMLimit: intPtr(10)}
	rpm, rpd, tpm, tpd := ApplyLimitsOverride(20, 500, 4000, 100000, override)
	if rpm != 10 {
		t.Errorf("expected rpm=10, got %d", rpm)
	}
	if rpd != 500 || tpm != 4000 || tpd != 100000 {
		t.Error("non-overridden fields should remain unchanged")
	}
}

func TestApplyLimitsOverride_AllFields(t *testing.T) {
	override := &FreeProviderLimits{
		RPMLimit: intPtr(5), RPDLimit: intPtr(100),
		TPMLimit: intPtr(2000), TPDLimit: intPtr(50000),
	}
	rpm, rpd, tpm, tpd := ApplyLimitsOverride(20, 500, 4000, 100000, override)
	if rpm != 5 || rpd != 100 || tpm != 2000 || tpd != 50000 {
		t.Errorf("all fields should be overridden, got (%d,%d,%d,%d)", rpm, rpd, tpm, tpd)
	}
}

func TestApplyLimitsOverride_ZeroOverride(t *testing.T) {
	override := &FreeProviderLimits{RPMLimit: intPtr(0), RPDLimit: intPtr(0)}
	rpm, rpd, tpm, tpd := ApplyLimitsOverride(20, 500, 4000, 100000, override)
	if rpm != 0 || rpd != 0 {
		t.Errorf("zero override should set to 0, got rpm=%d, rpd=%d", rpm, rpd)
	}
	if tpm != 4000 || tpd != 100000 {
		t.Error("non-overridden fields should remain unchanged")
	}
}

func TestValidateFreeProviderLimits_Nil(t *testing.T) {
	if err := ValidateFreeProviderLimits(nil); err != nil {
		t.Errorf("nil should pass, got: %v", err)
	}
}

func TestValidateFreeProviderLimits_Valid(t *testing.T) {
	l := &FreeProviderLimits{RPMLimit: intPtr(10), RPDLimit: intPtr(0), TPMLimit: intPtr(5000), TPDLimit: intPtr(99999)}
	if err := ValidateFreeProviderLimits(l); err != nil {
		t.Errorf("valid limits should pass, got: %v", err)
	}
}

func TestValidateFreeProviderLimits_NegativeRPM(t *testing.T) {
	l := &FreeProviderLimits{RPMLimit: intPtr(-1)}
	if err := ValidateFreeProviderLimits(l); err == nil {
		t.Error("negative rpm should fail")
	}
}

func TestValidateFreeProviderLimits_NegativeRPD(t *testing.T) {
	l := &FreeProviderLimits{RPDLimit: intPtr(-5)}
	if err := ValidateFreeProviderLimits(l); err == nil {
		t.Error("negative rpd should fail")
	}
}

func TestValidateFreeProviderLimits_NegativeTPM(t *testing.T) {
	l := &FreeProviderLimits{TPMLimit: intPtr(-10)}
	if err := ValidateFreeProviderLimits(l); err == nil {
		t.Error("negative tpm should fail")
	}
}

func TestValidateFreeProviderLimits_NegativeTPD(t *testing.T) {
	l := &FreeProviderLimits{TPDLimit: intPtr(-1)}
	if err := ValidateFreeProviderLimits(l); err == nil {
		t.Error("negative tpd should fail")
	}
}

// ── Multi-key scenario tests ──

func TestMultiKey_TwoKeysIndependentNames(t *testing.T) {
	// Two keys (keyA/keyB) coexisting should produce independent,
	// uniquely identifiable channel names and deployment IDs.
	keys := []string{"sk-or-v1-key-a-abc123", "sk-or-v1-key-b-def456"}
	hashes := make([]string, len(keys))
	for i, k := range keys {
		hashes[i] = SafeKeyHash(k)
	}
	if hashes[0] == hashes[1] {
		t.Fatal("keys should produce different hashes")
	}

	// Verify channel names are unique and recognized as auto channels
	chA := channelName("openrouter", hashes[0])
	chB := channelName("openrouter", hashes[1])
	if chA == chB {
		t.Error("channel names should differ for different keys")
	}
	if !IsAutoChannelName(chA) || !IsAutoChannelName(chB) {
		t.Error("both auto channel names should be recognized by IsAutoChannelName")
	}

	// Verify deployment IDs are unique and recognized as auto deployments
	depA := deploymentID("openrouter", hashes[0])
	depB := deploymentID("openrouter", hashes[1])
	if depA == depB {
		t.Error("deployment IDs should differ for different keys")
	}
	if !IsAutoDeploymentID(depA) || !IsAutoDeploymentID(depB) {
		t.Error("both auto deployment IDs should be recognized by IsAutoDeploymentID")
	}
}

func TestMultiKey_RemoveKeyDesiredList(t *testing.T) {
	// Start with 3 keys, then remove one. Verify desired resources list
	// no longer includes the removed key's channel and deployment.
	cfg := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"openrouter": {Enabled: true, Keys: []string{"key-a", "key-b", "key-c"}},
		},
	}
	channels, deployments := computeExpectedAutoResources(cfg)
	if len(channels) != 3 || len(deployments) != 3 {
		t.Fatalf("expected 3 channels and 3 deployments, got %d, %d", len(channels), len(deployments))
	}

	// Remove key-a
	cfg2 := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"openrouter": {Enabled: true, Keys: []string{"key-b", "key-c"}},
		},
	}
	channels2, deployments2 := computeExpectedAutoResources(cfg2)
	if len(channels2) != 2 || len(deployments2) != 2 {
		t.Fatalf("expected 2 channels and 2 deployments after removal, got %d, %d", len(channels2), len(deployments2))
	}

	// Removed key's resources should NOT appear in desired list
	removedHash := SafeKeyHash("key-a")
	if channels2[channelName("openrouter", removedHash)] {
		t.Error("removed key's channel should not be in desired list")
	}
	if deployments2[deploymentID("openrouter", removedHash)] {
		t.Error("removed key's deployment should not be in desired list")
	}

	// Remaining keys' resources should still be present with same names
	for _, key := range []string{"key-b", "key-c"} {
		kh := SafeKeyHash(key)
		if !channels2[channelName("openrouter", kh)] {
			t.Errorf("remaining key %q channel should still be in desired list", key)
		}
		if !deployments2[deploymentID("openrouter", kh)] {
			t.Errorf("remaining key %q deployment should still be in desired list", key)
		}
	}
}

func TestMultiKey_AddKeyDesiredList(t *testing.T) {
	// Start with 2 keys, add a third. Verify existing resources stay and
	// the new key gets its own resources in the desired list.
	cfg := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {Enabled: true, Keys: []string{"key-x", "key-y"}},
		},
	}
	channels, deployments := computeExpectedAutoResources(cfg)
	if len(channels) != 2 || len(deployments) != 2 {
		t.Fatalf("expected 2 channels and 2 deployments, got %d, %d", len(channels), len(deployments))
	}

	// Add third key
	cfg2 := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"groq": {Enabled: true, Keys: []string{"key-x", "key-y", "key-z"}},
		},
	}
	channels2, deployments2 := computeExpectedAutoResources(cfg2)
	if len(channels2) != 3 || len(deployments2) != 3 {
		t.Fatalf("expected 3 channels and 3 deployments after adding key, got %d, %d", len(channels2), len(deployments2))
	}

	// Original keys still present
	for _, key := range []string{"key-x", "key-y"} {
		kh := SafeKeyHash(key)
		if !channels2[channelName("groq", kh)] {
			t.Errorf("original key %q channel should still be in desired list", key)
		}
		if !deployments2[deploymentID("groq", kh)] {
			t.Errorf("original key %q deployment should still be in desired list", key)
		}
	}

	// New key gets its own resources
	newHash := SafeKeyHash("key-z")
	if !channels2[channelName("groq", newHash)] {
		t.Error("new key's channel should be in desired list")
	}
	if !deployments2[deploymentID("groq", newHash)] {
		t.Error("new key's deployment should be in desired list")
	}
}

func TestMultiKey_ProviderDisabledDesiredList(t *testing.T) {
	// All providers enabled → desired list contains all 3 keys
	cfg := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"openrouter": {Enabled: true, Keys: []string{"key-a", "key-b"}},
			"groq":       {Enabled: true, Keys: []string{"key-c"}},
		},
	}
	channels, deployments := computeExpectedAutoResources(cfg)
	if len(channels) != 3 || len(deployments) != 3 {
		t.Fatalf("expected 3 channels and 3 deployments, got %d, %d", len(channels), len(deployments))
	}

	// Disable openrouter → only groq resources in desired list
	cfg2 := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"openrouter": {Enabled: false, Keys: []string{"key-a", "key-b"}},
			"groq":       {Enabled: true, Keys: []string{"key-c"}},
		},
	}
	channels2, deployments2 := computeExpectedAutoResources(cfg2)
	if len(channels2) != 1 || len(deployments2) != 1 {
		t.Fatalf("expected 1 channel and 1 deployment after disabling provider, got %d, %d", len(channels2), len(deployments2))
	}

	// Openrouter resources should NOT be in desired list
	for _, key := range []string{"key-a", "key-b"} {
		kh := SafeKeyHash(key)
		if channels2[channelName("openrouter", kh)] {
			t.Errorf("disabled provider key %q channel should not be in desired list", key)
		}
		if deployments2[deploymentID("openrouter", kh)] {
			t.Errorf("disabled provider key %q deployment should not be in desired list", key)
		}
	}

	// Groq still present
	groqHash := SafeKeyHash("key-c")
	if !channels2[channelName("groq", groqHash)] {
		t.Error("enabled provider's channel should still be in desired list")
	}
	if !deployments2[deploymentID("groq", groqHash)] {
		t.Error("enabled provider's deployment should still be in desired list")
	}

	// Disable all providers → empty desired list
	cfg3 := &Config{
		FreeProviders: map[string]FreeProviderConfig{
			"openrouter": {Enabled: false, Keys: []string{"key-a", "key-b"}},
			"groq":       {Enabled: false, Keys: []string{"key-c"}},
		},
	}
	channels3, deployments3 := computeExpectedAutoResources(cfg3)
	if len(channels3) != 0 || len(deployments3) != 0 {
		t.Fatalf("expected 0 channels and 0 deployments when all disabled, got %d, %d", len(channels3), len(deployments3))
	}
}

func intPtr(v int) *int {
	return &v
}

// ===== 缺口2+4 单测:纯函数,不打真实 HTTP =====

func TestParseFreeModels_OpenRouterFreeFilter(t *testing.T) {
	body := []byte(`{"data":[
		{"id":"meta-llama/llama-3-8b-instruct:free"},
		{"id":"deepseek/deepseek-chat:free"},
		{"id":"openai/gpt-4"},
		{"id":"meta-llama/llama-3-8b-instruct:free"},
		{"id":"google/gemini-2.0-flash-exp:free"},
		{"id":"anthropic/claude-3-opus"}
	]}`)
	free, err := parseFreeModels(body)
	if err != nil {
		t.Fatalf("parseFreeModels error: %v", err)
	}
	// 过滤 :free,去重,排序
	want := []string{
		"deepseek/deepseek-chat:free",
		"google/gemini-2.0-flash-exp:free",
		"meta-llama/llama-3-8b-instruct:free",
	}
	if len(free) != len(want) {
		t.Fatalf("expected %d free models, got %d: %v", len(want), len(free), free)
	}
	for i, m := range free {
		if m != want[i] {
			t.Errorf("free[%d] = %q, want %q", i, m, want[i])
		}
	}
}

func TestParseFreeModels_EmptyAndNoFree(t *testing.T) {
	// 无 :free 模型 → 空切片,无 error
	free, err := parseFreeModels([]byte(`{"data":[{"id":"openai/gpt-4"},{"id":"anthropic/claude-3"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(free) != 0 {
		t.Errorf("expected 0 free models, got %v", free)
	}
	// 空 data → 空切片
	free, err = parseFreeModels([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("unexpected error on empty: %v", err)
	}
	if len(free) != 0 {
		t.Errorf("expected 0 free models on empty, got %v", free)
	}
}

func TestParseCreditsBalance_TokenCalc(t *testing.T) {
	// total_credits=10.0, total_usage=2.5 → balance=7.5
	body := []byte(`{"data":{"total_credits":10.0,"total_usage":2.5}}`)
	balance, err := parseCreditsBalance(body)
	if err != nil {
		t.Fatalf("parseCreditsBalance error: %v", err)
	}
	if balance != 7.5 {
		t.Errorf("balance = %v, want 7.5", balance)
	}
	// tokens 换算: balance * 1e6 / 7.5 = 7.5*1e6/7.5 = 1e6
	tokens := int64(balance * 1_000_000 / 7.5)
	if tokens != 1_000_000 {
		t.Errorf("tokens = %d, want 1000000", tokens)
	}
}

func TestParseCreditsBalance_ZeroUsage(t *testing.T) {
	// 全额未用 → balance = total_credits
	body := []byte(`{"data":{"total_credits":5.0,"total_usage":0.0}}`)
	balance, err := parseCreditsBalance(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 5.0 {
		t.Errorf("balance = %v, want 5.0", balance)
	}
}

func TestParseKiloFreeModels_FilterIsFree(t *testing.T) {
	// 模拟 Kilo /models 响应
	body := []byte(`{"data":[
		{"id":"kilo-auto/free","isFree":true,"name":"Auto Free"},
		{"id":"stepfun/step-3.7-flash:free","isFree":true,"name":"Step 3.7 Flash"},
		{"id":"nvidia/nemotron-3-ultra-550b-a55b:free","isFree":true,"name":"Nemotron 3 Ultra"},
		{"id":"openai/gpt-4","isFree":false,"name":"GPT-4"},
		{"id":"anthropic/claude-3","isFree":false,"name":"Claude 3"},
		{"id":"kilo-auto/free","isFree":true,"name":"Auto Free (duplicate)"}
	]}`)
	free, err := parseKiloFreeModels(body)
	if err != nil {
		t.Fatalf("parseKiloFreeModels error: %v", err)
	}
	// 应过滤 isFree:true，去重，排序
	want := []string{
		"kilo-auto/free",
		"nvidia/nemotron-3-ultra-550b-a55b:free",
		"stepfun/step-3.7-flash:free",
	}
	if len(free) != len(want) {
		t.Fatalf("len(free) = %d, want %d; got %v", len(free), len(want), free)
	}
	for i, m := range free {
		if m != want[i] {
			t.Errorf("free[%d] = %q, want %q", i, m, want[i])
		}
	}
}

func TestParseKiloFreeModels_EmptyAndNoFree(t *testing.T) {
	// 无 isFree:true 模型 → 空切片
	free, err := parseKiloFreeModels([]byte(`{"data":[{"id":"gpt-4","isFree":false}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(free) != 0 {
		t.Errorf("expected 0 free models, got %v", free)
	}
	// 空 data → 空切片
	free, err = parseKiloFreeModels([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("unexpected error on empty: %v", err)
	}
	if len(free) != 0 {
		t.Errorf("expected 0 free models on empty, got %v", free)
	}
}

func TestBuiltinFreeProviderRegistry_Kilo(t *testing.T) {
	meta, ok := BuiltinFreeProviders["kilo"]
	if !ok {
		t.Fatal("expected kilo in BuiltinFreeProviders")
	}
	if meta.ChannelType != channeltype.OpenAICompatible {
		t.Errorf("expected channel type %d, got %d", channeltype.OpenAICompatible, meta.ChannelType)
	}
	if meta.DefaultBaseURL != "https://api.kilo.ai/api/gateway/v1" {
		t.Errorf("unexpected base URL: %s", meta.DefaultBaseURL)
	}
	// Kilo 是动态拉取,DefaultModels 应为空
	if len(meta.DefaultModels) != 0 {
		t.Errorf("expected empty DefaultModels for dynamic provider, got %v", meta.DefaultModels)
	}
	if meta.DefaultRPM <= 0 {
		t.Errorf("expected DefaultRPM > 0, got %d", meta.DefaultRPM)
	}
	if meta.ContextLength != 256000 {
		t.Errorf("expected ContextLength 256000, got %d", meta.ContextLength)
	}
}
