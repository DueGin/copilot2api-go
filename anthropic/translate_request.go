package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"copilot-go/store"
)

// TranslateToOpenAI converts an Anthropic messages payload to OpenAI chat completions payload.
func TranslateToOpenAI(payload AnthropicMessagesPayload) ChatCompletionsPayload {
	result := ChatCompletionsPayload{
		Model:       store.ToCopilotID(normalizeAnthropicModelName(payload.Model)),
		Stream:      payload.Stream,
		Temperature: payload.Temperature,
		TopP:        payload.TopP,
		User:        payloadMetadataUser(payload.Metadata),
	}

	if payload.MaxTokens > 0 {
		result.MaxTokens = payload.MaxTokens
	}
	if payload.Stream {
		result.StreamOptions = &StreamOptions{IncludeUsage: true}
	}
	if len(payload.StopSequences) > 0 {
		result.Stop = payload.StopSequences
	}
	result.Messages = translateAnthropicMessagesToOpenAI(payload.Messages, payload.System)
	if len(payload.Tools) > 0 {
		result.Tools = translateAnthropicToolsToOpenAI(payload.Tools)
	}
	if payload.ToolChoice != nil {
		result.ToolChoice = convertToolChoice(payload.ToolChoice)
	}

	return result
}

func normalizeAnthropicModelName(model string) string {
	switch {
	case strings.HasPrefix(model, "claude-sonnet-4-"):
		return "claude-sonnet-4"
	case strings.HasPrefix(model, "claude-opus-4-"):
		return "claude-opus-4"
	default:
		return model
	}
}

func payloadMetadataUser(metadata *Metadata) string {
	if metadata == nil {
		return ""
	}
	return metadata.UserID
}

func translateAnthropicMessagesToOpenAI(anthropicMessages []AnthropicMessage, system interface{}) []OpenAIMessage {
	systemMessages := handleSystemPrompt(system)
	otherMessages := make([]OpenAIMessage, 0, len(anthropicMessages))
	for _, message := range anthropicMessages {
		if message.Role == "user" {
			otherMessages = append(otherMessages, handleUserMessage(message)...)
			continue
		}
		otherMessages = append(otherMessages, handleAssistantMessage(message)...)
	}
	return append(systemMessages, otherMessages...)
}

func handleSystemPrompt(system interface{}) []OpenAIMessage {
	if system == nil {
		return nil
	}

	systemText := extractSystemText(system)
	if systemText == "" {
		return nil
	}
	return []OpenAIMessage{{Role: "system", Content: systemText}}
}

func extractSystemText(system interface{}) string {
	switch v := system.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n\n")
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		var blocks []SystemBlock
		if err := json.Unmarshal(data, &blocks); err != nil {
			return string(data)
		}
		parts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			parts = append(parts, block.Text)
		}
		return strings.Join(parts, "\n\n")
	}
}

func convertMessage(msg AnthropicMessage) []OpenAIMessage {
	if str, ok := msg.Content.(string); ok {
		return []OpenAIMessage{{Role: msg.Role, Content: str}}
	}

	blocks := parseContentBlocks(msg.Content)
	if len(blocks) == 0 {
		return []OpenAIMessage{{Role: msg.Role, Content: ""}}
	}
	if msg.Role == "assistant" {
		return convertAssistantMessage(blocks)
	}
	if msg.Role == "user" {
		return convertUserMessage(blocks)
	}

	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return []OpenAIMessage{{Role: msg.Role, Content: strings.Join(texts, "\n\n")}}
}

func handleAssistantMessage(message AnthropicMessage) []OpenAIMessage {
	return convertMessage(message)
}

func convertAssistantMessage(blocks []ContentBlock) []OpenAIMessage {
	textBlocks := make([]string, 0, len(blocks))
	thinkingBlocks := make([]string, 0, len(blocks))
	toolCalls := make([]ToolCall, 0)

	for _, block := range blocks {
		switch block.Type {
		case "text":
			textBlocks = append(textBlocks, block.Text)
		case "thinking":
			if block.Thinking != "" {
				thinkingBlocks = append(thinkingBlocks, block.Thinking)
			}
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      block.Name,
					Arguments: marshalJSON(block.Input),
				},
			})
		}
	}

	allTextContent := strings.Join(append(textBlocks, thinkingBlocks...), "\n\n")
	msg := OpenAIMessage{Role: "assistant"}
	if allTextContent != "" {
		msg.Content = allTextContent
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	if len(toolCalls) == 0 && msg.Content == nil {
		msg.Content = ""
	}
	return []OpenAIMessage{msg}
}

