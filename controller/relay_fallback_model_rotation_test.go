package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/fallback"
	dbmodel "github.com/songquanpeng/one-api/model"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const relayRotationVirtualModel = "test/kilo-rotation"

type relayRotationFixture struct {
	db *gorm.DB
}

func TestRelayWithFallbackRotatesKiloModels(t *testing.T) {
	fixture := setupRelayRotationFixture(t, []string{
		"free:kilo-00000001",
		"free:kilo-00000002",
		"free:kilo-00000003",
		"free:kilo-00000004",
		"free:kilo-00000005",
	})

	t.Run("rate limit rotates to next Kilo model without provider penalty", func(t *testing.T) {
		kiloID := "free:kilo-00000001"
		pollinationsID := "free:pollinations-00000001"
		loadRelayRotationConfig(t, kiloID, pollinationsID)

		var attempted []string
		c, _ := newRelayRotationContext()
		relayWithFallbackUsing(c, func(c *gin.Context, _ int) *relaymodel.ErrorWithStatusCode {
			attempted = append(attempted, c.GetString(ctxkey.FallbackRealModel))
			if len(attempted) == 1 {
				return relayRotationRateLimitError()
			}
			return nil
		})

		wantAttempts := []string{"kilo/a:free", "kilo/b:free"}
		if !reflect.DeepEqual(attempted, wantAttempts) {
			t.Fatalf("attempted models = %v, want %v", attempted, wantAttempts)
		}
		providerRuntime := fallback.SnapshotRuntimeState(kiloID)
		if providerRuntime.FailureCount != 0 || providerRuntime.RateLimitScore != 0 {
			t.Fatalf("intermediate model 429 penalized provider: %+v", providerRuntime)
		}
		_, requestCount, errorCount, err := fallback.GetDeploymentStats(kiloID)
		if err != nil {
			t.Fatalf("GetDeploymentStats: %v", err)
		}
		if requestCount != 0 || errorCount != 0 {
			t.Fatalf("intermediate model 429 changed persistent provider accounting: requests=%d errors=%d", requestCount, errorCount)
		}
		providerCooldown := fallback.SnapshotDeploymentCooldown(kiloID)
		if providerCooldown.CooldownActive {
			t.Fatalf("intermediate model 429 cooled provider: %+v", providerCooldown)
		}
		modelRuntime := fallback.SnapshotFreeProviderModelRuntime(kiloID)
		if modelRuntime.LastSuccessfulModel != "kilo/b:free" || modelRuntime.ActiveCooldownCount != 1 {
			t.Fatalf("actual Kilo model success was not recorded: %+v", modelRuntime)
		}
		configured, ok := fallback.GetDeployment(kiloID)
		if !ok || configured.RealModel != "kilo/a:free" {
			t.Fatalf("request attempt mutated configured deployment: %#v, ok=%v", configured, ok)
		}
		if count := fixture.switchEventCount(t); count != 0 {
			t.Fatalf("same-deployment model rotation emitted %d switch events", count)
		}
	})

	t.Run("all Kilo models rate limited penalizes provider once then advances", func(t *testing.T) {
		kiloID := "free:kilo-00000002"
		pollinationsID := "free:pollinations-00000002"
		loadRelayRotationConfig(t, kiloID, pollinationsID)

		var attempted []string
		c, _ := newRelayRotationContext()
		relayWithFallbackUsing(c, func(c *gin.Context, _ int) *relaymodel.ErrorWithStatusCode {
			attempted = append(attempted, c.GetString(ctxkey.FallbackRealModel))
			if c.GetString(ctxkey.FallbackDeploymentID) == kiloID {
				return relayRotationRateLimitError()
			}
			return nil
		})

		wantAttempts := []string{"kilo/a:free", "kilo/b:free", "openai-fast"}
		if !reflect.DeepEqual(attempted, wantAttempts) {
			t.Fatalf("attempted models = %v, want %v", attempted, wantAttempts)
		}
		providerRuntime := fallback.SnapshotRuntimeState(kiloID)
		if providerRuntime.FailureCount != 1 || providerRuntime.RateLimitScore != 1 {
			t.Fatalf("provider accounting = %+v, want one rate-limit failure", providerRuntime)
		}
		_, requestCount, errorCount, err := fallback.GetDeploymentStats(kiloID)
		if err != nil {
			t.Fatalf("GetDeploymentStats: %v", err)
		}
		if requestCount != 1 || errorCount != 1 {
			t.Fatalf("persistent provider accounting requests=%d errors=%d, want 1/1", requestCount, errorCount)
		}
		providerCooldown := fallback.SnapshotDeploymentCooldown(kiloID)
		if !providerCooldown.CooldownActive {
			t.Fatalf("final Kilo exhaustion did not cool provider: %+v", providerCooldown)
		}
		modelRuntime := fallback.SnapshotFreeProviderModelRuntime(kiloID)
		if modelRuntime.ActiveCooldownCount != 2 {
			t.Fatalf("model cooldown count = %d, want 2: %+v", modelRuntime.ActiveCooldownCount, modelRuntime)
		}
		events := fixture.switchEvents(t)
		if len(events) != 1 || events[0].FromDeployment != kiloID || events[0].ToDeployment != pollinationsID {
			t.Fatalf("provider switch events = %+v, want one Kilo-to-Pollinations event", events)
		}
	})

	t.Run("non rate limit Kilo failure skips remaining models", func(t *testing.T) {
		kiloID := "free:kilo-00000003"
		pollinationsID := "free:pollinations-00000003"
		loadRelayRotationConfig(t, kiloID, pollinationsID)

		var attempted []string
		c, _ := newRelayRotationContext()
		relayWithFallbackUsing(c, func(c *gin.Context, _ int) *relaymodel.ErrorWithStatusCode {
			attempted = append(attempted, c.GetString(ctxkey.FallbackRealModel))
			if c.GetString(ctxkey.FallbackDeploymentID) == kiloID {
				return &relaymodel.ErrorWithStatusCode{
					StatusCode: http.StatusInternalServerError,
					Error:      relaymodel.Error{Message: "service unavailable", Type: "server_error", Code: "upstream_error"},
				}
			}
			return nil
		})

		wantAttempts := []string{"kilo/a:free", "openai-fast"}
		if !reflect.DeepEqual(attempted, wantAttempts) {
			t.Fatalf("attempted models = %v, want %v", attempted, wantAttempts)
		}
		providerRuntime := fallback.SnapshotRuntimeState(kiloID)
		if providerRuntime.FailureCount != 1 || providerRuntime.RateLimitScore != 0 {
			t.Fatalf("non-429 provider accounting = %+v, want one non-rate-limit failure", providerRuntime)
		}
		if modelRuntime := fallback.SnapshotFreeProviderModelRuntime(kiloID); len(modelRuntime.Models) != 0 {
			t.Fatalf("non-429 failure changed model cooldowns: %+v", modelRuntime)
		}
	})

	t.Run("written response is not replayed", func(t *testing.T) {
		kiloID := "free:kilo-00000004"
		pollinationsID := "free:pollinations-00000004"
		loadRelayRotationConfig(t, kiloID, pollinationsID)

		var attempted []string
		c, recorder := newRelayRotationContext()
		relayWithFallbackUsing(c, func(c *gin.Context, _ int) *relaymodel.ErrorWithStatusCode {
			attempted = append(attempted, c.GetString(ctxkey.FallbackRealModel))
			_, _ = c.Writer.Write([]byte("partial stream"))
			return relayRotationRateLimitError()
		})

		if want := []string{"kilo/a:free"}; !reflect.DeepEqual(attempted, want) {
			t.Fatalf("attempted models = %v, want %v", attempted, want)
		}
		if recorder.Body.String() != "partial stream" {
			t.Fatalf("written response = %q", recorder.Body.String())
		}
		if modelRuntime := fallback.SnapshotFreeProviderModelRuntime(kiloID); len(modelRuntime.Models) != 0 {
			t.Fatalf("written response incorrectly rotated/cooldown-marked model: %+v", modelRuntime)
		}
	})

	t.Run("non-429 rate-limit-shaped failure skips remaining Kilo models", func(t *testing.T) {
		kiloID := "free:kilo-00000005"
		pollinationsID := "free:pollinations-00000005"
		loadRelayRotationConfig(t, kiloID, pollinationsID)

		var attempted []string
		c, _ := newRelayRotationContext()
		relayWithFallbackUsing(c, func(c *gin.Context, _ int) *relaymodel.ErrorWithStatusCode {
			attempted = append(attempted, c.GetString(ctxkey.FallbackRealModel))
			if c.GetString(ctxkey.FallbackDeploymentID) == kiloID {
				return &relaymodel.ErrorWithStatusCode{
					StatusCode: http.StatusInternalServerError,
					Error: relaymodel.Error{
						Message: "upstream rate limit proxy failure", Type: "server_error", Code: "upstream_error",
					},
				}
			}
			return nil
		})

		wantAttempts := []string{"kilo/a:free", "openai-fast"}
		if !reflect.DeepEqual(attempted, wantAttempts) {
			t.Fatalf("attempted models = %v, want %v", attempted, wantAttempts)
		}
		providerRuntime := fallback.SnapshotRuntimeState(kiloID)
		if providerRuntime.FailureCount != 1 || providerRuntime.RateLimitScore != 0 {
			t.Fatalf(
				"non-429 rate-limit-shaped failure accounting = %+v, want one ordinary failure and zero rate-limit score",
				providerRuntime,
			)
		}
		if modelRuntime := fallback.SnapshotFreeProviderModelRuntime(kiloID); len(modelRuntime.Models) != 0 {
			t.Fatalf("non-429 rate-limit-shaped failure changed model cooldowns: %+v", modelRuntime)
		}
	})
}

