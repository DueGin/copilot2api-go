package anthropic

import "encoding/json"

// jsonUnmarshal is a package-level alias to avoid import in translate_response.go
var jsonUnmarshal = json.Unmarshal

// TranslateChunkToAnthropicEvents converts an OpenAI stream chunk to Anthropic SSE events.
func TranslateChunkToAnthropicEvents(chunk ChatCompletionResponse, state *AnthropicStreamState) []StreamEvent {
	events := make([]StreamEvent, 0)
	if len(chunk.Choices) == 0 {
		return events
	}

	if chunk.Model != "" {
		state.Model = chunk.Model
	}
	if chunk.ID != "" {
		state.ID = chunk.ID
	}
	if chunk.Usage != nil {
		state.InputTokens = promptInputTokens(chunk.Usage)
		state.OutputTokens = completionTokens(chunk.Usage)
		state.CacheReadInputTokens = cachedPromptTokens(chunk.Usage)
	}

	choice := chunk.Choices[0]
	delta := choice.Delta
	if delta == nil {
		delta = &ChoiceMsg{}
	}

	if !state.MessageStartSent {
		events = append(events, StreamEvent{
			Event: "message_start",
			Data: MessageStartEvent{
				Type: "message_start",
				Message: AnthropicResponse{
					ID:           state.ID,
					Type:         "message",
					Role:         "assistant",
					Content:      []AnthropicContentBlock{},
					Model:        state.Model,
					StopReason:   nil,
					StopSequence: nil,
					Usage: AnthropicUsage{
						InputTokens:          state.InputTokens,
						OutputTokens:         0,
						CacheReadInputTokens: state.CacheReadInputTokens,
					},
				},
			},
		})
		state.MessageStartSent = true
	}

	if delta.Content != "" {
		if isToolBlockOpen(state) {
			events = append(events, stopContentBlock(state, true)...)
		}
		if !state.ContentBlockOpen {
			events = append(events, StreamEvent{
				Event: "content_block_start",
				Data: ContentBlockStartEvent{
					Type:  "content_block_start",
					Index: state.ContentBlockIndex,
					ContentBlock: StreamContentBlock{
						Type: "text",
						Text: emptyStringPtr(""),
					},
				},
			})
			state.ContentBlockOpen = true
		}
		events = append(events, StreamEvent{
			Event: "content_block_delta",
			Data: ContentBlockDeltaEvent{
				Type:  "content_block_delta",
				Index: state.ContentBlockIndex,
				Delta: DeltaBlock{Type: "text_delta", Text: delta.Content},
			},
		})
	}

	if len(delta.ToolCalls) > 0 {
		for _, toolCall := range delta.ToolCalls {
			idx := 0
			if toolCall.Index != nil {
				idx = *toolCall.Index
			}
			if toolCall.ID != "" && toolCall.Function.Name != "" {
				if state.ContentBlockOpen {
					events = append(events, stopContentBlock(state, true)...)
				}
				anthropicBlockIndex := state.ContentBlockIndex
				state.ToolCalls[idx] = &ToolCallState{
					ID:                  toolCall.ID,
					Name:                toolCall.Function.Name,
					AnthropicBlockIndex: anthropicBlockIndex,
				}
				events = append(events, StreamEvent{
					Event: "content_block_start",
					Data: ContentBlockStartEvent{
						Type:  "content_block_start",
						Index: anthropicBlockIndex,
						ContentBlock: StreamContentBlock{
							Type:  "tool_use",
							ID:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Input: map[string]interface{}{},
						},
					},
				})
				state.ContentBlockOpen = true
			}
			if toolCall.Function.Arguments != "" {
				if toolCallInfo := state.ToolCalls[idx]; toolCallInfo != nil {
					toolCallInfo.Arguments += toolCall.Function.Arguments
					events = append(events, StreamEvent{
						Event: "content_block_delta",
						Data: ContentBlockDeltaEvent{
							Type:  "content_block_delta",
							Index: toolCallInfo.AnthropicBlockIndex,
							Delta: DeltaBlock{Type: "input_json_delta", PartialJSON: toolCall.Function.Arguments},
						},
					})
				}
			}
		}
	}

	if choice.FinishReason != nil {
		if state.ContentBlockOpen {
			events = append(events, stopContentBlock(state, false)...)
		}
		events = append(events,
			StreamEvent{
				Event: "message_delta",
				Data: MessageDeltaEvent{
					Type: "message_delta",
					Delta: MessageDelta{
						StopReason:   mapOpenAIStopReasonToAnthropicPtr(choice.FinishReason),
						StopSequence: nil,
					},
					Usage: &DeltaUsage{
						InputTokens:          state.InputTokens,
						OutputTokens:         state.OutputTokens,
						CacheReadInputTokens: state.CacheReadInputTokens,
					},
				},
			},
			StreamEvent{Event: "message_stop", Data: map[string]string{"type": "message_stop"}},
		)
	}

	return events
}

func emptyStringPtr(s string) *string {
	return &s
}

func isToolBlockOpen(state *AnthropicStreamState) bool {
	for _, toolCall := range state.ToolCalls {
		if toolCall != nil && toolCall.AnthropicBlockIndex == state.ContentBlockIndex {
			return true
		}
	}
	return false
}

func stopContentBlock(state *AnthropicStreamState, increment bool) []StreamEvent {
	if !state.ContentBlockOpen {
		return nil
	}
	index := state.ContentBlockIndex
	state.ContentBlockOpen = false
	if increment {
		state.ContentBlockIndex++
	}
	return []StreamEvent{{
		Event: "content_block_stop",
		Data: ContentBlockStopEvent{
			Type:  "content_block_stop",
			Index: index,
		},
	}}
}
