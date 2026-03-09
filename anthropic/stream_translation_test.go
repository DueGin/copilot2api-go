package anthropic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateChunkToAnthropicEvents_TextStreamingMatchesTargetOrder(t *testing.T) {
	t.Parallel()

	state := NewStreamState()
	roleChunk := ChatCompletionResponse{
		ID:    "cmpl-1",
		Model: "gpt-4.1",
		Choices: []Choice{{
			Index: 0,
			Delta: &ChoiceMsg{Role: "assistant"},
		}},
	}
	contentChunk := ChatCompletionResponse{
		ID:    "cmpl-1",
		Model: "gpt-4.1",
		Choices: []Choice{{
			Index: 0,
			Delta: &ChoiceMsg{Content: "Hello"},
		}},
	}
	finishChunk := ChatCompletionResponse{
		ID:    "cmpl-1",
		Model: "gpt-4.1",
		Choices: []Choice{{
			Index:        0,
			Delta:        &ChoiceMsg{},
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

	var events []StreamEvent
	events = append(events, TranslateChunkToAnthropicEvents(roleChunk, state)...)
	events = append(events, TranslateChunkToAnthropicEvents(contentChunk, state)...)
	events = append(events, TranslateChunkToAnthropicEvents(finishChunk, state)...)

	if len(events) != 6 {
		t.Fatalf("events len = %d, want 6", len(events))
	}
	wantTypes := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	for i, want := range wantTypes {
		if events[i].Event != want {
			t.Fatalf("event[%d] = %q, want %q", i, events[i].Event, want)
		}
	}

	start, ok := events[0].Data.(MessageStartEvent)
	if !ok {
		t.Fatalf("message_start data type = %T", events[0].Data)
	}
	if start.Message.StopReason != nil {
		t.Fatalf("message_start stop_reason = %#v, want nil", start.Message.StopReason)
	}
	if start.Message.StopSequence != nil {
		t.Fatalf("message_start stop_sequence = %#v, want nil", start.Message.StopSequence)
	}

	delta, ok := events[4].Data.(MessageDeltaEvent)
	if !ok {
		t.Fatalf("message_delta data type = %T", events[4].Data)
	}
	if delta.Delta.StopReason == nil || *delta.Delta.StopReason != "end_turn" {
		t.Fatalf("message_delta stop_reason = %#v", delta.Delta.StopReason)
	}
	if delta.Usage == nil || delta.Usage.InputTokens != 7 || delta.Usage.OutputTokens != 2 || delta.Usage.CacheReadInputTokens != 3 {
		t.Fatalf("message_delta usage = %#v", delta.Usage)
	}
}

func TestTranslateChunkToAnthropicEvents_ClosesToolBlockBeforeOpeningTextBlock(t *testing.T) {
	t.Parallel()

	state := NewStreamState()
	idx := 0
	chunks := []ChatCompletionResponse{
		{
			ID:      "cmpl-2",
			Model:   "gpt-4.1",
			Choices: []Choice{{Index: 0, Delta: &ChoiceMsg{Role: "assistant"}}},
		},
		{
			ID:    "cmpl-2",
			Model: "gpt-4.1",
			Choices: []Choice{{
				Index: 0,
				Delta: &ChoiceMsg{ToolCalls: []ToolCall{{
					Index: &idx,
					ID:    "call_1",
					Type:  "function",
					Function: FunctionCall{
						Name:      "get_weather",
						Arguments: "",
					},
				}}},
			}},
		},
		{
			ID:    "cmpl-2",
			Model: "gpt-4.1",
			Choices: []Choice{{
				Index: 0,
				Delta: &ChoiceMsg{ToolCalls: []ToolCall{{
					Index: &idx,
					Function: FunctionCall{
						Arguments: `{"city":"Paris"}`,
					},
				}}},
			}},
		},
		{
			ID:      "cmpl-2",
			Model:   "gpt-4.1",
			Choices: []Choice{{Index: 0, Delta: &ChoiceMsg{Content: "done"}}},
		},
		{
			ID:      "cmpl-2",
			Model:   "gpt-4.1",
			Choices: []Choice{{Index: 0, Delta: &ChoiceMsg{}, FinishReason: strPtr("tool_calls")}},
		},
	}

	var events []StreamEvent
	for _, chunk := range chunks {
		events = append(events, TranslateChunkToAnthropicEvents(chunk, state)...)
	}

	wantTypes := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("events len = %d, want %d", len(events), len(wantTypes))
	}
	for i, want := range wantTypes {
		if events[i].Event != want {
			t.Fatalf("event[%d] = %q, want %q", i, events[i].Event, want)
		}
	}

	toolStart := events[1].Data.(ContentBlockStartEvent)
	if toolStart.Index != 0 || toolStart.ContentBlock.Type != "tool_use" {
		t.Fatalf("unexpected tool block start: %#v", toolStart)
	}
	textStart := events[4].Data.(ContentBlockStartEvent)
	if textStart.Index != 1 || textStart.ContentBlock.Type != "text" {
		t.Fatalf("unexpected text block start: %#v", textStart)
	}
	messageDelta := events[7].Data.(MessageDeltaEvent)
	if messageDelta.Delta.StopReason == nil || *messageDelta.Delta.StopReason != "tool_use" {
		t.Fatalf("unexpected final stop_reason: %#v", messageDelta.Delta.StopReason)
	}
}

func TestTranslateChunkToAnthropicEvents_StreamJSONKeepsEmptyTextAndNullStopSequence(t *testing.T) {
	t.Parallel()

	state := NewStreamState()
	chunks := []ChatCompletionResponse{
		{ID: "cmpl-json", Object: "chat.completion.chunk", Created: 0, Model: "gpt-4.1", Choices: []Choice{{Index: 0, Delta: &ChoiceMsg{Role: "assistant"}}}},
		{ID: "cmpl-json", Object: "chat.completion.chunk", Created: 0, Model: "gpt-4.1", Choices: []Choice{{Index: 0, Delta: &ChoiceMsg{Content: "Hello"}}}},
		{ID: "cmpl-json", Object: "chat.completion.chunk", Created: 0, Model: "gpt-4.1", Choices: []Choice{{Index: 0, Delta: &ChoiceMsg{}, FinishReason: strPtr("stop")}}},
	}

	var events []StreamEvent
	for _, chunk := range chunks {
		events = append(events, TranslateChunkToAnthropicEvents(chunk, state)...)
	}

	contentStartJSON, err := json.Marshal(events[1].Data)
	if err != nil {
		t.Fatalf("marshal content_block_start: %v", err)
	}
	if !strings.Contains(string(contentStartJSON), "\"text\":\"\"") {
		t.Fatalf("content_block_start JSON = %s, want empty text field", string(contentStartJSON))
	}

	messageDeltaJSON, err := json.Marshal(events[4].Data)
	if err != nil {
		t.Fatalf("marshal message_delta: %v", err)
	}
	if !strings.Contains(string(messageDeltaJSON), "\"stop_sequence\":null") {
		t.Fatalf("message_delta JSON = %s, want stop_sequence null", string(messageDeltaJSON))
	}
}
