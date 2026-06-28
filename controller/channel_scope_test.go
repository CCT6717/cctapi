package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupChannelScopeTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("failed to migrate channel table: %v", err)
	}
	model.DB = db
	config.ItemsPerPage = 10

	channels := []model.Channel{
		{Id: 1, Name: "[CCT Auto] groq-free", Models: "llama-free", Status: model.ChannelStatusEnabled},
		{Id: 2, Name: "manual-high", Models: "gpt-4", Status: model.ChannelStatusEnabled},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatalf("failed to seed channels: %v", err)
	}
}

func setupChannelPaginationScopeTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("failed to migrate channel table: %v", err)
	}
	model.DB = db
	config.ItemsPerPage = 10

	channels := []model.Channel{
		{Id: 1, Name: "manual-old-1", Models: "gpt-4", Status: model.ChannelStatusEnabled},
		{Id: 2, Name: "manual-old-2", Models: "deepseek", Status: model.ChannelStatusEnabled},
	}
	for id := 3; id <= 12; id++ {
		channels = append(channels, model.Channel{
			Id:     id,
			Name:   "[CCT Auto] free-" + strconv.Itoa(id),
			Models: "free-model",
			Status: model.ChannelStatusEnabled,
		})
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatalf("failed to seed channels: %v", err)
	}
}

func getChannelNames(t *testing.T, target string) []string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	GetAllChannels(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body struct {
		Success bool             `json:"success"`
		Data    []*model.Channel `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response")
	}

	names := make([]string, 0, len(body.Data))
	for _, ch := range body.Data {
		names = append(names, ch.Name)
	}
	return names
}

func TestGetAllChannelsScopeFiltersManualAutoAndAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupChannelScopeTestDB(t)

	tests := []struct {
		name string
		url  string
		want []string
	}{
		{name: "manual", url: "/api/channel/?scope=manual", want: []string{"manual-high"}},
		{name: "auto", url: "/api/channel/?scope=auto", want: []string{"[CCT Auto] groq-free"}},
		{name: "all", url: "/api/channel/?scope=all", want: []string{"manual-high", "[CCT Auto] groq-free"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getChannelNames(t, tt.url)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("expected %v, got %v", tt.want, got)
				}
			}
		})
	}
}

func TestGetAllChannelsManualScopeFiltersBeforePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupChannelPaginationScopeTestDB(t)

	got := getChannelNames(t, "/api/channel/?p=0&scope=manual")
	want := []string{"manual-old-2", "manual-old-1"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
