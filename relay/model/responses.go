package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ResponsesUnsupportedInputError struct {
	Message string
}

func (e *ResponsesUnsupportedInputError) Error() string {
	return e.Message
}

func UnsupportedResponsesInputError(message string) error {
	return &ResponsesUnsupportedInputError{Message: message}
}

type ResponsesRequest struct {
	Model             string   `json:"model"`
	Input             any      `json:"input"`
	Instructions      string   `json:"instructions,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	MaxOutputTokens   *int     `json:"max_output_tokens,omitempty"`
	Tools             []Tool   `json:"tools,omitempty"`
	ToolChoice        any      `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool    `json:"parallel_tool_calls,omitempty"`
	Stream            bool     `json:"stream,omitempty"`
	Store             *bool    `json:"store,omitempty"`
	Metadata          any      `json:"metadata,omitempty"`
	User              string   `json:"user,omitempty"`
}

func (r ResponsesRequest) ToChatRequest() (*GeneralOpenAIRequest, error) {
	messages, err := responsesInputToMessages(r.Input)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("field input is required")
	}
	if strings.TrimSpace(r.Instructions) != "" {
		messages = append([]Message{{
			Role:    "system",
			Content: strings.TrimSpace(r.Instructions),
		}}, messages...)
	}
	req := &GeneralOpenAIRequest{
		Model:            r.Model,
		Messages:         messages,
		Temperature:      r.Temperature,
		TopP:             r.TopP,
		Tools:            append([]Tool{}, r.Tools...),
		ToolChoice:       r.ToolChoice,
		ParallelTooCalls: r.ParallelToolCalls,
		Stream:           r.Stream,
		Store:            r.Store,
		Metadata:         r.Metadata,
		User:             r.User,
	}
	if r.MaxOutputTokens != nil {
		req.MaxTokens = *r.MaxOutputTokens
	}
	return req, nil
}

func responsesInputToMessages(input any) ([]Message, error) {
	switch value := input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("field input is required")
		}
		return []Message{{Role: "user", Content: value}}, nil
	case []any:
		out := make([]Message, 0, len(value))
		for _, item := range value {
			msg, err := responseInputItemToMessage(item)
			if err != nil {
				return nil, err
			}
			out = append(out, msg)
		}
		return out, nil
	default:
		return nil, UnsupportedResponsesInputError("responses input must be a string or an array of messages")
	}
}

func responseInputItemToMessage(item any) (Message, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return Message{}, UnsupportedResponsesInputError("responses input items must be objects")
	}
	msg := Message{}
	role, _ := obj["role"].(string)
	role = strings.TrimSpace(role)
	if role == "" {
		role = "user"
	}
	msg.Role = role
	if name := responseStringPtr(obj["name"]); name != nil {
		msg.Name = name
	}
	if reasoningContent, ok := obj["reasoning_content"]; ok {
		msg.ReasoningContent = reasoningContent
	}
	if toolCallID, ok := obj["tool_call_id"].(string); ok {
		msg.ToolCallId = toolCallID
	}
	if rawToolCalls, ok := obj["tool_calls"]; ok {
		toolCalls, err := responseToolCalls(rawToolCalls)
		if err != nil {
			return Message{}, err
		}
		msg.ToolCalls = toolCalls
	}
	content, ok := obj["content"]
	if !ok {
		if text, ok := obj["text"].(string); ok {
			if strings.TrimSpace(text) == "" && len(msg.ToolCalls) == 0 {
				return Message{}, UnsupportedResponsesInputError("responses input message content is required")
			}
			msg.Content = text
			return msg, nil
		}
		if len(msg.ToolCalls) > 0 {
			return msg, nil
		}
		return Message{}, UnsupportedResponsesInputError("responses input message content is required")
	}
	converted, err := responseContentToChatContent(content)
	if err != nil {
		return Message{}, err
	}
	if text, ok := converted.(string); ok {
		if strings.TrimSpace(text) == "" && len(msg.ToolCalls) == 0 {
			return Message{}, UnsupportedResponsesInputError("responses input message content is required")
		}
		msg.Content = text
		return msg, nil
	}
	if contentList, ok := converted.([]any); ok && len(contentList) == 0 && len(msg.ToolCalls) == 0 {
		return Message{}, UnsupportedResponsesInputError("responses input message content is required")
	}
	msg.Content = converted
	return msg, nil
}

