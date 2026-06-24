package anthropic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
)

// claudeStreamChunk is a minimal struct for unmarshalling OpenAI SSE chunks.
// Defined locally to avoid importing the openai adaptor package.
type claudeStreamChunk struct {
	Id      string                   `json:"id"`
	Model   string                   `json:"model"`
	Choices []claudeStreamChoice     `json:"choices"`
	Usage   *claudeStreamUsage       `json:"usage,omitempty"`
}

type claudeStreamChoice struct {
	Delta        claudeStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason,omitempty"`
}

type claudeStreamDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   any                   `json:"content,omitempty"`
	ToolCalls []claudeStreamToolCall `json:"tool_calls,omitempty"`
}

type claudeStreamToolCall struct {
	Index    int                      `json:"index"`
	Id       string                   `json:"id,omitempty"`
	Type     string                   `json:"type,omitempty"`
	Function claudeStreamFunctionCall `json:"function"`
}

type claudeStreamFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type claudeStreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// writeClaudeEvent writes a Claude-formatted SSE event: event: <type>\ndata: <json>\n\n
func writeClaudeEvent(c *gin.Context, eventType string, data any) {
	jsonData, _ := json.Marshal(data)
	c.Writer.WriteString(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData)))
	c.Writer.Flush()
}

// ConvertOpenAIStreamToClaude reads an OpenAI SSE stream and emits Claude SSE events.
func ConvertOpenAIStreamToClaude(c *gin.Context, resp *http.Response) {
	defer resp.Body.Close()

	common.SetEventStreamHeaders(c)

	scanner := bufio.NewScanner(resp.Body)

	// State
	var msgId string
	var inputTokens int
	var blockOpen bool
	var blockType string // "text" or "tool_use"
	var blockIndex int

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			break
		}

		var chunk claudeStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		// Accumulate usage if present (some providers send per-chunk usage)
		if chunk.Usage != nil {
			inputTokens += chunk.Usage.PromptTokens
		}

		// First chunk: emit message_start + content_block_start(text)
		if msgId == "" {
			msgId = "msg_" + strings.TrimPrefix(chunk.Id, "chatcmpl-")
			writeClaudeEvent(c, "message_start", map[string]any{
				"message": map[string]any{
					"id":            msgId,
					"type":          "message",
					"role":          "assistant",
					"content":       []any{},
					"model":         chunk.Model,
					"stop_reason":   nil,
					"stop_sequence": nil,
					"usage":         map[string]any{"input_tokens": inputTokens, "output_tokens": 0},
				},
			})
			writeClaudeEvent(c, "ping", map[string]any{"type": "ping"})
			writeClaudeEvent(c, "content_block_start", map[string]any{
				"type":         "content_block_start",
				"index":        blockIndex,
				"content_block": map[string]any{"type": "text", "text": ""},
			})
			blockOpen = true
			blockType = "text"
		}

		// Text content
		if text, ok := choice.Delta.Content.(string); ok && text != "" {
			if !blockOpen || blockType != "text" {
				closeCurrentBlock(c, &blockOpen, &blockIndex)
				writeClaudeEvent(c, "content_block_start", map[string]any{
					"type":         "content_block_start",
					"index":        blockIndex,
					"content_block": map[string]any{"type": "text", "text": ""},
				})
				blockOpen = true
				blockType = "text"
			}
			writeClaudeEvent(c, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": map[string]any{"type": "text_delta", "text": text},
			})
		}

		// Tool calls
		for _, tc := range choice.Delta.ToolCalls {
			if tc.Id != "" {
				// New tool_use block
				closeCurrentBlock(c, &blockOpen, &blockIndex)
				writeClaudeEvent(c, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": blockIndex,
					"content_block": map[string]any{
						"type": "tool_use",
						"id":   tc.Id,
						"name": tc.Function.Name,
					},
				})
				blockOpen = true
				blockType = "tool_use"
			}
			if tc.Function.Arguments != "" {
				writeClaudeEvent(c, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": blockIndex,
					"delta": map[string]any{"type": "input_json_delta", "partial_json": tc.Function.Arguments},
				})
			}
		}

		// Finish reason
		if choice.FinishReason != nil {
			closeCurrentBlock(c, &blockOpen, &blockIndex)
			stopReason := convertOpenAIFinishReason(*choice.FinishReason)
			outputTokens := 0
			if chunk.Usage != nil {
				outputTokens = chunk.Usage.CompletionTokens
			}
			writeClaudeEvent(c, "message_delta", map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
				"usage": map[string]any{"output_tokens": outputTokens},
			})
			writeClaudeEvent(c, "message_stop", map[string]any{"type": "message_stop"})
			return
		}
	}

	// Fallback: stream ended without finish_reason (e.g. network interruption)
	if blockOpen {
		closeCurrentBlock(c, &blockOpen, &blockIndex)
	}
	if msgId != "" {
		writeClaudeEvent(c, "message_delta", map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
			"usage": map[string]any{"output_tokens": 0},
		})
		writeClaudeEvent(c, "message_stop", map[string]any{"type": "message_stop"})
	}
}

func closeCurrentBlock(c *gin.Context, blockOpen *bool, blockIndex *int) {
	if *blockOpen {
		writeClaudeEvent(c, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": *blockIndex,
		})
		*blockOpen = false
		*blockIndex++
	}
}
