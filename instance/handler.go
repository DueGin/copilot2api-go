package instance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"copilot-go/anthropic"
	"copilot-go/config"
	"copilot-go/store"

	"github.com/gin-gonic/gin"
)

// DoCompletionsProxy performs the upstream request for completions and returns the raw response.
// The caller is responsible for closing resp.Body.
func DoCompletionsProxy(c *gin.Context, state *config.State, bodyBytes []byte) (*http.Response, error) {
	state.RLock()
	models := state.Models
	state.RUnlock()

	rewrittenBody, extraHeaders, hasVision, err := rewriteCompletionsPayload(bodyBytes, models)
	if err != nil {
		return nil, err
	}

	return ProxyRequestWithBytesCtx(c.Request.Context(), state, "POST", "/chat/completions", rewrittenBody, extraHeaders, hasVision)
}

// ForwardCompletionsResponse writes the upstream response to the client.
func ForwardCompletionsResponse(c *gin.Context, resp *http.Response) {
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	if isStream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(resp.StatusCode)

		reader := bufio.NewReaderSize(resp.Body, 10*1024*1024)
		c.Stream(func(w io.Writer) bool {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				if _, writeErr := w.Write(line); writeErr != nil {
					return false
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("Stream read error: %v", err)
				}
				return false
			}
			return true
		})
	} else {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response"})
			return
		}
		c.Data(resp.StatusCode, "application/json", body)
	}
}

// ModelsHandler returns cached models with display ID mapping.
func ModelsHandler(c *gin.Context, state *config.State) {
	state.RLock()
	models := state.Models
	state.RUnlock()

	if models == nil {
		c.JSON(http.StatusOK, config.ModelsResponse{Object: "list", Data: []config.ModelEntry{}})
		return
	}

	mapped := config.ModelsResponse{Object: models.Object, Data: make([]config.ModelEntry, len(models.Data))}
	for i, model := range models.Data {
		mapped.Data[i] = config.ModelEntry{
			ID:                 store.ToDisplayID(model.ID),
			Object:             model.Object,
			Created:            model.Created,
			OwnedBy:            firstNonEmpty(model.OwnedBy, model.Vendor),
			Capabilities:       model.Capabilities,
			ModelPickerEnabled: model.ModelPickerEnabled,
			Name:               model.Name,
			Preview:            model.Preview,
			Vendor:             model.Vendor,
			Version:            model.Version,
			Policy:             model.Policy,
		}
	}

	c.JSON(http.StatusOK, mapped)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// DoEmbeddingsProxy performs the upstream request for embeddings.
func DoEmbeddingsProxy(state *config.State, bodyBytes []byte) (*http.Response, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err == nil {
		if model, ok := payload["model"].(string); ok {
			payload["model"] = store.ToCopilotID(model)
			bodyBytes, _ = json.Marshal(payload)
		}
	}

	return ProxyRequestWithBytes(state, "POST", "/embeddings", bodyBytes, nil, false)
}

// ForwardEmbeddingsResponse writes the upstream embeddings response to the client.
func ForwardEmbeddingsResponse(c *gin.Context, resp *http.Response) {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response"})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}

// DoMessagesProxy performs the upstream request for Anthropic messages.
// Returns the raw response. bodyBytes is the original Anthropic payload.
func DoMessagesProxy(c *gin.Context, state *config.State, bodyBytes []byte) (*http.Response, error) {
	var anthropicPayload anthropic.AnthropicMessagesPayload
	if err := json.Unmarshal(bodyBytes, &anthropicPayload); err != nil {
		return nil, fmt.Errorf("invalid request: %v", err)
	}

	hasVision := checkVisionContent(anthropicPayload)
	openaiPayload := anthropic.TranslateToOpenAI(anthropicPayload)
	openaiBytes, err := json.Marshal(openaiPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	extraHeaders := extraHeadersForMessages(openaiPayload.Messages)
	return ProxyRequestWithBytesCtx(c.Request.Context(), state, "POST", "/chat/completions", openaiBytes, extraHeaders, hasVision)
}

// ForwardMessagesResponse writes the upstream response to the client in Anthropic format.
// originalBody is the original Anthropic request (used to determine stream mode).
func ForwardMessagesResponse(c *gin.Context, resp *http.Response, originalBody []byte) {
	defer func() { _ = resp.Body.Close() }()

	var anthropicPayload anthropic.AnthropicMessagesPayload
	if err := json.Unmarshal(originalBody, &anthropicPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	if anthropicPayload.Stream {
		handleAnthropicStream(c, resp)
	} else {
		handleAnthropicNonStream(c, resp)
	}
}

func handleAnthropicNonStream(c *gin.Context, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	var openaiResp anthropic.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse upstream response"})
		return
	}

	c.JSON(http.StatusOK, anthropic.TranslateToAnthropic(openaiResp))
}

func handleAnthropicStream(c *gin.Context, resp *http.Response) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Stream] Upstream returned status %d: %s", resp.StatusCode, string(body))
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	w := c.Writer
	flusher, hasFlusher := w.(http.Flusher)
	clientGone := c.Request.Context().Done()

	state := anthropic.NewStreamState()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-clientGone:
			log.Printf("[Stream] Client disconnected, stopping stream")
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			if hasFlusher {
				flusher.Flush()
			}
			return
		}

		var chunk anthropic.ChatCompletionResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[Stream] Failed to parse SSE chunk: %v", err)
			continue
		}

		events := anthropic.TranslateChunkToAnthropicEvents(chunk, state)
		for _, event := range events {
			if err := writeSSE(w, event.Event, event.Data); err != nil {
				log.Printf("[Stream] Write error: %v", err)
				return
			}
		}
		if hasFlusher {
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[Stream] Scanner error: %v", err)
		_ = writeSSE(w, "error", anthropic.ErrorEvent{
			Type: "error",
			Error: anthropic.ErrorData{
				Type:    "stream_error",
				Message: fmt.Sprintf("upstream stream error: %v", err),
			},
		})
		if hasFlusher {
			flusher.Flush()
		}
	}
}

func writeSSE(w io.Writer, event string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	return err
}

// CountTokensHandler provides a target-compatible token count estimation.
func CountTokensHandler(c *gin.Context, state *config.State) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var payload anthropic.AnthropicMessagesPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	state.RLock()
	models := state.Models
	state.RUnlock()

	inputTokens := calculateAnthropicTokenCountOrDefault(payload, c.GetHeader("anthropic-beta"), models)
	c.JSON(http.StatusOK, gin.H{"input_tokens": inputTokens})
}

func checkVisionContent(payload anthropic.AnthropicMessagesPayload) bool {
	for _, msg := range payload.Messages {
		blocks := anthropic.ParseContentBlocksPublic(msg.Content)
		for _, b := range blocks {
			if b.Type == "image" {
				return true
			}
		}
	}
	return false
}
