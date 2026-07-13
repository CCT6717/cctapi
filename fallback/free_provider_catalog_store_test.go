package fallback

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFreeProviderCatalogStoreTestDB(t *testing.T) func() {
	t.Helper()
	originalDB := dbmodel.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open catalog test database: %v", err)
	}
	dbmodel.DB = db
	resetFreeProviderCatalogCacheForTest()
	if err := InitFreeProviderCatalogStore(); err != nil {
		t.Fatalf("InitFreeProviderCatalogStore: %v", err)
	}
	return func() {
		resetFreeProviderCatalogCacheForTest()
		dbmodel.DB = originalDB
	}
}

func TestCatalogStoreFailurePreservesLastSuccessfulSnapshot(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	successAt := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	tools := true
	snapshot := FreeProviderCatalogSnapshot{
		DeploymentID: "free:kilo-e3b0c442",
		Provider:     "kilo",
		Source:       ModelFetchKiloFree,
		Models: []FreeModelCatalogEntry{
			{ID: "kilo/tools:free", SupportsTools: &tools},
		},
		SelectedModel: "kilo/tools:free",
		LastAttemptAt: successAt,
		LastSuccessAt: successAt,
	}
	if err := saveFreeProviderCatalogSuccess(snapshot); err != nil {
		t.Fatalf("saveFreeProviderCatalogSuccess: %v", err)
	}

	failureAt := successAt.Add(time.Hour)
	if err := markFreeProviderCatalogFailure(
		snapshot.DeploymentID,
		snapshot.Provider,
		snapshot.Source,
		failureAt,
		errors.New("upstream timeout"),
	); err != nil {
		t.Fatalf("markFreeProviderCatalogFailure: %v", err)
	}

	resetFreeProviderCatalogCacheForTest()
	if err := InitFreeProviderCatalogStore(); err != nil {
		t.Fatalf("reload catalog store: %v", err)
	}
	got, ok := GetFreeProviderCatalogSnapshot(snapshot.DeploymentID)
	if !ok {
		t.Fatal("expected persisted catalog snapshot")
	}
	if got.SelectedModel != snapshot.SelectedModel || len(got.Models) != 1 {
		t.Fatalf("last successful catalog was replaced: %#v", got)
	}
	if got.Models[0].SupportsTools == nil || !*got.Models[0].SupportsTools {
		t.Fatalf("model capabilities were not preserved: %#v", got.Models[0])
	}
	if !got.LastSuccessAt.Equal(successAt) {
		t.Fatalf("last success = %s, want %s", got.LastSuccessAt, successAt)
	}
	if !got.LastAttemptAt.Equal(failureAt) || got.LastError != "upstream timeout" {
		t.Fatalf("failure diagnostics not updated: %#v", got)
	}
}

func TestCatalogStoreReturnsDeepCopies(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	tools := true
	snapshot := FreeProviderCatalogSnapshot{
		DeploymentID:  "free:kilo-copy",
		Provider:      "kilo",
		Source:        ModelFetchKiloFree,
		Models:        []FreeModelCatalogEntry{{ID: "model-a", SupportsTools: &tools}},
		SelectedModel: "model-a",
		LastAttemptAt: now,
		LastSuccessAt: now,
	}
	if err := saveFreeProviderCatalogSuccess(snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	first, _ := GetFreeProviderCatalogSnapshot(snapshot.DeploymentID)
	first.Models[0].ID = "mutated"
	*first.Models[0].SupportsTools = false
	second, _ := GetFreeProviderCatalogSnapshot(snapshot.DeploymentID)
	if second.Models[0].ID != "model-a" || second.Models[0].SupportsTools == nil || !*second.Models[0].SupportsTools {
		t.Fatalf("cached snapshot was mutated through caller: %#v", second)
	}
}

func TestBuildFreeProviderCatalogProjectsStaleDynamicSnapshot(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-13 * time.Hour)
	tools := true
	depID := deploymentID("kilo", SafeKeyHash(""))
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID:  depID,
		Provider:      "kilo",
		Source:        ModelFetchKiloFree,
		Models:        []FreeModelCatalogEntry{{ID: "kilo/tools:free", SupportsTools: &tools}},
		SelectedModel: "kilo/tools:free",
		LastAttemptAt: lastSuccess,
		LastSuccessAt: lastSuccess,
	}); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	catalog := buildFreeProviderCatalogAt(&Config{
		FreeProviders: map[string]FreeProviderConfig{
			"kilo": {Enabled: true},
		},
	}, now)
	var kilo *FreeProviderCatalogEntry
	for i := range catalog {
		if catalog[i].Name == "kilo" {
			kilo = &catalog[i]
			break
		}
	}
	if kilo == nil {
		t.Fatal("kilo catalog entry missing")
	}
	if !kilo.CatalogStatus.Refreshable || !kilo.CatalogStatus.Stale {
		t.Fatalf("expected stale refreshable catalog status: %#v", kilo.CatalogStatus)
	}
	if kilo.CatalogStatus.ModelCount != 1 || kilo.CatalogStatus.LastSuccessAt == nil {
		t.Fatalf("catalog status missing snapshot data: %#v", kilo.CatalogStatus)
	}
	if len(kilo.ModelCapabilities) != 1 || kilo.ModelCapabilities[0].SupportsTools == nil || !*kilo.ModelCapabilities[0].SupportsTools {
		t.Fatalf("model capabilities not projected: %#v", kilo.ModelCapabilities)
	}
}

