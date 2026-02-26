package store

import "testing"

func TestUserStatus(t *testing.T) {
	s := NewStore()
	user, err := s.CreateUser("13900000001", "tester")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Initially not found
	if _, ok := s.GetUserStatus(user.ID); ok {
		t.Fatal("expected no status initially")
	}

	// Save and retrieve
	s.SaveUserStatus(user.ID, "😴", "困了")
	st, ok := s.GetUserStatus(user.ID)
	if !ok {
		t.Fatal("expected status found")
	}
	if st.Emoji != "😴" || st.Text != "困了" {
		t.Fatalf("got %s %s, want 😴 困了", st.Emoji, st.Text)
	}

	// Overwrite
	s.SaveUserStatus(user.ID, "🎧", "听歌")
	st, _ = s.GetUserStatus(user.ID)
	if st.Emoji != "🎧" || st.Text != "听歌" {
		t.Fatalf("got %s %s, want 🎧 听歌", st.Emoji, st.Text)
	}
}

func TestTurnCount(t *testing.T) {
	s := NewStore()
	user, _ := s.CreateUser("13900000002", "tester2")
	bot := s.CreateBot(user.ID, Bot{Name: "test-bot", Gender: "unknown"})

	for i := 1; i <= 10; i++ {
		count := s.IncrementTurnCount(user.ID, bot.ID)
		if count != i {
			t.Fatalf("turn %d: got %d", i, count)
		}
	}

	// Verify via GetBotState
	state, ok := s.GetBotState(user.ID, bot.ID)
	if !ok {
		t.Fatal("expected bot state")
	}
	if state.TurnCount != 10 {
		t.Fatalf("turn count = %d, want 10", state.TurnCount)
	}
}

func TestBotStatusSave(t *testing.T) {
	s := NewStore()
	user, _ := s.CreateUser("13900000003", "tester3")
	bot := s.CreateBot(user.ID, Bot{Name: "status-bot", Gender: "unknown"})

	s.SaveBotStatus(user.ID, bot.ID, "😊", "开心")
	state, ok := s.GetBotState(user.ID, bot.ID)
	if !ok {
		t.Fatal("expected bot state")
	}
	if state.StatusEmoji != "😊" || state.StatusText != "开心" {
		t.Fatalf("got %s %s, want 😊 开心", state.StatusEmoji, state.StatusText)
	}
}
