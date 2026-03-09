package anthropic

import "testing"

func strPtr(s string) *string { return &s }

func TestTranslateToAnthropic_MapsUsageAndSkipsEmptyFallbackBlock(t *testing.T) {
	t.Parallel()

	resp := ChatCompletionResponse{
		ID:    "chatcmpl-1",
		Model: "gpt-4.1",
		Choices: []Choice{{
			Index:        0,
			Message:      &ChoiceMsg{Role: "assistant", Content: ""},
			FinishReason: strPtr("stop"),
		}},
		Usage: &OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 2,
			PromptTokensDetails: &PromptTokensDetails{
				CachedTokens: 3,
			},
		},
	}

	got := TranslateToAnthropic(resp)
	if got.StopReason == nil || *got.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %#v", got.StopReason)
	}
	if got.StopSequence != nil {
		t.Fatalf("stop_sequence = %#v, want nil", got.StopSequence)
	}
	if len(got.Content) != 0 {
		t.Fatalf("content len = %d, want 0", len(got.Content))
	}
	if got.Usage.InputTokens != 7 {
		t.Fatalf("input_tokens = %d, want 7", got.Usage.InputTokens)
	}
	if got.Usage.CacheReadInputTokens != 3 {
		t.Fatalf("cache_read_input_tokens = %d, want 3", got.Usage.CacheReadInputTokens)
	}
}

func TestTranslateToAnthropic_MapsToolCallsAndSafelyParsesInvalidJSON(t *testing.T) {
	t.Parallel()

	resp := ChatCompletionResponse{
		ID:    "chatcmpl-2",
		Model: "gpt-4.1",
		Choices: []Choice{{
			Index: 0,
			Message: &ChoiceMsg{
				Role:    "assistant",
				Content: "hello",
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: FunctionCall{
						Name:      "get_weather",
						Arguments: "{",
					},
				}},
			},
			FinishReason: strPtr("tool_calls"),
		}},
	}

	got := TranslateToAnthropic(resp)
	if got.StopReason == nil || *got.StopReason != "tool_use" {
		t.Fatalf("stop_reason = %#v", got.StopReason)
	}
	if len(got.Content) != 2 {
		t.Fatalf("content len = %d, want 2", len(got.Content))
	}
	if got.Content[0].Type != "text" || got.Content[0].Text != "hello" {
		t.Fatalf("unexpected text block: %#v", got.Content[0])
	}
	if got.Content[1].Type != "tool_use" {
		t.Fatalf("unexpected tool block type: %#v", got.Content[1])
	}
	input, ok := got.Content[1].Input.(map[string]interface{})
	if !ok {
		t.Fatalf("tool input type = %T", got.Content[1].Input)
	}
	if len(input) != 0 {
		t.Fatalf("tool input = %#v, want empty object", input)
	}
}
