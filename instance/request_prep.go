package instance

import (
	"encoding/json"
	"net/http"

	"copilot-go/anthropic"
	"copilot-go/config"
	"copilot-go/store"
)

func initiatorForMessages(messages []anthropic.OpenAIMessage) string {
	for _, msg := range messages {
		if msg.Role == "assistant" || msg.Role == "tool" {
			return "agent"
		}
	}
	return "user"
}

func applyDefaultMaxTokens(payload *anthropic.ChatCompletionsPayload, models *config.ModelsResponse) {
	if payload == nil || payload.MaxTokens > 0 || models == nil {
		return
	}
	for _, model := range models.Data {
		if model.ID == payload.Model || model.ID == store.ToCopilotID(payload.Model) {
			if model.Capabilities.Limits.MaxOutputTokens > 0 {
				payload.MaxTokens = model.Capabilities.Limits.MaxOutputTokens
			}
			return
		}
	}
}

func extraHeadersForMessages(messages []anthropic.OpenAIMessage) http.Header {
	headers := make(http.Header)
	headers.Set("X-Initiator", initiatorForMessages(messages))
	return headers
}

func rewriteCompletionsPayload(bodyBytes []byte, models *config.ModelsResponse) ([]byte, http.Header, bool, error) {
	var payload anthropic.ChatCompletionsPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return bodyBytes, nil, false, nil
	}
	applyDefaultMaxTokens(&payload, models)
	payload.Model = store.ToCopilotID(payload.Model)
	updatedBody, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, false, err
	}
	return updatedBody, extraHeadersForMessages(payload.Messages), hasOpenAIVisionContent(payload.Messages), nil
}

func hasOpenAIVisionContent(messages []anthropic.OpenAIMessage) bool {
	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case []anthropic.OpenAIContentPart:
			for _, part := range content {
				if part.Type == "image_url" {
					return true
				}
			}
		case []interface{}:
			for _, raw := range content {
				part, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if partType, _ := part["type"].(string); partType == "image_url" {
					return true
				}
			}
		}
	}
	return false
}
