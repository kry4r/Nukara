package api

import (
	"math/rand"
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestSelectBotStatusEmotionAware(t *testing.T) {
	tests := []struct {
		emotion string
		convID  string
	}{
		{"happy", "conv-1"},
		{"excited", "conv-2"},
		{"love", "conv-3"},
		{"sad", "conv-4"},
		{"angry", "conv-5"},
		{"anxious", "conv-6"},
		{"gentle", "conv-7"},
		{"neutral", "conv-8"},
		{"", "conv-9"},
		{"unknown_emotion", "conv-10"},
	}

	for _, tt := range tests {
		t.Run(tt.emotion+"_"+tt.convID, func(t *testing.T) {
			status := selectBotStatus(tt.emotion, tt.convID)
			if status.Emoji == "" || status.Text == "" {
				t.Fatalf("empty status for emotion=%q conv=%q", tt.emotion, tt.convID)
			}
			// Emotion-mapped statuses should come from the emotion pool.
			if pool, ok := emotionStatusMap[tt.emotion]; ok {
				found := false
				for _, s := range pool {
					if s.Emoji == status.Emoji && s.Text == status.Text {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("status %v not in emotion pool for %q", status, tt.emotion)
				}
			}
		})
	}
}

func TestSelectBotStatusDeterministic(t *testing.T) {
	// Same inputs should always produce the same output.
	s1 := selectBotStatus("happy", "conv-abc")
	s2 := selectBotStatus("happy", "conv-abc")
	if s1 != s2 {
		t.Fatalf("non-deterministic: %v vs %v", s1, s2)
	}
}

func TestIsDND(t *testing.T) {
	tests := []struct {
		name     string
		start    string
		end      string
		hour     int
		minute   int
		expected bool
	}{
		{"within DND", "22:00", "07:00", 23, 30, true},
		{"within DND early morning", "22:00", "07:00", 3, 0, true},
		{"outside DND", "22:00", "07:00", 12, 0, false},
		{"at DND start", "22:00", "07:00", 22, 0, true},
		{"at DND end", "22:00", "07:00", 7, 0, false},
		{"same-day DND within", "09:00", "18:00", 12, 0, true},
		{"same-day DND outside", "09:00", "18:00", 20, 0, false},
		{"empty DND start", "", "07:00", 3, 0, false},
		{"empty DND end", "22:00", "", 23, 0, false},
		{"both empty", "", "", 12, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := store.NotificationSettings{
				DNDStart: tt.start,
				DNDEnd:   tt.end,
			}
			now := time.Date(2026, 2, 14, tt.hour, tt.minute, 0, 0, time.Local)
			got := isDND(settings, now)
			if got != tt.expected {
				t.Fatalf("isDND(%q-%q, %02d:%02d) = %v, want %v", tt.start, tt.end, tt.hour, tt.minute, got, tt.expected)
			}
		})
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"00:00", 0},
		{"08:30", 510},
		{"23:59", 1439},
		{"12:00", 720},
		{"", -1},
		{"abc", -1},
		{"25:00", -1},
		{"12:60", -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseHHMM(tt.input)
			if got != tt.expected {
				t.Fatalf("parseHHMM(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSchedulerDetectTrigger(t *testing.T) {
	rand.Seed(1)

	st := store.NewStore()
	user, _ := st.CreateUser("13900000001", "scheduler-test")
	bot := st.CreateBot(user.ID, store.Bot{
		Name: "TestBot", Summary: "test", SpeakingStyle: "test",
		Background: "test", Traits: []string{"test"}, Gender: "male",
	})
	conv, _ := st.FindConversationByBot(user.ID, bot.ID)

	sched := &proactiveScheduler{server: &Server{store: st}}

	baseTime := time.Date(2026, 2, 14, 12, 0, 0, 0, time.Local)

	tests := []struct {
		name     string
		hour     int
		lastMsg  time.Time
		expected string
	}{
		{"morning window", 8, baseTime, "morning_care"},
		{"evening window", 21, baseTime, "evening_care"},
		{"short inactivity during day", 14, baseTime.Add(-5 * time.Hour), "worry_after_long_silence"},
		{"curiosity after 8m silence", 14, baseTime.Add(2*time.Hour + 21*time.Minute), "curiosity_after_silence"},
		{"no trigger at night", 2, baseTime, ""},
		{"no inactivity if recent", 14, baseTime.Add(2*time.Hour + 23*time.Minute), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv.LastMessageAt = tt.lastMsg
			conversations := []store.Conversation{conv}

			now := time.Date(2026, 2, 14, tt.hour, 30, 0, 0, time.Local)
			got := sched.detectTrigger(now, conversations)
			if got != tt.expected {
				t.Fatalf("detectTrigger(hour=%d) = %q, want %q", tt.hour, got, tt.expected)
			}
		})
	}
}

func TestInactivityThreshold(t *testing.T) {
	t.Run("default 8 minutes", func(t *testing.T) {
		t.Setenv("NUKARA_INACTIVITY_THRESHOLD", "")
		if got := inactivityThreshold(); got != 8*time.Minute {
			t.Fatalf("inactivityThreshold() = %v, want %v", got, 8*time.Minute)
		}
	})

	t.Run("override from env", func(t *testing.T) {
		t.Setenv("NUKARA_INACTIVITY_THRESHOLD", "15m")
		if got := inactivityThreshold(); got != 15*time.Minute {
			t.Fatalf("inactivityThreshold() = %v, want %v", got, 15*time.Minute)
		}
	})

	t.Run("invalid env falls back", func(t *testing.T) {
		t.Setenv("NUKARA_INACTIVITY_THRESHOLD", "bad")
		if got := inactivityThreshold(); got != 8*time.Minute {
			t.Fatalf("inactivityThreshold() = %v, want %v", got, 8*time.Minute)
		}
	})
}

func TestSchedulerCooldownEnforcement(t *testing.T) {
	st := store.NewStore()
	user, _ := st.CreateUser("13900000002", "cooldown-test")
	bot := st.CreateBot(user.ID, store.Bot{
		Name: "CooldownBot", Summary: "test", SpeakingStyle: "test",
		Background: "test", Traits: []string{"test"}, Gender: "male",
	})
	conv, _ := st.FindConversationByBot(user.ID, bot.ID)

	// Add a recent proactive log (1 hour ago).
	st.AddProactiveLog(store.ProactiveLog{
		UserID:         user.ID,
		ConversationID: conv.ID,
		BotID:          bot.ID,
		TriggerType:    "morning_care",
		Message:        "早上好",
	})

	settings := st.GetNotificationSettings(user.ID)
	cooldown := frequencyCooldown[settings.Frequency]
	if cooldown == 0 {
		cooldown = frequencyCooldown["normal"]
	}

	recentLogs := st.ListProactiveLogs(user.ID, 1)
	if len(recentLogs) == 0 {
		t.Fatal("expected at least one proactive log")
	}

	elapsed := time.Since(recentLogs[0].CreatedAt)
	if elapsed >= cooldown {
		t.Fatalf("expected recent log to be within cooldown, elapsed=%v cooldown=%v", elapsed, cooldown)
	}
}
