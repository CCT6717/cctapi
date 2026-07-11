package fallback

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type countingLedgerMigrator struct {
	gorm.Migrator
	autoMigrateCalls *int
}

func (m countingLedgerMigrator) AutoMigrate(values ...interface{}) error {
	*m.autoMigrateCalls++
	return m.Migrator.AutoMigrate(values...)
}

type countingLedgerDialector struct {
	gorm.Dialector
	autoMigrateCalls int
}

func (d *countingLedgerDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return countingLedgerMigrator{
		Migrator:         d.Dialector.Migrator(db),
		autoMigrateCalls: &d.autoMigrateCalls,
	}
}

func setupFreeProviderLedgerTestDB(t *testing.T) func() {
	t.Helper()
	originalDB := dbmodel.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	dbmodel.DB = db
	return func() {
		dbmodel.DB = originalDB
	}
}

func TestInitFreeProviderLedgerStoreMigratesEachDatabaseOnlyOnce(t *testing.T) {
	originalDB := dbmodel.DB
	defer func() {
		dbmodel.DB = originalDB
	}()

	dialector := &countingLedgerDialector{Dialector: sqlite.Open(":memory:")}
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	dbmodel.DB = db

	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("first InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("second InitFreeProviderLedgerStore failed: %v", err)
	}
	if dialector.autoMigrateCalls != 1 {
		t.Fatalf("expected one migration for one database, got %d", dialector.autoMigrateCalls)
	}
}

func TestRecordFreeProviderUsageAggregatesByProviderKeyModelAndPeriod(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}

	usageA := UsageInfo{PromptTokens: 100, CompletionTokens: 25, TotalTokens: 125}
	usageB := UsageInfo{PromptTokens: 40, CompletionTokens: 10, TotalTokens: 50}
	if err := RecordFreeProviderUsage("free:groq-001122ff", "llama-3.1-free", usageA); err != nil {
		t.Fatalf("first RecordFreeProviderUsage failed: %v", err)
	}
	if err := RecordFreeProviderUsage("free:groq-001122ff", "llama-3.1-free", usageB); err != nil {
		t.Fatalf("second RecordFreeProviderUsage failed: %v", err)
	}

	row, err := GetFreeProviderUsage("groq", "001122ff", "llama-3.1-free", todayString())
	if err != nil {
		t.Fatalf("GetFreeProviderUsage failed: %v", err)
	}
	if row.Provider != "groq" || row.KeyHash != "001122ff" || row.ModelName != "llama-3.1-free" || row.Period != todayString() {
		t.Fatalf("unexpected usage identity: %+v", row)
	}
	if row.RequestCount != 2 || row.SuccessCount != 2 {
		t.Fatalf("expected two requests and successes, got requests=%d successes=%d", row.RequestCount, row.SuccessCount)
	}
	if row.PromptTokens != 140 || row.CompletionTokens != 35 || row.TotalTokens != 175 {
		t.Fatalf("unexpected token totals: prompt=%d completion=%d total=%d", row.PromptTokens, row.CompletionTokens, row.TotalTokens)
	}

	var count int64
	if err := dbmodel.DB.Model(&FreeProviderUsageLedger{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count ledger rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one aggregate row, got %d", count)
	}
}

func TestFreeProviderUsageLedgerIndexedStringFieldsHaveSizes(t *testing.T) {
	typ := reflect.TypeOf(FreeProviderUsageLedger{})
	for _, fieldName := range []string{"Provider", "KeyHash", "ModelName", "Period"} {
		field, ok := typ.FieldByName(fieldName)
		if !ok {
			t.Fatalf("missing field %s", fieldName)
		}
		tag := string(field.Tag.Get("gorm"))
		if !strings.Contains(tag, "size:") {
			t.Fatalf("indexed field %s must declare a size to keep MySQL composite indexes safe, tag=%q", fieldName, tag)
		}
	}
}

func TestRecordFreeProviderUsageIgnoresManualFreeDeploymentIDs(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := RecordFreeProviderUsage("free:custom-manual", "manual-model", UsageInfo{TotalTokens: 100}); err != nil {
		t.Fatalf("manual free deployment should be ignored without error: %v", err)
	}

	var count int64
	if err := dbmodel.DB.Model(&FreeProviderUsageLedger{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count ledger rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no rows for manual free deployment, got %d", count)
	}

	_, err := GetFreeProviderUsage("custom", "manual", "manual-model", todayString())
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for ignored manual deployment, got %v", err)
	}
}

