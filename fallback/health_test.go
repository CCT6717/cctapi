package fallback

import (
	"io"
	"strings"
	"testing"

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
