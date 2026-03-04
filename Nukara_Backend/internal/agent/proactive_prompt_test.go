package agent

import (
	"strings"
	"testing"
)

func TestProactivePromptShareTriggers(t *testing.T) {
	tests := []struct {
		name    string
		trigger string
		want    string
	}{
		{
			name:    "share personal moment has topic finding hint",
			trigger: "share_personal_moment",
			want:    "找话题开启聊天",
		},
		{
			name:    "share interesting fact has topic finding hint",
			trigger: "share_interesting_fact",
			want:    "找共同话题",
		},
		{
			name:    "share emotion has open ending hint",
			trigger: "share_emotion",
			want:    "开放式小尾巴",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proactivePrompt(tt.trigger, map[string]any{})
			if !strings.Contains(got, tt.want) {
				t.Fatalf("proactivePrompt(%q)=%q, want contains %q", tt.trigger, got, tt.want)
			}
		})
	}
}

func TestProactivePromptIncludesEmotionHint(t *testing.T) {
	got := proactivePrompt("morning_care", map[string]any{"emotion_trend": "anxious"})
	if !strings.Contains(got, "用户最近情绪倾向：anxious") {
		t.Fatalf("prompt missing emotion hint: %q", got)
	}
}

func TestProactivePromptContainsNoGuiltTrippingRule(t *testing.T) {
	got := proactivePrompt("worry_after_long_silence", map[string]any{
		"last_user_message":            "我今天有点累",
		"time_since_last_user_message": "3h0m0s",
	})
	if !strings.Contains(got, "你怎么不理我") {
		t.Fatalf("prompt should include anti guilt-tripping rule, got=%q", got)
	}
	if !strings.Contains(got, "用户上次消息") {
		t.Fatalf("prompt should include last user message context, got=%q", got)
	}
}

func TestProactivePromptFallback(t *testing.T) {
	got := proactivePrompt("unknown_trigger", map[string]any{})
	if !strings.Contains(got, "自然的主动关怀消息") {
		t.Fatalf("fallback prompt mismatch: %q", got)
	}
}
