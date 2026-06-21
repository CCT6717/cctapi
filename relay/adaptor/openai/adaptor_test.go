package openai

import (
	"testing"

	"github.com/songquanpeng/one-api/relay/channeltype"
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