func responseContentToChatContent(content any) (any, error) {
	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		parts := make([]any, 0, len(value))
		for _, rawPart := range value {
			part, ok := rawPart.(map[string]any)
			if !ok {
				return nil, UnsupportedResponsesInputError("responses content parts must be objects")
			}
			partType, _ := part["type"].(string)
			switch partType {
			case "input_text", "output_text", ContentTypeText:
				text, _ := part["text"].(string)
				parts = append(parts, map[string]any{"type": ContentTypeText, "text": text})
			case "input_image", "image_url", "input_file", "file":
				return nil, UnsupportedResponsesInputError("responses image and file input is not supported; use /v1/chat/completions")
			default:
				return nil, UnsupportedResponsesInputError(fmt.Sprintf("responses content type %q is not supported", partType))
			}
		}
		return parts, nil
	default:
		return nil, UnsupportedResponsesInputError("responses message content must be a string or content array")
	}
}

type ResponsesObject struct {
	ID        string                `json:"id"`
	Object    string                `json:"object"`
	CreatedAt int64                 `json:"created_at"`
	Status    string                `json:"status"`
	Model     string                `json:"model,omitempty"`
	Output    []ResponsesOutputItem `json:"output"`
	Usage     *ResponsesUsage       `json:"usage,omitempty"`
	Error     *ResponsesErrorObject `json:"error,omitempty"`
}

type ResponsesOutputItem struct {
	ID        string                   `json:"id,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Type      string                   `json:"type"`
	Role      string                   `json:"role,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Content   []ResponsesOutputContent `json:"content,omitempty"`
}

type ResponsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type ResponsesErrorObject struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

func ChatCompletionToResponses(body []byte, fallbackModel string) (*ResponsesObject, int, error) {
	var errPayload struct {
		Error Error `json:"error"`
	}
	if err := json.Unmarshal(body, &errPayload); err == nil && errPayload.Error.Message != "" {
		return &ResponsesObject{
			ID:        responsesID(),
			Object:    "response",
			CreatedAt: time.Now().Unix(),
			Status:    "failed",
			Model:     fallbackModel,
			Error: &ResponsesErrorObject{
				Message: errPayload.Error.Message,
				Type:    errPayload.Error.Type,
				Code:    errPayload.Error.Code,
			},
		}, 500, nil
	}

	var chat struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		return nil, 0, err
	}
	id := chat.ID
	if id == "" {
		id = responsesID()
	}
	created := chat.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	modelName := chat.Model
	if modelName == "" {
		modelName = fallbackModel
	}
	output := make([]ResponsesOutputItem, 0, len(chat.Choices))
	for index, choice := range chat.Choices {
		text := choice.Message.StringContent()
		if text != "" {
			output = append(output, ResponsesOutputItem{
				ID:   fmt.Sprintf("msg_%d", index),
				Type: "message",
				Role: "assistant",
				Content: []ResponsesOutputContent{{
					Type: "output_text",
					Text: text,
				}},
			})
		}
		for _, toolCall := range choice.Message.ToolCalls {
			callID := toolCall.Id
			output = append(output, ResponsesOutputItem{
				ID:        callID,
				CallID:    callID,
				Type:      "function_call",
				Name:      toolCall.Function.Name,
				Arguments: responseToolCallArguments(toolCall.Function.Arguments),
			})
		}
	}
	return &ResponsesObject{
		ID:        id,
		Object:    "response",
		CreatedAt: created,
		Status:    "completed",
		Model:     modelName,
		Output:    output,
		Usage: &ResponsesUsage{
			InputTokens:  chat.Usage.PromptTokens,
			OutputTokens: chat.Usage.CompletionTokens,
			TotalTokens:  chat.Usage.TotalTokens,
		},
	}, 200, nil
}

func responsesID() string {
	return fmt.Sprintf("resp_%d", time.Now().UnixNano())
}

func responseStringPtr(value any) *string {
	s, ok := value.(string)
	if !ok {
		return nil
	}
	return &s
}

func responseToolCalls(value any) ([]Tool, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, UnsupportedResponsesInputError("responses tool_calls field is invalid")
	}
	var toolCalls []Tool
	if err := json.Unmarshal(raw, &toolCalls); err != nil {
		return nil, UnsupportedResponsesInputError("responses tool_calls field is invalid")
	}
	return toolCalls, nil
}

func responseToolCallArguments(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case nil:
		return ""
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}
