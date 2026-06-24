package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/model"
)

// ConvertClaudeRequestToOpenAI converts a Claude/Anthropic format request to OpenAI format.
func ConvertClaudeRequestToOpenAI(claudeReq *Request) *model.GeneralOpenAIRequest {
	openaiReq := &model.GeneralOpenAIRequest{
		Model:       claudeReq.Model,
		MaxTokens:   claudeReq.MaxTokens,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		TopK:        claudeReq.TopK,
		Stream:      claudeReq.Stream,
	}
	if len(claudeReq.StopSequences) > 0 {
		openaiReq.Stop = claudeReq.StopSequences
	}
	// system → messages[0]
	if claudeReq.System != "" {
		openaiReq.Messages = append(openaiReq.Messages, model.Message{
			Role:    "system",
			Content: claudeReq.System,
		})
	}
	// messages → messages
	for _, msg := range claudeReq.Messages {
		openaiMsgs := convertClaudeMessages(msg)
		openaiReq.Messages = append(openaiReq.Messages, openaiMsgs...)
	}
	// tools → tools
	if len(claudeReq.Tools) > 0 {
		openaiReq.Tools = convertClaudeTools(claudeReq.Tools)
	}
	// tool_choice → tool_choice
	if claudeReq.ToolChoice != nil {
		openaiReq.ToolChoice = convertClaudeToolChoice(claudeReq.ToolChoice)
	}
	return openaiReq
}

func convertClaudeMessages(msg Message) []model.Message {
	var textParts []string
	var toolUseBlocks []Content
	var toolResultBlocks []Content
	var imageParts []model.MessageContent

	for _, c := range msg.Content {
		switch c.Type {
		case "text":
			if c.Text != "" {
				textParts = append(textParts, c.Text)
			}
		case "image":
			if c.Source != nil {
				imageParts = append(imageParts, model.MessageContent{
					Type: model.ContentTypeImageURL,
					ImageURL: &model.ImageURL{
						Url: fmt.Sprintf("data:%s;base64,%s", c.Source.MediaType, c.Source.Data),
					},
				})
			}
		case "tool_use":
			toolUseBlocks = append(toolUseBlocks, c)
		case "tool_result":
			toolResultBlocks = append(toolResultBlocks, c)
		}
	}

	var msgs []model.Message

	if msg.Role == "assistant" {
		m := model.Message{Role: "assistant"}
		// text + image → content
		var contentParts []model.MessageContent
		for _, t := range textParts {
			contentParts = append(contentParts, model.MessageContent{
				Type: model.ContentTypeText,
				Text: t,
			})
		}
		contentParts = append(contentParts, imageParts...)
		if len(contentParts) > 0 {
			if len(contentParts) == 1 && contentParts[0].Type == model.ContentTypeText {
				m.Content = contentParts[0].Text
			} else {
				m.Content = contentParts
			}
		}
		// tool_use → tool_calls
		for _, tu := range toolUseBlocks {
			args, _ := json.Marshal(tu.Input)
			m.ToolCalls = append(m.ToolCalls, model.Tool{
				Id:   tu.Id,
				Type: "function",
				Function: model.Function{
					Name:      tu.Name,
					Arguments: string(args),
				},
			})
		}
		msgs = append(msgs, m)
	} else {
		// user role
		// tool_result → separate tool messages
		for _, tr := range toolResultBlocks {
			content := tr.Content
			if content == "" {
				content = " " // OpenAI 不允许空 content
			}
			msgs = append(msgs, model.Message{
				Role:       "tool",
				ToolCallId: tr.ToolUseId,
				Content:    content,
			})
		}
		// text + image → user message
		var contentParts []model.MessageContent
		for _, t := range textParts {
			contentParts = append(contentParts, model.MessageContent{
				Type: model.ContentTypeText,
				Text: t,
			})
		}
		contentParts = append(contentParts, imageParts...)
		if len(contentParts) > 0 {
			um := model.Message{Role: "user"}
			if len(contentParts) == 1 && contentParts[0].Type == model.ContentTypeText {
				um.Content = contentParts[0].Text
			} else {
				um.Content = contentParts
			}
			msgs = append(msgs, um)
		}
	}

	return msgs
}

func convertClaudeTools(tools []Tool) []model.Tool {
	result := make([]model.Tool, 0, len(tools))
	for _, t := range tools {
		result = append(result, model.Tool{
			Type: "function",
			Function: model.Function{
				Name:        t.Name,
				Description: t.Description,
				Parameters: map[string]any{
					"type":       t.InputSchema.Type,
					"properties": t.InputSchema.Properties,
					"required":   t.InputSchema.Required,
				},
			},
		})
	}
	return result
}