func TestListFreeProviderUsageFiltersByProviderAndPeriod(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := RecordFreeProviderUsage("free:groq-001122ff", "llama-free", UsageInfo{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}); err != nil {
		t.Fatalf("record groq: %v", err)
	}
	if err := RecordFreeProviderUsage("free:nvidia-deadbeef", "qwen-free", UsageInfo{PromptTokens: 4, CompletionTokens: 5, TotalTokens: 9}); err != nil {
		t.Fatalf("record nvidia: %v", err)
	}

	rows, err := ListFreeProviderUsage(FreeProviderUsageFilter{Provider: "groq", Period: todayString()})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Provider != "groq" || rows[0].KeyHash != "001122ff" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestListFreeProviderUsageFiltersByKeyHashModelAndCombinedFields(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	seed := []struct {
		deploymentID string
		modelName    string
	}{
		{deploymentID: "free:groq-001122ff", modelName: "llama-free"},
		{deploymentID: "free:groq-aabbccdd", modelName: "llama-free"},
		{deploymentID: "free:groq-001122ff", modelName: "mixtral-free"},
		{deploymentID: "free:nvidia-deadbeef", modelName: "llama-free"},
	}
	for _, row := range seed {
		if err := RecordFreeProviderUsage(row.deploymentID, row.modelName, UsageInfo{TotalTokens: 1}); err != nil {
			t.Fatalf("record %s/%s: %v", row.deploymentID, row.modelName, err)
		}
	}

	keyRows, err := ListFreeProviderUsage(FreeProviderUsageFilter{KeyHash: "001122ff"})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage by key_hash failed: %v", err)
	}
	if len(keyRows) != 2 {
		t.Fatalf("expected two rows for key_hash filter, got %+v", keyRows)
	}
	for _, row := range keyRows {
		if row.KeyHash != "001122ff" {
			t.Fatalf("key_hash filter returned wrong row: %+v", row)
		}
	}

	modelRows, err := ListFreeProviderUsage(FreeProviderUsageFilter{ModelName: "llama-free"})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage by model failed: %v", err)
	}
	if len(modelRows) != 3 {
		t.Fatalf("expected three rows for model filter, got %+v", modelRows)
	}
	for _, row := range modelRows {
		if row.ModelName != "llama-free" {
			t.Fatalf("model filter returned wrong row: %+v", row)
		}
	}

	combinedRows, err := ListFreeProviderUsage(FreeProviderUsageFilter{
		Provider:  "groq",
		KeyHash:   "001122ff",
		ModelName: "llama-free",
		Period:    todayString(),
	})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage by combined filters failed: %v", err)
	}
	if len(combinedRows) != 1 {
		t.Fatalf("expected one combined-filter row, got %+v", combinedRows)
	}
	got := combinedRows[0]
	if got.Provider != "groq" || got.KeyHash != "001122ff" || got.ModelName != "llama-free" || got.Period != todayString() {
		t.Fatalf("combined filter returned wrong row: %+v", got)
	}
}

func TestListFreeProviderUsageDefaultsToToday(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()
	if err := InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := RecordFreeProviderUsage("free:groq-001122ff", "llama-free", UsageInfo{TotalTokens: 3}); err != nil {
		t.Fatalf("record: %v", err)
	}

	rows, err := ListFreeProviderUsage(FreeProviderUsageFilter{})
	if err != nil {
		t.Fatalf("ListFreeProviderUsage failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Period != todayString() {
		t.Fatalf("expected one current-period row, got %+v", rows)
	}
}

func TestRecordFallbackDeploymentSuccessUpdatesDeploymentStateAndFreeProviderLedger(t *testing.T) {
	cleanupDB := setupFreeProviderLedgerTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	usage := UsageInfo{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20}
	if err := RecordFallbackDeploymentSuccess("free:groq-001122ff", "llama-3.1-free", usage); err != nil {
		t.Fatalf("RecordFallbackDeploymentSuccess failed: %v", err)
	}

	stats, requestCount, errorCount, err := GetDeploymentStats("free:groq-001122ff")
	if err != nil {
		t.Fatalf("GetDeploymentStats failed: %v", err)
	}
	if requestCount != 1 || errorCount != 0 {
		t.Fatalf("unexpected deployment counters: requests=%d errors=%d", requestCount, errorCount)
	}
	if stats.PromptTokens != 12 || stats.CompletionTokens != 8 || stats.TotalTokens != 20 {
		t.Fatalf("unexpected deployment stats: %+v", stats)
	}

	row, err := GetFreeProviderUsage("groq", "001122ff", "llama-3.1-free", todayString())
	if err != nil {
		t.Fatalf("GetFreeProviderUsage failed: %v", err)
	}
	if row.RequestCount != 1 || row.TotalTokens != 20 {
		t.Fatalf("unexpected ledger row: %+v", row)
	}
}