func setupRelayRotationFixture(t *testing.T, kiloDeploymentIDs []string) relayRotationFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := dbmodel.DB
	originalConfig := fallback.GetConfig()

	dsn := "file:relay_rotation_" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory DB: %v", err)
	}
	dbmodel.DB = db
	if err := db.AutoMigrate(&dbmodel.Channel{}, &fallback.SwitchEvent{}); err != nil {
		t.Fatalf("migrate relay fixture: %v", err)
	}
	channels := []dbmodel.Channel{
		{Id: 1, Name: "kilo", Status: dbmodel.ChannelStatusEnabled, Models: "kilo/a:free,kilo/b:free", Group: "default"},
		{Id: 2, Name: "pollinations", Status: dbmodel.ChannelStatusEnabled, Models: "openai-fast", Group: "default"},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatalf("create channels: %v", err)
	}
	if err := fallback.InitFreeProviderCatalogStore(); err != nil {
		t.Fatalf("InitFreeProviderCatalogStore: %v", err)
	}
	modelsJSON := `[{"id":"kilo/a:free"},{"id":"kilo/b:free"}]`
	now := time.Now().UTC()
	for _, deploymentID := range kiloDeploymentIDs {
		if err := db.Exec(`INSERT INTO free_provider_catalog_records
			(deployment_id, provider, source, models_json, selected_model, last_attempt_at, last_success_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			deploymentID, "kilo", "kilo_free", modelsJSON, "kilo/a:free", now, now, now, now).Error; err != nil {
			t.Fatalf("seed Kilo catalog %s: %v", deploymentID, err)
		}
	}

	db = db.Session(&gorm.Session{NewDB: true})
	dbmodel.DB = db
	if err := fallback.InitStateStore(); err != nil {
		t.Fatalf("InitStateStore: %v", err)
	}

	t.Cleanup(func() {
		fallback.ClearStickyDeployment(relayRotationVirtualModel)
		restoreRelayRotationConfig(t, originalConfig)
		dbmodel.DB = originalDB
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return relayRotationFixture{db: db}
}

func loadRelayRotationConfig(t *testing.T, kiloID, pollinationsID string) {
	t.Helper()
	fallback.ClearStickyDeployment(relayRotationVirtualModel)
	fallback.ResetFreeProviderModelRuntime(kiloID)
	cfg := fallback.Config{
		Enabled: true,
		VirtualModels: map[string]fallback.VirtualModelConfig{
			relayRotationVirtualModel: {
				Enabled: true, Strategy: fallback.StrategyFreeFirst, Pools: []string{"free"},
				RoutingMode: fallback.RoutingModeFallback, PreferredDeployment: kiloID,
			},
		},
		Deployments: map[string]fallback.DeploymentConfig{
			kiloID: {
				Enabled: true, ChannelID: 1, RealModel: "kilo/a:free", Pool: "free",
				Priority: 1, SupportsStream: true, SupportsTools: true, SupportsJSON: true,
			},
			pollinationsID: {
				Enabled: true, ChannelID: 2, RealModel: "openai-fast", Pool: "free",
				Priority: 2, SupportsStream: true, SupportsTools: true, SupportsJSON: true,
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal fallback config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fallback.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fallback config: %v", err)
	}
	if err := fallback.LoadConfig(path); err != nil {
		t.Fatalf("load fallback config: %v", err)
	}
}

func restoreRelayRotationConfig(t *testing.T, cfg *fallback.Config) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fallback-restore.json")
	if cfg == nil {
		if err := fallback.LoadConfig(path); err != nil {
			t.Errorf("restore empty fallback config: %v", err)
		}
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Errorf("marshal original fallback config: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Errorf("write original fallback config: %v", err)
		return
	}
	if err := fallback.LoadConfig(path); err != nil {
		t.Errorf("restore fallback config: %v", err)
	}
}

func newRelayRotationContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"test/kilo-rotation","messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.RequestModel, relayRotationVirtualModel)
	return c, recorder
}

func relayRotationRateLimitError() *relaymodel.ErrorWithStatusCode {
	retryAfter := 30
	return &relaymodel.ErrorWithStatusCode{
		StatusCode:        http.StatusTooManyRequests,
		RetryAfterSeconds: &retryAfter,
		Error: relaymodel.Error{
			Message: "model limited", Type: "rate_limit_error", Code: "rate_limit",
		},
	}
}

func (fixture relayRotationFixture) switchEvents(t *testing.T) []fallback.SwitchEvent {
	t.Helper()
	var events []fallback.SwitchEvent
	if err := fixture.db.Where("virtual_model = ?", relayRotationVirtualModel).Order("id asc").Find(&events).Error; err != nil {
		t.Fatalf("query switch events: %v", err)
	}
	return events
}

func (fixture relayRotationFixture) switchEventCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := fixture.db.Model(&fallback.SwitchEvent{}).Where("virtual_model = ?", relayRotationVirtualModel).Count(&count).Error; err != nil {
		t.Fatalf("count switch events: %v", err)
	}
	return count
}
