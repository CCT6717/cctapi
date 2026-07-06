package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/fallback"
	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRouterFreeProviderLedgerDB(t *testing.T) func() {
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

func callFreePoolUsageGET(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	getFreePoolUsage(c)
	return w
}

func TestGetFreePoolUsageReturnsRows(t *testing.T) {
	cleanupDB := setupRouterFreeProviderLedgerDB(t)
	defer cleanupDB()
	if err := fallback.InitFreeProviderLedgerStore(); err != nil {
		t.Fatalf("InitFreeProviderLedgerStore failed: %v", err)
	}
	if err := fallback.RecordFreeProviderUsage("free:groq-001122ff", "llama-free", fallback.UsageInfo{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}); err != nil {
		t.Fatalf("record usage: %v", err)
	}

	w := callFreePoolUsageGET(t, "/api/fallback/free-pool/usage?provider=groq")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool                               `json:"success"`
		Data    []fallback.FreeProviderUsageLedger `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data[0].Provider != "groq" || resp.Data[0].KeyHash != "001122ff" {
		t.Fatalf("unexpected row: %+v", resp.Data[0])
	}
	if searchString(w.Body.String(), "gsk_") {
		t.Fatalf("usage API must not expose raw keys: %s", w.Body.String())
	}
}

func TestGetFreePoolUsageAppliesCombinedFiltersAndOmitsRawKeySentinel(t *testing.T) {
	cleanupDB := setupRouterFreeProviderLedgerDB(t)
	defer cleanupDB()
	if err := fallback.InitFreeProviderLedgerStore(); err != nil {
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
		if err := fallback.RecordFreeProviderUsage(row.deploymentID, row.modelName, fallback.UsageInfo{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}); err != nil {
			t.Fatalf("record usage %s/%s: %v", row.deploymentID, row.modelName, err)
		}
	}

	w := callFreePoolUsageGET(t, "/api/fallback/free-pool/usage?provider=groq&key_hash=001122ff&model=llama-free")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool                               `json:"success"`
		Data    []fallback.FreeProviderUsageLedger `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	row := resp.Data[0]
	if row.Provider != "groq" || row.KeyHash != "001122ff" || row.ModelName != "llama-free" {
		t.Fatalf("unexpected combined-filter row: %+v", row)
	}

	rawKeySentinel := "raw-free-provider-secret-should-not-leak"
	if searchString(w.Body.String(), rawKeySentinel) {
		t.Fatalf("usage API must not expose raw provider keys: %s", w.Body.String())
	}
}
