package openai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/relay/model"
)

// Claude response types (local to avoid circular import with anthropic package)

type claudeResponse struct {
	Id         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []claudeBlock  `json:"content"`
	Model      string         `json:"model"`
	StopReason *string        `json:"stop_reason"`
	Usage      claudeUsage    `json:"usage"`
}

type claudeBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Id    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func openaiFinishReasonToClaude(reason string) string {
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

// claudeFormatNonStream writes the upstream OpenAI response as a Claude response.
func claudeFormatNonStream(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	body, err := readBody(resp)
	if err != nil {
		return ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}

	var openaiResp TextResponse
	if err = json.Unmarshal(body, &openaiResp); err != nil {
		return ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
	}

	claudeResp := claudeResponse{
		Id:    "msg_" + strings.TrimPrefix(openaiResp.Id, "chatcmpl-"),
		Type:  "message",
		Role:  "assistant",
		Model: openaiResp.Model,
		Usage: claudeUsage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		if s, ok := choice.Content.(string); ok && s != "" {
			claudeResp.Content = append(claudeResp.Content, claudeBlock{Type: "text", Text: s})
		}
		for _, tc := range choice.ToolCalls {
			var input map[string]any
			if args, ok := tc.Function.Arguments.(string); ok {
				json.Unmarshal([]byte(args), &input)
			}
			if input == nil {
				input = map[string]any{}
			}
			claudeResp.Content = append(claudeResp.Content, claudeBlock{
				Type: "tool_use", Id: tc.Id, Name: tc.Function.Name, Input: input,
			})
		}
		if claudeResp.Content == nil {
			claudeResp.Content = []claudeBlock{}
		}
		reason := openaiFinishReasonToClaude(choice.FinishReason)
		claudeResp.StopReason = &reason
	} else {
		claudeResp.Content = []claudeBlock{}
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	json.NewEncoder(c.Writer).Encode(claudeResp)

	return nil, &model.Usage{
		PromptTokens:     claudeResp.Usage.InputTokens,
		CompletionTokens: claudeResp.Usage.OutputTokens,
		TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
	}
}

// claudeFormatStream reads OpenAI SSE and emits Claude SSE events.
func claudeFormatStream(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	defer resp.Body.Close()
	common.SetEventStreamHeaders(c)

	scanner := bufio.NewScanner(resp.Body)
	var msgId string
	var inputTokens int
	var blockOpen bool
	var blockType string
	var blockIndex int

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Id      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Role      string `json:"role,omitempty"`
					Content   any    `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						Id       string `json:"id,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason,omitempty"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage,omitempty"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		if chunk.Usage != nil {
			inputTokens += chunk.Usage.PromptTokens
		}

		// message_start (once)
		if msgId == "" {
			msgId = "msg_" + strings.TrimPrefix(chunk.Id, "chatcmpl-")
			writeClaudeSSE(c, "message_start", map[string]any{
				"message": map[string]any{
					"id": msgId, "type": "message", "role": "assistant",
					"content": []any{}, "model": chunk.Model,
					"stop_reason": nil, "stop_sequence": nil,
					"usage": map[string]any{"input_tokens": inputTokens, "output_tokens": 0},
				},
			})
			writeClaudeSSE(c, "ping", map[string]any{"type": "ping"})
			writeClaudeSSE(c, "content_block_start", map[string]any{
				"type": "content_block_start", "index": blockIndex,
				"content_block": map[string]any{"type": "text", "text": ""},
			})
			blockOpen = true
			blockType = "text"
		}

		// Text delta
		if text, ok := choice.Delta.Content.(string); ok && text != "" {
			if !blockOpen || blockType != "text" {
				closeClaudeBlock(c, &blockOpen, &blockIndex)
				writeClaudeSSE(c, "content_block_start", map[string]any{
					"type": "content_block_start", "index": blockIndex,
					"content_block": map[string]any{"type": "text", "text": ""},
				})
				blockOpen = true
				blockType = "text"
			}
			writeClaudeSSE(c, "content_block_delta", map[string]any{
				"type": "content_block_delta", "index": blockIndex,
				"delta": map[string]any{"type": "text_delta", "text": text},
			})
		}

		// Tool calls
		for _, tc := range choice.Delta.ToolCalls {
			if tc.Id != "" {
				closeClaudeBlock(c, &blockOpen, &blockIndex)
				writeClaudeSSE(c, "content_block_start", map[string]any{
					"type": "content_block_start", "index": blockIndex,
					"content_block": map[string]any{"type": "tool_use", "id": tc.Id, "name": tc.Function.Name},
				})
				blockOpen = true
				blockType = "tool_use"
			}
			if tc.Function.Arguments != "" {
				writeClaudeSSE(c, "content_block_delta", map[string]any{
					"type": "content_block_delta", "index": blockIndex,
					"delta": map[string]any{"type": "input_json_delta", "partial_json": tc.Function.Arguments},
				})
			}
		}

		// Finish
		if choice.FinishReason != nil {
			closeClaudeBlock(c, &blockOpen, &blockIndex)
			stopReason := openaiFinishReasonToClaude(*choice.FinishReason)
			outputTokens := 0
			if chunk.Usage != nil {
				outputTokens = chunk.Usage.CompletionTokens
			}
			writeClaudeSSE(c, "message_delta", map[string]any{
				"type": "message_delta",
				"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
				"usage": map[string]any{"output_tokens": outputTokens},
			})
			writeClaudeSSE(c, "message_stop", map[string]any{"type": "message_stop"})
			return nil, nil
		}
	}

	// Fallback: stream ended without finish_reason
	if blockOpen {
		closeClaudeBlock(c, &blockOpen, &blockIndex)
	}
	if msgId != "" {
		writeClaudeSSE(c, "message_delta", map[string]any{
			"type": "message_delta",
			"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
			"usage": map[string]any{"output_tokens": 0},
		})
		writeClaudeSSE(c, "message_stop", map[string]any{"type": "message_stop"})
	}
	return nil, nil
}

func writeClaudeSSE(c *gin.Context, event string, data any) {
	jsonData, _ := json.Marshal(data)
	c.Writer.WriteString(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData)))
	c.Writer.Flush()
}

func closeClaudeBlock(c *gin.Context, blockOpen *bool, blockIndex *int) {
	if *blockOpen {
		writeClaudeSSE(c, "content_block_stop", map[string]any{"type": "content_block_stop", "index": *blockIndex})
		*blockOpen = false
		*blockIndex++
	}
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
