package instance

import (
	"testing"

	"copilot-go/anthropic"
	"copilot-go/config"
	"copilot-go/store"
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

func TestCalculateAnthropicTokenCountOrDefault_UsesCustomClaudeDisplayIDMapping(t *testing.T) {
	prevAppDir := store.AppDir
	store.AppDir = t.TempDir()
	t.Cleanup(func() {
		store.AppDir = prevAppDir
	})

	if err := store.EnsurePaths(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	if err := store.SetModelMappings([]store.ModelMapping{
		{
			CopilotID: "claude-sonnet-4.6",
			DisplayID: "claude-sonnet-4-6",
		},
		{
			CopilotID: "claude-opus-4.6",
			DisplayID: "claude-opus-4-6",
		},
	}); err != nil {
		t.Fatalf("set model mappings: %v", err)
	}
	models := &config.ModelsResponse{
		Object: "list",
		Data: []config.ModelEntry{
			{
				ID: "claude-sonnet-4.6",
				Capabilities: config.ModelCapabilities{
					Tokenizer: "o200k_base",
				},
			},
			{
				ID: "claude-opus-4.6",
				Capabilities: config.ModelCapabilities{
					Tokenizer: "o200k_base",
				},
			},
		},
	}

	tests := []struct {
		name  string
		model string
	}{
		{name: "sonnet display id", model: "claude-sonnet-4-6"},
		{name: "opus display id", model: "claude-opus-4-6"},
	}

	for _, tt := range tests {
		payload := anthropic.AnthropicMessagesPayload{
			Model:     tt.model,
			Messages:  []anthropic.AnthropicMessage{{Role: "user", Content: "hello from mapping"}},
			MaxTokens: 128,
		}

		if got := calculateAnthropicTokenCountOrDefault(payload, "", models); got <= 1 {
			t.Fatalf("%s: token count = %d, want > 1", tt.name, got)
		}
	}
}
