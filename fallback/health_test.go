package fallback

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

func strPtr(s string) *string { return &s }

func TestBuildHealthProbeRequestKeylessOmitsAuthorization(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:      1,
		Type:    channeltype.OpenAICompatible,
		Key:     "",
		BaseURL: strPtr("https://text.pollinations.ai/openai"),
	}
	req, err := buildHealthProbeRequest("free:pollinations-"+SafeKeyHash(""), channel, DeploymentConfig{RealModel: "openai-fast"})
	if err != nil {
		t.Fatalf("buildHealthProbeRequest returned error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no Authorization header for keyless provider, got %q", got)
	}
	if req.URL.String() != "https://text.pollinations.ai/openai/v1/chat/completions" {
		t.Fatalf("unexpected URL: %s", req.URL.String())
	}
}

func TestBuildHealthProbeRequestAppliesUserAgentQuirk(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:      2,
		Type:    channeltype.OpenAICompatible,
		Key:     "routeway-key",
		BaseURL: strPtr("https://api.routeway.ai/v1"),
	}
	req, err := buildHealthProbeRequest("free:routeway-001122ff", channel, DeploymentConfig{RealModel: "auto"})
	if err != nil {
		t.Fatalf("buildHealthProbeRequest returned error: %v", err)
	}
	if got := req.Header.Get("User-Agent"); got != "cctapi-free-pool/1.0" {
		t.Fatalf("expected routeway user-agent quirk, got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer routeway-key" {
		t.Fatalf("expected auth header, got %q", got)
	}
}

func TestBuildHealthProbeRequestAppliesMaxOutputTokenQuirk(t *testing.T) {
	channel := &dbmodel.Channel{
		Id:      3,
		Type:    channeltype.OpenAICompatible,
		Key:     "",
		BaseURL: strPtr("https://oai.aihorde.net/v1"),
	}
	req, err := buildHealthProbeRequest("free:aihorde-001122ff", channel, DeploymentConfig{RealModel: "horde"})
	if err != nil {
		t.Fatalf("buildHealthProbeRequest returned error: %v", err)
	}
	body, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(body), `"max_tokens":1`) {
		t.Fatalf("expected max_tokens=1 body, got %s", string(body))
	}
	if strings.Contains(string(body), `"stream":true`) {
		t.Fatalf("health body must not stream, got %s", string(body))
	}
}

func TestBuildHealthProbeBodyAppliesMaxOutputTokenQuirk(t *testing.T) {
	body := buildHealthProbeBody(
		DeploymentConfig{RealModel: "synthetic-model"},
		&FreeProviderQuirks{MaxOutputTokens: 7},
		13,
	)
	if !strings.Contains(body, `"max_tokens":7`) {
		t.Fatalf("expected max_tokens capped by quirk, got %s", body)
	}
	if !strings.Contains(body, `"model":"synthetic-model"`) {
		t.Fatalf("expected model in body, got %s", body)
	}
}

func TestCheckOneDeploymentRecordsRuntimeErrorWhenChannelMissing(t *testing.T) {
	cleanupDB := setupFreePoolTestDB(t)
	defer cleanupDB()

	deploymentID := "free:pollinations-" + SafeKeyHash("") + "-missing-channel"

	checkOneDeployment(deploymentID, DeploymentConfig{
		ID:        deploymentID,
		ChannelID: 404,
		RealModel: "openai-fast",
	}, time.Millisecond)

	snap := SnapshotRuntimeState(deploymentID)
	if snap.LastError == "" {
		t.Fatal("expected health check channel failure to be visible in runtime last_error")
	}
	if !strings.Contains(snap.LastError, "health check channel") {
		t.Fatalf("expected health check channel error, got %q", snap.LastError)
	}
	if snap.FailureCount != 1 {
		t.Fatalf("expected one recorded runtime failure, got %d", snap.FailureCount)
	}
}

func TestCheckOneDeploymentClearsRuntimeErrorWhenHealthy(t *testing.T) {
	cleanupDB := setupFreePoolTestDB(t)
	defer cleanupDB()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL := server.URL
	channel := dbmodel.Channel{
		Name:    "health-check-test",
		Type:    channeltype.OpenAICompatible,
		BaseURL: &baseURL,
		Status:  dbmodel.ChannelStatusEnabled,
	}
	if err := dbmodel.DB.Create(&channel).Error; err != nil {
		t.Fatalf("failed to create test channel: %v", err)
	}

	deploymentID := "free:pollinations-" + SafeKeyHash("") + "-healthy"
	RecordFailure(deploymentID, "old health check failure", true)

	checkOneDeployment(deploymentID, DeploymentConfig{
		ID:        deploymentID,
		ChannelID: channel.Id,
		RealModel: "openai-fast",
	}, time.Second)

	if got := GetHealthStatus(deploymentID); got != HealthHealthy {
		t.Fatalf("expected healthy status, got %s", got)
	}
	snap := SnapshotRuntimeState(deploymentID)
	if snap.LastError != "" {
		t.Fatalf("expected healthy health check to clear runtime last_error, got %q", snap.LastError)
	}
}
