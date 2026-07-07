package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/relay/channeltype"
	"github.com/songquanpeng/one-api/relay/meta"
	"github.com/songquanpeng/one-api/relay/model"
)

func TestOpenAICompatibleConvertRequestStripsCacheControl(t *testing.T) {
	adaptor := &Adaptor{ChannelType: channeltype.OpenAICompatible}
	request := &model.GeneralOpenAIRequest{
		Messages: []model.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type":          model.ContentTypeText,
						"text":          "hello",
						"cache_control": map[string]any{"type": "ephemeral"},
					},
				},
			},
		},
	}

	converted, err := adaptor.ConvertRequest(nil, 0, request)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	convertedRequest := converted.(*model.GeneralOpenAIRequest)
	parts := convertedRequest.Messages[0].Content.([]any)
	part := parts[0].(map[string]any)
	if _, ok := part["cache_control"]; ok {
		t.Fatal("cache_control should be stripped for OpenAI-compatible channels")
	}
	if part["type"] != model.ContentTypeText || part["text"] != "hello" {
		t.Fatalf("standard text content was not preserved: %#v", part)
	}
}

func TestOpenAIConvertRequestKeepsCacheControl(t *testing.T) {
	adaptor := &Adaptor{ChannelType: channeltype.OpenAI}
	request := &model.GeneralOpenAIRequest{
		Messages: []model.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type":          model.ContentTypeText,
						"text":          "hello",
						"cache_control": map[string]any{"type": "ephemeral"},
					},
				},
			},
		},
	}

	converted, err := adaptor.ConvertRequest(nil, 0, request)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	convertedRequest := converted.(*model.GeneralOpenAIRequest)
	parts := convertedRequest.Messages[0].Content.([]any)
	part := parts[0].(map[string]any)
	if _, ok := part["cache_control"]; !ok {
		t.Fatal("cache_control should be preserved for native OpenAI channels")
	}
}

func newFreeProviderAdaptorTestContext(provider string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.ChannelName, "[CCT Auto] "+provider+"-001122ff")
	return c
}

func TestOpenAICompatibleConvertRequestAppliesNvidiaParallelToolQuirk(t *testing.T) {
	adaptor := &Adaptor{ChannelType: channeltype.OpenAICompatible}
	c := newFreeProviderAdaptorTestContext("nvidia")
	parallel := true
	request := &model.GeneralOpenAIRequest{
		Model:            "nvidia/test-free",
		ParallelTooCalls: &parallel,
	}

	converted, err := adaptor.ConvertRequest(c, 0, request)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	convertedRequest := converted.(*model.GeneralOpenAIRequest)
	if convertedRequest.ParallelTooCalls == nil {
		t.Fatal("expected parallel_tool_calls to be set")
	}
	if *convertedRequest.ParallelTooCalls {
		t.Fatal("expected nvidia quirk to force parallel_tool_calls=false")
	}
}

func TestOpenAICompatibleSetupRequestHeaderAppliesRoutewayUserAgent(t *testing.T) {
	adaptor := &Adaptor{ChannelType: channeltype.OpenAICompatible}
	c := newFreeProviderAdaptorTestContext("routeway")
	req := httptest.NewRequest(http.MethodPost, "https://api.routeway.ai/v1/chat/completions", nil)

	err := adaptor.SetupRequestHeader(c, req, &meta.Meta{
		APIKey:      "test-key",
		ChannelType: channeltype.OpenAICompatible,
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader failed: %v", err)
	}

	if got := req.Header.Get("User-Agent"); got != "cctapi-free-pool/1.0" {
		t.Fatalf("expected routeway user-agent quirk, got %q", got)
	}
}

func TestApplyFreeProviderRequestQuirksDisablesAIHordeUnsupportedFields(t *testing.T) {
	c := newFreeProviderAdaptorTestContext("aihorde")
	maxCompletion := 4096
	request := &model.GeneralOpenAIRequest{
		Model:               "aihorde/test-free",
		MaxTokens:           4096,
		MaxCompletionTokens: &maxCompletion,
		Stop:                []string{"stop-here"},
		Stream:              true,
	}

	ApplyFreeProviderRequestQuirks(c, request)

	if request.Stream {
		t.Fatal("expected aihorde quirk to disable stream")
	}
	if request.MaxTokens != 1024 {
		t.Fatalf("expected max_tokens capped to 1024, got %d", request.MaxTokens)
	}
	if request.MaxCompletionTokens == nil || *request.MaxCompletionTokens != 1024 {
		t.Fatalf("expected max_completion_tokens capped to 1024, got %v", request.MaxCompletionTokens)
	}
	if request.Stop != nil {
		t.Fatalf("expected aihorde quirk to drop stop, got %#v", request.Stop)
	}
}

func TestApplyFreeProviderRequestQuirksUsesExplicitProviderContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ctxkey.ChannelName, "manual-channel-name")
	c.Set(ctxkey.FallbackFreeProviderName, "aihorde")
	maxTokens := 4096
	req := &model.GeneralOpenAIRequest{
		Stream:              true,
		MaxTokens:           4096,
		MaxCompletionTokens: &maxTokens,
		Stop:                []string{"stop"},
	}

	ApplyFreeProviderRequestQuirks(c, req)

	if req.Stream {
		t.Fatalf("stream should be disabled for aihorde")
	}
	if req.MaxTokens != 1024 {
		t.Fatalf("MaxTokens = %d, want 1024", req.MaxTokens)
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 1024 {
		t.Fatalf("MaxCompletionTokens = %v, want 1024", req.MaxCompletionTokens)
	}
	if req.Stop != nil {
		t.Fatalf("Stop = %#v, want nil", req.Stop)
	}
}