func handleUserMessage(message AnthropicMessage) []OpenAIMessage {
	blocks := parseContentBlocks(message.Content)
	if !isContentArray(message.Content) {
		return []OpenAIMessage{{Role: "user", Content: mapContent(message.Content)}}
	}

	toolResultBlocks := make([]ContentBlock, 0)
	otherBlocks := make([]ContentBlock, 0)
	for _, block := range blocks {
		if block.Type == "tool_result" {
			toolResultBlocks = append(toolResultBlocks, block)
			continue
		}
		otherBlocks = append(otherBlocks, block)
	}

	newMessages := make([]OpenAIMessage, 0, len(toolResultBlocks)+1)
	for _, block := range toolResultBlocks {
		newMessages = append(newMessages, OpenAIMessage{
			Role:       "tool",
			ToolCallID: block.ToolUseID,
			Content:    mapContent(block.Content2),
		})
	}
	if len(otherBlocks) > 0 {
		newMessages = append(newMessages, OpenAIMessage{
			Role:    "user",
			Content: mapContent(otherBlocks),
		})
	}
	if len(newMessages) == 0 {
		newMessages = append(newMessages, OpenAIMessage{Role: "user", Content: ""})
	}
	return newMessages
}

func convertUserMessage(blocks []ContentBlock) []OpenAIMessage {
	message := AnthropicMessage{Role: "user", Content: blocks}
	return handleUserMessage(message)
}

func mapContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []ContentBlock:
		return mapContentBlocks(v)
	case []interface{}:
		return mapContentBlocks(parseContentBlocks(v))
	default:
		blocks := parseContentBlocks(v)
		if len(blocks) > 0 {
			return mapContentBlocks(blocks)
		}
		return nil
	}
}

func mapContentBlocks(blocks []ContentBlock) interface{} {
	hasImage := false
	for _, block := range blocks {
		if block.Type == "image" {
			hasImage = true
			break
		}
	}
	if !hasImage {
		parts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			switch block.Type {
			case "text":
				parts = append(parts, block.Text)
			case "thinking":
				parts = append(parts, block.Thinking)
			}
		}
		return strings.Join(parts, "\n\n")
	}

	contentParts := make([]OpenAIContentPart, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			contentParts = append(contentParts, OpenAIContentPart{Type: "text", Text: block.Text})
		case "thinking":
			contentParts = append(contentParts, OpenAIContentPart{Type: "text", Text: block.Thinking})
		case "image":
			if block.Source != nil {
				contentParts = append(contentParts, OpenAIContentPart{
					Type:     "image_url",
					ImageURL: &OpenAIImageURL{URL: fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)},
				})
			}
		}
	}
	return contentParts
}

func translateAnthropicToolsToOpenAI(anthropicTools []AnthropicTool) []OpenAITool {
	tools := make([]OpenAITool, 0, len(anthropicTools))
	for _, tool := range anthropicTools {
		tools = append(tools, OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return tools
}

func convertToolChoice(tc interface{}) interface{} {
	switch v := tc.(type) {
	case string:
		switch v {
		case "auto":
			return "auto"
		case "any":
			return "required"
		case "none":
			return "none"
		default:
			return nil
		}
	case map[string]interface{}:
		typeValue, _ := v["type"].(string)
		switch typeValue {
		case "auto":
			return "auto"
		case "any":
			return "required"
		case "none":
			return "none"
		case "tool":
			name, _ := v["name"].(string)
			if name == "" {
				return nil
			}
			return map[string]interface{}{
				"type": "function",
				"function": map[string]string{
					"name": name,
				},
			}
		default:
			return nil
		}
	default:
		return nil
	}
}

func marshalJSON(value interface{}) string {
	if value == nil {
		return "null"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func isContentArray(content interface{}) bool {
	switch content.(type) {
	case []ContentBlock, []interface{}:
		return true
	default:
		return false
	}
}

// ParseContentBlocksPublic is the exported version of parseContentBlocks.
func ParseContentBlocksPublic(content interface{}) []ContentBlock {
	return parseContentBlocks(content)
}

func parseContentBlocks(content interface{}) []ContentBlock {
	data, err := json.Marshal(content)
	if err != nil {
		return nil
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil
	}
	return blocks
}