func TestBuildFreeProviderCatalogProjectsLatestRefreshFailureWithoutKeys(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	key := "secret-provider-key"
	depID := deploymentID("nvidia", SafeKeyHash(key))
	if err := markFreeProviderCatalogFailure(
		depID, "nvidia", ModelFetchOpenAIModels, now, errors.New("upstream status 503"),
	); err != nil {
		t.Fatalf("mark catalog failure: %v", err)
	}

	catalog := buildFreeProviderCatalogAt(&Config{
		FreeProviders: map[string]FreeProviderConfig{
			"nvidia": {Enabled: true, Keys: []string{key}},
		},
	}, now)
	var nvidia *FreeProviderCatalogEntry
	for i := range catalog {
		if catalog[i].Name == "nvidia" {
			nvidia = &catalog[i]
			break
		}
	}
	if nvidia == nil {
		t.Fatal("nvidia catalog entry missing")
	}
	if !nvidia.CatalogStatus.Stale || nvidia.CatalogStatus.LastError != "upstream status 503" {
		t.Fatalf("failure status missing: %#v", nvidia.CatalogStatus)
	}
	if strings.Contains(nvidia.CatalogStatus.LastError, key) {
		t.Fatal("catalog status leaked provider key")
	}
}

func TestBuildFreeProviderCatalogDistinguishesNotRefreshedFromStale(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	catalog := buildFreeProviderCatalogAt(&Config{
		FreeProviders: map[string]FreeProviderConfig{
			"kilo": {Enabled: true},
		},
	}, time.Now().UTC())
	var kilo *FreeProviderCatalogEntry
	for i := range catalog {
		if catalog[i].Name == "kilo" {
			kilo = &catalog[i]
			break
		}
	}
	if kilo == nil {
		t.Fatal("kilo catalog entry missing")
	}
	if kilo.CatalogStatus.State != "not_refreshed" || kilo.CatalogStatus.Stale {
		t.Fatalf("never-refreshed catalog must not be stale: %#v", kilo.CatalogStatus)
	}
	encoded, err := json.Marshal(kilo)
	if err != nil {
		t.Fatalf("marshal catalog entry: %v", err)
	}
	if !strings.Contains(string(encoded), `"model_capabilities":[]`) {
		t.Fatalf("model_capabilities must be a stable empty array: %s", encoded)
	}
}

func TestCatalogStoreSkipsCorruptRowAndRestoresValidSnapshot(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	validModels, err := json.Marshal([]FreeModelCatalogEntry{{ID: "valid-model"}})
	if err != nil {
		t.Fatalf("marshal valid models: %v", err)
	}
	records := []freeProviderCatalogRecord{
		{
			DeploymentID:  "free:kilo-corrupt",
			Provider:      "kilo",
			Source:        ModelFetchKiloFree,
			ModelsJSON:    "{not-json",
			SelectedModel: "bad",
			LastAttemptAt: &now,
			LastSuccessAt: &now,
		},
		{
			DeploymentID:  "free:kilo-valid",
			Provider:      "kilo",
			Source:        ModelFetchKiloFree,
			ModelsJSON:    string(validModels),
			SelectedModel: "valid-model",
			LastAttemptAt: &now,
			LastSuccessAt: &now,
		},
	}
	if err := dbmodel.DB.Create(&records).Error; err != nil {
		t.Fatalf("insert catalog records: %v", err)
	}
	resetFreeProviderCatalogCacheForTest()
	if err := InitFreeProviderCatalogStore(); err != nil {
		t.Fatalf("one corrupt row must not fail catalog initialization: %v", err)
	}
	if _, ok := GetFreeProviderCatalogSnapshot("free:kilo-corrupt"); ok {
		t.Fatal("corrupt snapshot should be quarantined from cache")
	}
	valid, ok := GetFreeProviderCatalogSnapshot("free:kilo-valid")
	if !ok || len(valid.Models) != 1 || valid.Models[0].ID != "valid-model" {
		t.Fatalf("valid snapshot was not restored: %#v", valid)
	}
}
