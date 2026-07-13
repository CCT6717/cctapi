package fallback

import (
	"errors"
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