func convertClaudeToolChoice(tc any) any {
	tcMap, ok := tc.(map[string]any)
	if !ok {
		return nil
	}
	switch tcMap["type"] {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "tool":
		name, _ := tcMap["name"].(string)
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": name,
			},
		}
	}
	return nil
}

// ExtractClaudeToolResultContent handles the case where tool_result content
// could be a string or a structured content array.
func ExtractClaudeToolResultContent(content any) string {
	if content == nil {
		return " "
	}
	if s, ok := content.(string); ok {
		if s == "" {
			return " "
		}
		return s
	}
	// content is []Content-like — serialize back to text
	b, err := json.Marshal(content)
	if err != nil {
		return " "
	}
	s := string(b)
	if s == "" || s == "null" || s == "[]" {
		return " "
	}
	return s
}

// UnmarshalClaudeRequest handles both string and array content formats.
// Anthropic SDK may send "content": "hello" (string) or "content": [{"type":"text","text":"hello"}] (array).
func UnmarshalClaudeRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err == nil {
		return &req, nil
	}
	// Fallback: parse as raw map, normalize string content to []Content, retry
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, err
	}
	msgsRaw, ok := rawMap["messages"]
	if !ok {
		return nil, json.Unmarshal(raw, &req) // return original error
	}
	var msgs []map[string]json.RawMessage
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return nil, err
	}
	for i, msg := range msgs {
		if contentRaw, exists := msg["content"]; exists {
			// Check if content is a string (not an array)
			var s string
			if err := json.Unmarshal(contentRaw, &s); err == nil {
				// String content → wrap in []Content
				wrapped, _ := json.Marshal([]Content{{Type: "text", Text: s}})
				msgs[i]["content"] = wrapped
			}
		}
	}
	normalized, _ := json.Marshal(msgs)
	rawMap["messages"] = normalized
	rebuilt, _ := json.Marshal(rawMap)
	if err := json.Unmarshal(rebuilt, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// IsClaudeFormat checks whether the request is Claude format.
// Primary signal: Anthropic SDK always sends anthropic-version header.
// Fallback: body-based heuristics (stop_sequences, system field, content block types).
func IsClaudeFormat(c *gin.Context, body map[string]any) bool {
	// 最可靠：Anthropic SDK 必发此 header
	if c.GetHeader("anthropic-version") != "" {
		return true
	}
	if _, ok := body["max_tokens"]; ok {
		if _, ok := body["messages"]; ok {
			if _, hasStopSeq := body["stop_sequences"]; hasStopSeq {
				return true
			}
			if _, hasSystem := body["system"]; hasSystem {
				return true
			}
			msgs, ok := body["messages"].([]any)
			if !ok || len(msgs) == 0 {
				return false
			}
			firstMsg, ok := msgs[0].(map[string]any)
			if !ok {
				return false
			}
			if content, ok := firstMsg["content"].([]any); ok && len(content) > 0 {
				if block, ok := content[0].(map[string]any); ok {
					if blockType, ok := block["type"].(string); ok {
						if blockType == "tool_use" || blockType == "tool_result" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// ConvertOpenAIResponseToClaude converts an OpenAI text response to Claude format.
func ConvertOpenAIResponseToClaude(openaiResp *openai.TextResponse) *Response {
	claudeResp := &Response{
		Id:    "msg_" + strings.TrimPrefix(openaiResp.Id, "chatcmpl-"),
		Type:  "message",
		Role:  "assistant",
		Model: openaiResp.Model,
		Usage: Usage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}
	if len(openaiResp.Choices) == 0 {
		claudeResp.Content = []Content{}
		return claudeResp
	}
	choice := openaiResp.Choices[0]
	if s, ok := choice.Content.(string); ok && s != "" {
		claudeResp.Content = append(claudeResp.Content, Content{
			Type: "text",
			Text: s,
		})
	}
	for _, tc := range choice.ToolCalls {
		var input map[string]any
		args, _ := tc.Function.Arguments.(string)
		if err := json.Unmarshal([]byte(args), &input); err != nil {
			input = map[string]any{}
		}
		claudeResp.Content = append(claudeResp.Content, Content{
			Type:  "tool_use",
			Id:    tc.Id,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	if claudeResp.Content == nil {
		claudeResp.Content = []Content{}
	}
	reason := convertOpenAIFinishReason(choice.FinishReason)
	claudeResp.StopReason = &reason
	return claudeResp
}

func convertOpenAIFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}
