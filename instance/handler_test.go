package instance

import (
	"testing"

	"copilot-go/anthropic"
	"copilot-go/config"
)

func TestInitiatorForMessages(t *testing.T) {
	t.Parallel()

	if got := initiatorForMessages([]anthropic.OpenAIMessage{{Role: "user", Content: "hi"}}); got != "user" {
		t.Fatalf("initiator = %q, want user", got)
	}
	if got := initiatorForMessages([]anthropic.OpenAIMessage{{Role: "assistant", Content: "hi"}}); got != "agent" {
		t.Fatalf("initiator = %q, want agent", got)
	}
	if got := initiatorForMessages([]anthropic.OpenAIMessage{{Role: "tool", Content: "tool output"}}); got != "agent" {
		t.Fatalf("initiator = %q, want agent", got)
	}
}

func TestApplyDefaultMaxTokensUsesCachedModelLimits(t *testing.T) {
	t.Parallel()

	payload := &anthropic.ChatCompletionsPayload{
		Model:    "gpt-4.1",
		Messages: []anthropic.OpenAIMessage{{Role: "user", Content: "hi"}},
	}
	models := &config.ModelsResponse{
		Object: "list",
		Data: []config.ModelEntry{{
			ID: "gpt-4.1",
			Capabilities: config.ModelCapabilities{
				Limits: config.ModelLimits{MaxOutputTokens: 8192},
			},
		}},
	}

	applyDefaultMaxTokens(payload, models)
	if payload.MaxTokens != 8192 {
		t.Fatalf("max_tokens = %d, want 8192", payload.MaxTokens)
	}

	payload.MaxTokens = 512
	applyDefaultMaxTokens(payload, models)
	if payload.MaxTokens != 512 {
		t.Fatalf("max_tokens overwritten = %d, want 512", payload.MaxTokens)
	}
}
