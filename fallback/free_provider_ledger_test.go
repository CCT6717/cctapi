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
