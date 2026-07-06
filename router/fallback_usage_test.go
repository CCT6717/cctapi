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
