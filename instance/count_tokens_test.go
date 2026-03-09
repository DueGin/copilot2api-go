package instance

import (
	"testing"

	"copilot-go/anthropic"
	"copilot-go/config"
)

func TestCalculateAnthropicTokenCountOrDefault_ModelMissingFallsBackToOne(t *testing.T) {
	t.Parallel()

	payload := anthropic.AnthropicMessagesPayload{
		Model:     "claude-sonnet-4",
		Messages:  []anthropic.AnthropicMessage{{Role: "user", Content: "hello"}},
		MaxTokens: 128,
	}

	if got := calculateAnthropicTokenCountOrDefault(payload, "", nil); got != 1 {
		t.Fatalf("token count = %d, want 1", got)
	}
}

func TestApplyAnthropicTokenAdjustments_ClaudeAndGrok(t *testing.T) {
	t.Parallel()

	claudePayload := anthropic.AnthropicMessagesPayload{
		Model: "claude-sonnet-4",
		Tools: []anthropic.AnthropicTool{{Name: "get_weather", InputSchema: map[string]interface{}{"type": "object"}}},
	}
	if got := applyAnthropicTokenAdjustments(100, claudePayload, ""); got != 513 {
		t.Fatalf("claude adjusted tokens = %d, want 513", got)
	}

	grokPayload := anthropic.AnthropicMessagesPayload{
		Model: "grok-3",
		Tools: []anthropic.AnthropicTool{{Name: "get_weather", InputSchema: map[string]interface{}{"type": "object"}}},
	}
	if got := applyAnthropicTokenAdjustments(100, grokPayload, ""); got != 597 {
		t.Fatalf("grok adjusted tokens = %d, want 597", got)
	}
}

func TestApplyAnthropicTokenAdjustments_ClaudeCodeMCPToolSkipsFixedOverhead(t *testing.T) {
	t.Parallel()

	payload := anthropic.AnthropicMessagesPayload{
		Model: "claude-sonnet-4",
		Tools: []anthropic.AnthropicTool{{Name: "mcp__weather", InputSchema: map[string]interface{}{"type": "object"}}},
	}

	if got := applyAnthropicTokenAdjustments(100, payload, "claude-code-20250219"); got != 115 {
		t.Fatalf("claude-code MCP adjusted tokens = %d, want 115", got)
	}
}

func TestCalculateAnthropicTokenCountOrDefault_UnknownTokenizerFallsBackToO200kBase(t *testing.T) {
	t.Parallel()

	payload := anthropic.AnthropicMessagesPayload{
		Model:     "claude-sonnet-4",
		Messages:  []anthropic.AnthropicMessage{{Role: "user", Content: "hello from fallback"}},
		MaxTokens: 128,
	}
	models := &config.ModelsResponse{
		Object: "list",
		Data: []config.ModelEntry{{
			ID: "claude-sonnet-4",
			Capabilities: config.ModelCapabilities{
				Tokenizer: "unknown_encoding",
			},
		}},
	}

	if got := calculateAnthropicTokenCountOrDefault(payload, "", models); got <= 1 {
		t.Fatalf("token count = %d, want > 1", got)
	}
}
