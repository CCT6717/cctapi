package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestShouldCheckModelIncludesResponsesRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	if !shouldCheckModel(c) {
		t.Fatal("expected /v1/responses to require request model checks")
	}
}

func TestTokenAuthInternalErrorIsSanitized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalRedis := common.RedisEnabled
	originalMemoryCache := config.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB = originalDB
		common.RedisEnabled = originalRedis
		config.MemoryCacheEnabled = originalMemoryCache
	})
	common.RedisEnabled = false
	config.MemoryCacheEnabled = false

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := database.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("migrate token table: %v", err)
	}
	model.DB = database
	if err := database.Create(&model.Token{
		UserId:         1,
		Key:            "internal",
		Status:         model.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	engine := gin.New()
	engine.Use(TokenAuth())
	engine.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-internal")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), "no such table") {
		t.Fatalf("response leaked database error: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "用户状态检查失败") {
		t.Fatalf("response does not contain generic error: %s", recorder.Body.String())
	}
}
