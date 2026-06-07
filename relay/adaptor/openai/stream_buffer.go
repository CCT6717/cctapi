package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/conv"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/render"
	"github.com/songquanpeng/one-api/relay/model"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

// BufferedStreamThreshold is the number of SSE chunks to buffer
// before committing the stream to the client.
// If the stream fails before this threshold, the caller can fallback.
const BufferedStreamThreshold = 3

// BufferedStreamKeepAliveInterval is how often to send SSE keepalive
// comments during the buffering phase to prevent reverse proxy timeout.
const BufferedStreamKeepAliveInterval = 5 * time.Second

// BufferedStreamHandler handles SSE streaming with initial buffering for fallback support.
// It buffers the first BufferedStreamThreshold SSE events before writing to the client.
// If the upstream connection fails before the threshold is reached, it returns an error
// so the fallback loop can try the next deployment.
// Once committed, all data is written directly to the client (passthrough mode).
func BufferedStreamHandler(c *gin.Context, resp *http.Response, relayMode int) (*model.ErrorWithStatusCode, string, *model.Usage) {
	responseText := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	var usage *model.Usage
	var eventBuf bytes.Buffer
	eventsBuffered := 0
	committed := false
	doneRendered := false

	// Set SSE headers (only stored in ResponseWriter, not actually written yet)
	common.SetEventStreamHeaders(c)

	// --- Keepalive mechanism ---
	// During the buffering phase (before 3 chunks are committed) the client
	// sees no data.  Long gaps can trigger timeouts in reverse proxies such
	// as Nginx, Cloudflare or browser EventSource implementations.
	// We send SSE comment lines (`: keepalive\n\n`) on a regular interval
	// so the connection stays alive until we commit the real stream.
	keepaliveStop := make(chan struct{})
	var stopKeepaliveOnce sync.Once
	stopKeepalive := func() {
		stopKeepaliveOnce.Do(func() {
			close(keepaliveStop)
		})
	}
	defer stopKeepalive()

	// writeMu serialises access to c.Writer between the keepalive goroutine
	// and the main goroutine when it commits buffered events.
	var writeMu sync.Mutex

	go func() {
		ticker := time.NewTicker(BufferedStreamKeepAliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				c.Writer.Write([]byte(": keepalive\n\n"))
				c.Writer.Flush()
				writeMu.Unlock()
			case <-keepaliveStop:
				return
			}
		}
	}()
	// --- End keepalive ---

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < dataPrefixLength {
			continue
		}
		if line[:dataPrefixLength] != dataPrefix && line[:dataPrefixLength] != done {
			continue
		}

		// Handle [DONE] marker
		if strings.HasPrefix(line[dataPrefixLength:], done) {
			doneRendered = true
			if committed {
				render.StringData(c, line)
			}
			continue
		}

		// Parse the SSE data based on relay mode
		switch relayMode {
		case relaymode.ChatCompletions:
			var streamResponse ChatCompletionsStreamResponse
			err := json.Unmarshal([]byte(line[dataPrefixLength:]), &streamResponse)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				if committed {
					render.StringData(c, line)
				}
				continue
			}
			if len(streamResponse.Choices) == 0 && streamResponse.Usage == nil {
				continue
			}

			// Write the event (buffer or passthrough)
			if committed {
				render.StringData(c, line)
			} else {
				eventBuf.WriteString(line)
				eventBuf.WriteString("\n\n")
				eventsBuffered++
			}

			// Accumulate response text and usage
			for _, choice := range streamResponse.Choices {
				responseText += conv.AsString(choice.Delta.Content)
			}
			if streamResponse.Usage != nil {
				usage = streamResponse.Usage
			}

		case relaymode.Completions:
			var streamResponse CompletionsStreamResponse
			err := json.Unmarshal([]byte(line[dataPrefixLength:]), &streamResponse)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				if committed {
					render.StringData(c, line)
				}
				continue
			}

			if committed {
				render.StringData(c, line)
			} else {
				eventBuf.WriteString(line)
				eventBuf.WriteString("\n\n")
				eventsBuffered++
			}

			for _, choice := range streamResponse.Choices {
				responseText += choice.Text
			}

		default:
			// Unknown relay mode — just pass through raw
			if committed {
				render.StringData(c, line)
			} else {
				eventBuf.WriteString(line)
				eventBuf.WriteString("\n\n")
				eventsBuffered++
			}
		}

		// Auto-commit after enough chunks are buffered
		if !committed && eventsBuffered >= BufferedStreamThreshold {
			stopKeepalive()
			writeMu.Lock()
			c.Writer.Write(eventBuf.Bytes())
			c.Writer.Flush()
			writeMu.Unlock()
			committed = true
		}
	}

	// Check for scanner errors (connection dropped, upstream failure, etc.)
	if err := scanner.Err(); err != nil {
		if !committed {
			return ErrorWrapper(err, "stream_connection_failed", http.StatusBadGateway), responseText, usage
		}
		logger.SysError("[fallback] stream read error after commit: " + err.Error())
	}

	// Flush any remaining buffered events if threshold was never reached
	if !committed {
		stopKeepalive()
		if eventBuf.Len() > 0 {
			writeMu.Lock()
			c.Writer.Write(eventBuf.Bytes())
			c.Writer.Flush()
			writeMu.Unlock()
		}
		committed = true
	}

	// Send [DONE] if not yet sent
	if !doneRendered {
		render.Done(c)
	}

	_ = resp.Body.Close()
	return nil, responseText, usage
}
