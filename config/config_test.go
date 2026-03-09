package config

import (
	"testing"

	"github.com/google/uuid"
)

func TestCopilotBaseURLMatchesTargetRepo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		accountType string
		want        string
	}{
		{name: "individual", accountType: "individual", want: "https://api.githubcopilot.com"},
		{name: "business", accountType: "business", want: "https://api.business.githubcopilot.com"},
		{name: "enterprise", accountType: "enterprise", want: "https://api.enterprise.githubcopilot.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := CopilotBaseURL(tc.accountType); got != tc.want {
				t.Fatalf("CopilotBaseURL(%q) = %q, want %q", tc.accountType, got, tc.want)
			}
		})
	}
}

func TestCopilotHeadersMatchTargetRepo(t *testing.T) {
	t.Parallel()

	state := &State{
		CopilotToken:  "copilot-token",
		VSCodeVersion: "1.100.0",
	}

	headers := CopilotHeaders(state, false)
	if got := headers.Get("Authorization"); got != "Bearer copilot-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := headers.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := headers.Get("X-Github-Api-Version"); got != "2025-04-01" {
		t.Fatalf("X-Github-Api-Version = %q", got)
	}
	if got := headers.Get("X-Vscode-User-Agent-Library-Version"); got != "electron-fetch" {
		t.Fatalf("X-Vscode-User-Agent-Library-Version = %q", got)
	}
	if got := headers.Get("X-Request-Id"); got == "" {
		t.Fatal("X-Request-Id is empty")
	} else if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("X-Request-Id is not a UUID: %v", err)
	}
	if got := headers.Get("Accept"); got != "" {
		t.Fatalf("Accept = %q, want empty", got)
	}
	if got := headers.Get("Copilot-Vision-Request"); got != "" {
		t.Fatalf("Copilot-Vision-Request = %q, want empty", got)
	}
	if got := headers.Get("Copilot-Vision-Enabled"); got != "" {
		t.Fatalf("Copilot-Vision-Enabled = %q, want empty", got)
	}
}

func TestCopilotHeadersVisionOnlyAddsRequestHeader(t *testing.T) {
	t.Parallel()

	state := &State{CopilotToken: "copilot-token", VSCodeVersion: "1.100.0"}
	headers := CopilotHeaders(state, true)

	if got := headers.Get("Copilot-Vision-Request"); got != "true" {
		t.Fatalf("Copilot-Vision-Request = %q", got)
	}
	if got := headers.Get("Copilot-Vision-Enabled"); got != "" {
		t.Fatalf("Copilot-Vision-Enabled = %q, want empty", got)
	}
}
