package anthropic

func MapOpenAIStopReasonToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

func mapOpenAIStopReasonToAnthropicPtr(reason *string) *string {
	if reason == nil {
		return nil
	}
	mapped := MapOpenAIStopReasonToAnthropic(*reason)
	return &mapped
}

func cachedPromptTokens(usage *OpenAIUsage) int {
	if usage == nil || usage.PromptTokensDetails == nil {
		return 0
	}
	return usage.PromptTokensDetails.CachedTokens
}

func promptInputTokens(usage *OpenAIUsage) int {
	if usage == nil {
		return 0
	}
	return usage.PromptTokens - cachedPromptTokens(usage)
}

func completionTokens(usage *OpenAIUsage) int {
	if usage == nil {
		return 0
	}
	return usage.CompletionTokens
}
