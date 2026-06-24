package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/relay/adaptor/anthropic"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/model"
)

// bufferedResponseWriter wraps gin.ResponseWriter to capture output.
// Used for Claude format conversion: buffer the adaptor's OpenAI output,
// convert to Claude format, then write to the real writer.
type bufferedResponseWriter struct {
	gin.ResponseWriter
	buf        bytes.Buffer
	statusCode int
	wroteHeader bool
}

func (w *bufferedResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteHeader = true
}

func (w *bufferedResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.statusCode = http.StatusOK
		w.wroteHeader = true
	}
	return w.buf.Write(b)
}

func (w *bufferedResponseWriter) Flush() {
	// Don't flush to real writer — we need to convert first
}

func (w *bufferedResponseWriter) WriteString(s string) (int, error) {
	return w.buf.WriteString(s)
}

func (w *bufferedResponseWriter) Status() int {
	return w.statusCode
}

func (w *bufferedResponseWriter) Written() bool {
	return w.buf.Len() > 0
}

func (w *bufferedResponseWriter) Size() int {
	return w.buf.Len()
}

func (w *bufferedResponseWriter) PushNotify() {}

// flushTo writes the buffered content to the real response writer.
func (w *bufferedResponseWriter) flushTo(real gin.ResponseWriter) {
	if w.wroteHeader {
		real.WriteHeader(w.statusCode)
	}
	real.Write(w.buf.Bytes())
}

// convertAndWriteClaudeResponse converts the buffered OpenAI response to Claude format.
func convertAndWriteClaudeResponse(c *gin.Context, buf *bufferedResponseWriter) {
	body := buf.buf.Bytes()
	if len(body) == 0 {
		buf.flushTo(c.Writer)
		return
	}

	// Check for error response
	var errResp struct {
		Error model.Error `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(buf.statusCode)
		claudeErr := map[string]any{
			"type":  "error",
			"error": map[string]any{"type": errResp.Error.Type, "message": errResp.Error.Message},
		}
		json.NewEncoder(c.Writer).Encode(claudeErr)
		return
	}

	// Normal response: unmarshal OpenAI, convert to Claude
	var openaiResp openai.TextResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		// Can't parse — pass through as-is
		buf.flushTo(c.Writer)
		return
	}

	claudeResp := anthropic.ConvertOpenAIResponseToClaude(&openaiResp)
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(buf.statusCode)
	json.NewEncoder(c.Writer).Encode(claudeResp)
}

// convertAndWriteClaudeStream converts buffered OpenAI SSE to Claude SSE format.
func convertAndWriteClaudeStream(c *gin.Context, buf *bufferedResponseWriter) {
	// Wrap buffered bytes as an http.Response.Body for ConvertOpenAIStreamToClaude
	fakeResp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(buf.buf.Bytes())),
	}
	anthropic.ConvertOpenAIStreamToClaude(c, fakeResp)
}
