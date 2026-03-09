package anthropic

import "testing"

func TestTranslateToOpenAI_NormalizesAnthropicModelAndMapsMetadata(t *testing.T) {
	t.Parallel()

	payload := AnthropicMessagesPayload{
		Model: "claude-sonnet-4-20250514",
		System: []SystemBlock{
			{Type: "text", Text: "line one"},
			{Type: "text", Text: "line two"},
		},
		Messages:  []AnthropicMessage{{Role: "user", Content: "Hello!"}},
		MaxTokens: 100,
		Metadata:  &Metadata{UserID: "user-123"},
		ToolChoice: map[string]interface{}{
			"type": "tool",
			"name": "get_weather",
		},
	}

	got := TranslateToOpenAI(payload)

	if got.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want claude-sonnet-4", got.Model)
	}
	if got.User != "user-123" {
		t.Fatalf("user = %q, want user-123", got.User)
	}
	if len(got.Messages) == 0 || got.Messages[0].Role != "system" {
		t.Fatalf("expected first message to be system: %#v", got.Messages)
	}
	if got.Messages[0].Content != "line one\n\nline two" {
		t.Fatalf("system content = %#v", got.Messages[0].Content)
	}

	toolChoice, ok := got.ToolChoice.(map[string]interface{})
	if !ok {
		t.Fatalf("tool_choice type = %T", got.ToolChoice)
	}
	if toolChoice["type"] != "function" {
		t.Fatalf("tool_choice.type = %#v", toolChoice["type"])
	}
	fn, ok := toolChoice["function"].(map[string]string)
	if !ok {
		t.Fatalf("tool_choice.function type = %T", toolChoice["function"])
	}
	if fn["name"] != "get_weather" {
		t.Fatalf("tool_choice.function.name = %q", fn["name"])
	}
}

func TestTranslateToOpenAI_PlacesToolResultsBeforeAggregatedUserContent(t *testing.T) {
	t.Parallel()

	payload := AnthropicMessagesPayload{
		Model: "claude-opus-4-20250514",
		Messages: []AnthropicMessage{{
			Role: "user",
			Content: []ContentBlock{
				{Type: "tool_result", ToolUseID: "call_1", Content2: "tool output"},
				{Type: "text", Text: "follow up question"},
				{Type: "image", Source: &ImageSource{Type: "base64", MediaType: "image/png", Data: "Zm9v"}},
			},
		}},
	}

	got := TranslateToOpenAI(payload)
	if got.Model != "claude-opus-4" {
		t.Fatalf("model = %q, want claude-opus-4", got.Model)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != "tool" || got.Messages[0].ToolCallID != "call_1" || got.Messages[0].Content != "tool output" {
		t.Fatalf("unexpected tool message: %#v", got.Messages[0])
	}
	if got.Messages[1].Role != "user" {
		t.Fatalf("unexpected user message role: %#v", got.Messages[1])
	}
	parts, ok := got.Messages[1].Content.([]OpenAIContentPart)
	if !ok {
		t.Fatalf("user content type = %T", got.Messages[1].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("user content parts len = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "follow up question" {
		t.Fatalf("unexpected text part: %#v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,Zm9v" {
		t.Fatalf("unexpected image part: %#v", parts[1])
	}
}

func TestTranslateToOpenAI_AssistantThinkingIsMergedWithTextAndToolCalls(t *testing.T) {
	t.Parallel()

	payload := AnthropicMessagesPayload{
		Model: "claude-sonnet-4-20250514",
		Messages: []AnthropicMessage{{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "thinking", Thinking: "think first"},
				{Type: "text", Text: "then answer"},
				{Type: "tool_use", ID: "call_1", Name: "get_weather", Input: map[string]interface{}{"city": "Paris"}},
			},
		}},
	}

	got := TranslateToOpenAI(payload)
	if len(got.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(got.Messages))
	}
	msg := got.Messages[0]
	if msg.Role != "assistant" {
		t.Fatalf("role = %q", msg.Role)
	}
	if msg.Content != "then answer\n\nthink first" {
		t.Fatalf("content = %#v", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Arguments != `{"city":"Paris"}` {
		t.Fatalf("tool args = %q", msg.ToolCalls[0].Function.Arguments)
	}
}
