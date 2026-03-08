package store

import (
	"reflect"
	"testing"
)

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

func TestBotPersonaV2Fields(t *testing.T) {
	s := NewStore()
	user, err := s.CreateUser("13900000004", "persona-v2")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	created := s.CreateBot(user.ID, Bot{
		Name:                 "苏子衿",
		Identity:             "你的恋人，也是会认真接住你情绪的人",
		Personality:          []string{"细腻", "敏锐"},
		ExpressionStyle:      "口语化，短句，会接梗",
		LifeContext:          "现在住在东京，平时摄影、通勤、喝便利店咖啡",
		TaboosAndPreferences: "不喜欢被命令式对待，更喜欢被温柔回应",
		ChatBackgroundStyle:  "lightPaper",
	})

	got, ok := s.GetBot(user.ID, created.ID)
	if !ok {
		t.Fatal("expected bot found")
	}
	if got.Identity != "你的恋人，也是会认真接住你情绪的人" {
		t.Fatalf("identity = %q", got.Identity)
	}
	if !reflect.DeepEqual(got.Personality, []string{"细腻", "敏锐"}) {
		t.Fatalf("personality = %#v", got.Personality)
	}
	if got.ExpressionStyle != "口语化，短句，会接梗" {
		t.Fatalf("expression_style = %q", got.ExpressionStyle)
	}
	if got.LifeContext != "现在住在东京，平时摄影、通勤、喝便利店咖啡" {
		t.Fatalf("life_context = %q", got.LifeContext)
	}
	if got.TaboosAndPreferences != "不喜欢被命令式对待，更喜欢被温柔回应" {
		t.Fatalf("taboos_and_preferences = %q", got.TaboosAndPreferences)
	}
	if got.Summary != got.Identity {
		t.Fatalf("summary should sync from identity, got %q want %q", got.Summary, got.Identity)
	}
	if got.Background != got.LifeContext {
		t.Fatalf("background should sync from life_context, got %q want %q", got.Background, got.LifeContext)
	}
	if got.SpeakingStyle != got.ExpressionStyle {
		t.Fatalf("speaking_style should sync from expression_style, got %q want %q", got.SpeakingStyle, got.ExpressionStyle)
	}
}

func TestNotificationSettingsUsesExplicitMinutes(t *testing.T) {
	s := NewStore()
	user, err := s.CreateUser("13900000005", "notif-minutes")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	saved := s.UpdateNotificationSettings(user.ID, NotificationSettings{
		ProactiveEnabled:         true,
		ProactiveIntervalMinutes: 10,
		DNDStart:                 "23:00",
		DNDEnd:                   "08:00",
	})
	if saved.ProactiveIntervalMinutes != 10 {
		t.Fatalf("proactive_interval_minutes = %d", saved.ProactiveIntervalMinutes)
	}

	got := s.GetNotificationSettings(user.ID)
	if got.ProactiveIntervalMinutes != 10 {
		t.Fatalf("stored proactive_interval_minutes = %d", got.ProactiveIntervalMinutes)
	}
	if got.DNDStart != "23:00" || got.DNDEnd != "08:00" {
		t.Fatalf("unexpected dnd window: %+v", got)
	}
}

func TestNotificationSettingsBackfillsLegacyFrequency(t *testing.T) {
	s := NewStore()
	user, err := s.CreateUser("13900000006", "notif-legacy")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	s.notifByUser[user.ID] = NotificationSettings{
		UserID:           user.ID,
		ProactiveEnabled: true,
		Frequency:        "high",
	}

	got := s.GetNotificationSettings(user.ID)
	if got.ProactiveIntervalMinutes != 120 {
		t.Fatalf("legacy high should map to 120 minutes, got %d", got.ProactiveIntervalMinutes)
	}
}

