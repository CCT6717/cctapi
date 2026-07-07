package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/claudeutil"
	"github.com/songquanpeng/one-api/common/ctxkey"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

type responsesCaptureWriter struct {
	gin.ResponseWriter
	header     http.Header
	body       bytes.Buffer
	statusCode int
	wroteCode  bool
}

var relayResponsesRelay = Relay

func newResponsesCaptureWriter(real gin.ResponseWriter) *responsesCaptureWriter {
	return &responsesCaptureWriter{
		ResponseWriter: real,
		header:         make(http.Header),
		statusCode:     http.StatusOK,
	}
}

func (w *responsesCaptureWriter) Header() http.Header {
	return w.header
}

func (w *responsesCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteCode = true
}

func (w *responsesCaptureWriter) WriteHeaderNow() {
	if !w.wroteCode {
		w.WriteHeader(http.StatusOK)
	}
}

func (w *responsesCaptureWriter) Write(data []byte) (int, error) {
	if !w.wroteCode {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(data)
}

func (w *responsesCaptureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responsesCaptureWriter) Flush() {}

func (w *responsesCaptureWriter) Status() int {
	return w.statusCode
}

func (w *responsesCaptureWriter) Written() bool {
	return w.wroteCode || w.body.Len() > 0
}

func (w *responsesCaptureWriter) Size() int {
	return w.body.Len()
}

func (w *responsesCaptureWriter) BodyBytes() []byte {
	return w.body.Bytes()
}

func (w *responsesCaptureWriter) BodyString() string {
	return w.body.String()
}

func rewriteResponsesContextForChatRelay(c *gin.Context, body []byte, modelName string) func() {
	oldPath := c.Request.URL.Path
	oldRawPath := c.Request.URL.RawPath
	oldRequestURI := c.Request.RequestURI
	oldBody := c.Request.Body
	oldCachedBody, hadCachedBody := c.Get(ctxkey.KeyRequestBody)
	oldRequestModel, hadRequestModel := c.Get(ctxkey.RequestModel)

	c.Request.URL.Path = "/v1/chat/completions"
	c.Request.URL.RawPath = ""
	c.Request.RequestURI = "/v1/chat/completions"
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Set(ctxkey.KeyRequestBody, body)
	c.Set(ctxkey.RequestModel, modelName)

	return func() {
		c.Request.URL.Path = oldPath
		c.Request.URL.RawPath = oldRawPath
		c.Request.RequestURI = oldRequestURI
		c.Request.Body = oldBody
		if hadCachedBody {
			c.Set(ctxkey.KeyRequestBody, oldCachedBody)
		} else if c.Keys != nil {
			delete(c.Keys, ctxkey.KeyRequestBody)
		}
		if hadRequestModel {
			c.Set(ctxkey.RequestModel, oldRequestModel)
		} else if c.Keys != nil {
			delete(c.Keys, ctxkey.RequestModel)
		}
	}
}

func responsesConversionStatus(capturedStatus, convertedStatus int) int {
	if capturedStatus == http.StatusOK && convertedStatus != http.StatusOK {
		return convertedStatus
	}
	return capturedStatus
}

func withResponsesCaptureWriter(c *gin.Context, fn func()) *responsesCaptureWriter {
	realWriter := c.Writer
	capture := newResponsesCaptureWriter(realWriter)
	c.Writer = capture
	defer func() {
		c.Writer = realWriter
	}()
	fn()
	return capture
}

func RelayResponses(c *gin.Context) {
	var req relaymodel.ResponsesRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	chatReq, err := req.ToChatRequest()
	if err != nil {
		var unsupported *relaymodel.ResponsesUnsupportedInputError
		statusCode := http.StatusBadRequest
		if errors.As(err, &unsupported) {
			statusCode = http.StatusUnprocessableEntity
		}
		claudeutil.WriteClaudeOrOpenAIError(c, statusCode, "invalid_request_error", err.Error())
		return
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_error", err.Error())
		return
	}

	restore := rewriteResponsesContextForChatRelay(c, chatBody, chatReq.Model)
	defer restore()

	capture := withResponsesCaptureWriter(c, func() {
		relayResponsesRelay(c)
	})

	if chatReq.Stream {
		writeResponsesStream(c, capture.BodyBytes(), chatReq.Model, capture.Status())
		return
	}

	resp, convertedStatus, err := relaymodel.ChatCompletionToResponses(capture.BodyBytes(), chatReq.Model)
	if err != nil {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_error", err.Error())
		return
	}

	c.JSON(responsesConversionStatus(capture.Status(), convertedStatus), resp)
}

func writeResponsesStream(c *gin.Context, raw []byte, modelName string, status int) {
	if !responsesStreamHasUsefulDataFrame(raw) {
		events := []relaymodel.ResponsesSSEEvent{
			relaymodel.ResponsesStreamFailureEvent(fmt.Sprintf("upstream stream returned HTTP %d without any useful SSE data", status), status),
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Status(status)
		if err := relaymodel.WriteResponsesSSE(c.Writer, events); err != nil {
			c.Error(err)
		}
		return
	}

	events, err := relaymodel.ChatCompletionStreamToResponsesEvents(raw, modelName)
	if err != nil {
		claudeutil.WriteClaudeOrOpenAIError(c, http.StatusInternalServerError, "one_api_error", err.Error())
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(status)
	if err := relaymodel.WriteResponsesSSE(c.Writer, events); err != nil {
		c.Error(err)
	}
}

func responsesStreamHasUsefulDataFrame(raw []byte) bool {
	var errPayload struct {
		StatusCode int              `json:"status_code"`
		Error      relaymodel.Error `json:"error"`
	}
	if err := json.Unmarshal(raw, &errPayload); err == nil && errPayload.Error.Message != "" {
		return true
	}

	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		return true
	}
	return false
}
