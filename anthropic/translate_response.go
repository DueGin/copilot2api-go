package anthropic

// TranslateToAnthropic converts an OpenAI chat completion response to Anthropic format.
func TranslateToAnthropic(resp ChatCompletionResponse) AnthropicResponse {
	allTextBlocks := make([]AnthropicContentBlock, 0)
	allToolUseBlocks := make([]AnthropicContentBlock, 0)
	var finishReason *string
	if len(resp.Choices) > 0 {
		finishReason = resp.Choices[0].FinishReason
	}

	for _, choice := range resp.Choices {
		if choice.Message == nil {
			continue
		}
		if textBlocks := getAnthropicTextBlocks(choice.Message.Content); len(textBlocks) > 0 {
			allTextBlocks = append(allTextBlocks, textBlocks...)
		}
		if toolUseBlocks := getAnthropicToolUseBlocks(choice.Message.ToolCalls); len(toolUseBlocks) > 0 {
			allToolUseBlocks = append(allToolUseBlocks, toolUseBlocks...)
		}
		if choice.FinishReason != nil && (*choice.FinishReason == "tool_calls" || finishReason == nil || *finishReason == "stop") {
			finishReason = choice.FinishReason
		}
	}

	return AnthropicResponse{
		ID:           resp.ID,
		Type:         "message",
		Role:         "assistant",
		Model:        resp.Model,
		Content:      append(allTextBlocks, allToolUseBlocks...),
		StopReason:   mapOpenAIStopReasonToAnthropicPtr(finishReason),
		StopSequence: nil,
		Usage: AnthropicUsage{
			InputTokens:          promptInputTokens(resp.Usage),
			OutputTokens:         completionTokens(resp.Usage),
			CacheReadInputTokens: cachedPromptTokens(resp.Usage),
		},
	}
}

func getAnthropicTextBlocks(messageContent interface{}) []AnthropicContentBlock {
	switch content := messageContent.(type) {
	case string:
		if content == "" {
			return nil
		}
		return []AnthropicContentBlock{{Type: "text", Text: content}}
	case []OpenAIContentPart:
		blocks := make([]AnthropicContentBlock, 0, len(content))
		for _, part := range content {
			if part.Type == "text" {
				blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: part.Text})
			}
		}
		return blocks
	case []interface{}:
		parts := make([]AnthropicContentBlock, 0, len(content))
		for _, raw := range content {
			part, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if partType, _ := part["type"].(string); partType == "text" {
				if text, _ := part["text"].(string); text != "" {
					parts = append(parts, AnthropicContentBlock{Type: "text", Text: text})
				}
			}
		}
		return parts
	default:
		return nil
	}
}

func getAnthropicToolUseBlocks(toolCalls []ToolCall) []AnthropicContentBlock {
	if len(toolCalls) == 0 {
		return nil
	}
	blocks := make([]AnthropicContentBlock, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		blocks = append(blocks, AnthropicContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: parseJSONSafe(toolCall.Function.Arguments),
		})
	}
	return blocks
}

func parseJSONSafe(s string) interface{} {
	var result interface{}
	if err := jsonUnmarshal([]byte(s), &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}