func TestBotPersonaV2Backfill(t *testing.T) {
	legacy := Bot{
		Summary:       "你的恋人，也是会认真接住你情绪的人",
		SpeakingStyle: "口语化，短句，会接梗",
		Background:    "现在住在东京，平时摄影、通勤、喝便利店咖啡",
		Traits:        []string{"细腻", "敏锐"},
		SelfCognition: []string{"不喜欢被命令式对待", "更喜欢被温柔回应"},
	}

	derived := DerivePersonaV2FromLegacy(legacy)
	if derived.Identity != legacy.Summary {
		t.Fatalf("identity = %q want %q", derived.Identity, legacy.Summary)
	}
	if !reflect.DeepEqual(derived.Personality, legacy.Traits) {
		t.Fatalf("personality = %#v want %#v", derived.Personality, legacy.Traits)
	}
	if derived.ExpressionStyle != legacy.SpeakingStyle {
		t.Fatalf("expression_style = %q want %q", derived.ExpressionStyle, legacy.SpeakingStyle)
	}
	if derived.LifeContext != legacy.Background {
		t.Fatalf("life_context = %q want %q", derived.LifeContext, legacy.Background)
	}
	if derived.TaboosAndPreferences != "" {
		t.Fatalf("taboos_and_preferences should stay independent, got %q", derived.TaboosAndPreferences)
	}

	preserved := DerivePersonaV2FromLegacy(Bot{
		Summary:              "旧摘要",
		SpeakingStyle:        "旧风格",
		Background:           "旧背景",
		Traits:               []string{"旧特质"},
		SelfCognition:        []string{"旧禁忌"},
		Identity:             "已存在身份",
		Personality:          []string{"冷静"},
		ExpressionStyle:      "克制",
		LifeContext:          "住在大阪",
		TaboosAndPreferences: "讨厌敷衍",
	})
	if preserved.Identity != "已存在身份" || preserved.ExpressionStyle != "克制" || preserved.LifeContext != "住在大阪" || preserved.TaboosAndPreferences != "讨厌敷衍" {
		t.Fatalf("existing v2 fields should be preserved: %+v", preserved)
	}
	if !reflect.DeepEqual(preserved.Personality, []string{"冷静"}) {
		t.Fatalf("existing personality should be preserved: %#v", preserved.Personality)
	}

	synced := SyncLegacyPersonaFields(Bot{
		Identity:             "你的恋人，也是会认真接住你情绪的人",
		Personality:          []string{"细腻", "敏锐"},
		ExpressionStyle:      "口语化，短句，会接梗",
		LifeContext:          "现在住在东京，平时摄影、通勤、喝便利店咖啡",
		TaboosAndPreferences: "不喜欢被命令式对待，更喜欢被温柔回应",
	})
	if synced.Summary != synced.Identity {
		t.Fatalf("summary = %q want %q", synced.Summary, synced.Identity)
	}
	if synced.Relationship != synced.Identity {
		t.Fatalf("relationship = %q want %q", synced.Relationship, synced.Identity)
	}
	if synced.SpeakingStyle != synced.ExpressionStyle {
		t.Fatalf("speaking_style = %q want %q", synced.SpeakingStyle, synced.ExpressionStyle)
	}
	if synced.Background != synced.LifeContext || synced.Role != synced.LifeContext {
		t.Fatalf("background/role not synced: background=%q role=%q", synced.Background, synced.Role)
	}
	if !reflect.DeepEqual(synced.Traits, synced.Personality) {
		t.Fatalf("traits = %#v want %#v", synced.Traits, synced.Personality)
	}
	if len(synced.SelfCognition) != 0 {
		t.Fatalf("self_cognition should stay independent, got %#v", synced.SelfCognition)
	}
}

func TestUpdateBotSelfCognitionDoesNotOverwriteStaticTaboos(t *testing.T) {
	s := NewStore()
	bot := s.CreateBot("user-1", Bot{
		Name:                 "bot",
		TaboosAndPreferences: "不喜欢被命令式对待",
	})

	updated, ok := s.UpdateBot("user-1", bot.ID, Bot{
		SelfCognition: []string{"我最近会在下班后散步，让自己慢慢冷静下来。"},
	})
	if !ok {
		t.Fatal("expected update to succeed")
	}
	if updated.TaboosAndPreferences != "不喜欢被命令式对待" {
		t.Fatalf("taboos_and_preferences = %q", updated.TaboosAndPreferences)
	}
	if !reflect.DeepEqual(updated.SelfCognition, []string{"我最近会在下班后散步，让自己慢慢冷静下来。"}) {
		t.Fatalf("self_cognition = %#v", updated.SelfCognition)
	}
}

func TestNewStoreSeedsStableDevUser(t *testing.T) {
	s := NewStore()
	user, ok := s.FindUserByEmail("tester@nukara.local")
	if !ok {
		t.Fatal("expected seeded dev user")
	}
	if user.ID != "dev-user-tester" {
		t.Fatalf("seeded dev user id = %q, want %q", user.ID, "dev-user-tester")
	}
}
