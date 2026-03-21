package upstreamerr

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleUpstreamError logs the raw upstream error and writes a sanitised error
// response to the downstream client. Use this for non-streaming responses and
// for streaming requests whose first response is already an error (i.e. the
// SSE stream has not started yet).
func HandleUpstreamError(
	c *gin.Context,
	upstreamStatus int,
	upstreamBody []byte,
	format ResponseFormat,
	endpoint string,
	accountName string,
) {
	logUpstreamError(endpoint, upstreamStatus, upstreamBody, c.ClientIP(), c.Request.URL.Path, accountName)

	// Only rewrite body when it contains "Copilot" to avoid leaking internal
	// implementation details.  Otherwise, pass through the original upstream
	// error so downstream callers can see the real reason.
	if containsCopilot(upstreamBody) {
		ue := Lookup(upstreamStatus)
		body := BuildErrorBody(ue, format)
		c.Data(ue.StatusCode, "application/json", body)
	} else {
		c.Data(upstreamStatus, "application/json", upstreamBody)
	}
}

// HandleUpstreamErrorSSE logs the raw upstream error and writes an Anthropic
// SSE error event into an already-open event stream. Use this when the SSE
// stream is in progress and an error occurs mid-flight (e.g. scanner error).
func HandleUpstreamErrorSSE(
	w io.Writer,
	upstreamStatus int,
	upstreamBody []byte,
	endpoint string,
	accountName string,
) {
	logUpstreamError(endpoint, upstreamStatus, upstreamBody, "", "", accountName)

	if containsCopilot(upstreamBody) {
		ue := Lookup(upstreamStatus)
		data := BuildSSEErrorData(ue)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
	} else {
		// Transparently forward the original upstream error as SSE event.
		ue := Lookup(upstreamStatus)
		ue.Message = string(upstreamBody)
		data := BuildSSEErrorData(ue)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// containsCopilot reports whether the body contains the word "Copilot"
// (case-insensitive). When present, the response likely exposes internal
// Copilot implementation details that should be sanitised before forwarding.
func containsCopilot(body []byte) bool {
	return bytes.Contains(bytes.ToLower(body), []byte("copilot"))
}

// logUpstreamError writes a structured log line with all available context.
func logUpstreamError(endpoint string, status int, body []byte, clientIP, path string, accountName string) {
	truncated := truncateBody(body, 2048)
	if clientIP != "" {
		log.Printf("[UpstreamError] endpoint=%s upstream_status=%d account=%s body=%s client_ip=%s path=%s",
			endpoint, status, accountName, truncated, clientIP, path)
	} else {
		log.Printf("[UpstreamError] endpoint=%s upstream_status=%d account=%s body=%s",
			endpoint, status, accountName, truncated)
	}
}

// truncateBody caps the body at maxLen bytes to prevent log explosion.
func truncateBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + fmt.Sprintf("... (truncated, total %d bytes)", len(body))
}
