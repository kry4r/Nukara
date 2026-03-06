package admin

import "testing"

func TestNormalizeProviderAPIMode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "defaults empty to chat completions", input: "", want: "chat_completions"},
		{name: "accepts chat completions", input: "chat_completions", want: "chat_completions"},
		{name: "accepts responses", input: "responses", want: "responses"},
		{name: "accepts auto", input: "auto", want: "auto"},
		{name: "normalizes legacy completion alias", input: "completion", want: "chat_completions"},
		{name: "trims and lowercases", input: " Responses ", want: "responses"},
		{name: "falls back for unknown values", input: "something-else", want: "chat_completions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeProviderAPIMode(tt.input); got != tt.want {
				t.Fatalf("normalizeProviderAPIMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
