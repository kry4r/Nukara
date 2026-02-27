package api

import (
	"testing"

	"nukara/backend/internal/store"
)

func TestBuildEmotionPrompt(t *testing.T) {
	messages := []string{"今天好累", "加班到很晚", "想休息"}
	prompt := buildEmotionPrompt(messages)

	if !contains(prompt, "1. 今天好累") {
		t.Fatalf("missing numbered message, got: %s", prompt)
	}
	if !contains(prompt, "3. 想休息") {
		t.Fatalf("missing last message, got: %s", prompt)
	}
	if !contains(prompt, "emotion_analysis") {
		t.Fatalf("missing system tag, got: %s", prompt)
	}
}

func TestParseEmotionResult(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		trend string
		tone  string
	}{
		{
			"valid json",
			`{"emotions":["tired","stressed","calm"],"trend":"negative","tone":"疲惫"}`,
			"negative", "疲惫",
		},
		{
			"json in code block",
			"```json\n{\"emotions\":[\"happy\"],\"trend\":\"positive\",\"tone\":\"开心\"}\n```",
			"positive", "开心",
		},
		{
			"invalid json fallback",
			"这不是JSON",
			"neutral", "平静",
		},
		{
			"empty fields default",
			`{"emotions":["neutral"],"trend":"","tone":""}`,
			"neutral", "平静",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEmotionResult(tt.reply)
			if result.Trend != tt.trend {
				t.Fatalf("trend: got %q, want %q", result.Trend, tt.trend)
			}
			if result.Tone != tt.tone {
				t.Fatalf("tone: got %q, want %q", result.Tone, tt.tone)
			}
		})
	}
}

func TestEmotionBufferThreshold(t *testing.T) {
	st := store.NewStore()
	user, _ := st.CreateUser("13900000010", "emotion-test")
	bot := st.CreateBot(user.ID, store.Bot{
		Name: "EmotionBot", Summary: "test", SpeakingStyle: "test",
		Background: "test", Traits: []string{"test"}, Gender: "female",
	})

	// Buffer 4 messages — should not trigger analysis
	for i := 0; i < 4; i++ {
		count := st.AppendEmotionBuffer(user.ID, bot.ID, "message")
		if count != i+1 {
			t.Fatalf("expected count %d, got %d", i+1, count)
		}
	}

	buf := st.GetEmotionBuffer(user.ID, bot.ID)
	if len(buf) != 4 {
		t.Fatalf("expected 4 buffered messages, got %d", len(buf))
	}

	// Clear and verify
	st.ClearEmotionBuffer(user.ID, bot.ID)
	buf = st.GetEmotionBuffer(user.ID, bot.ID)
	if len(buf) != 0 {
		t.Fatalf("expected empty buffer after clear, got %d", len(buf))
	}
}

func TestEmotionContextSaveAndGet(t *testing.T) {
	st := store.NewStore()
	user, _ := st.CreateUser("13900000011", "ctx-test")
	bot := st.CreateBot(user.ID, store.Bot{
		Name: "CtxBot", Summary: "test", SpeakingStyle: "test",
		Background: "test", Traits: []string{"test"}, Gender: "male",
	})

	// No context initially
	_, ok := st.GetEmotionContext(user.ID, bot.ID)
	if ok {
		t.Fatal("expected no emotion context initially")
	}

	// Save and retrieve
	st.SaveEmotionContext(user.ID, bot.ID, store.EmotionContext{
		RecentEmotions: []string{"happy", "excited"},
		EmotionTrend:   "positive",
		LastTone:       "开心",
	})

	ctx, ok := st.GetEmotionContext(user.ID, bot.ID)
	if !ok {
		t.Fatal("expected emotion context after save")
	}
	if ctx.EmotionTrend != "positive" {
		t.Fatalf("trend: got %q, want %q", ctx.EmotionTrend, "positive")
	}
	if len(ctx.RecentEmotions) != 2 {
		t.Fatalf("emotions count: got %d, want 2", len(ctx.RecentEmotions))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
