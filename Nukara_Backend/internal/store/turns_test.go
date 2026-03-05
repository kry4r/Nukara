package store

import "testing"

func TestTurnAndMemoryPersistence(t *testing.T) {
	s := NewStore()

	turn, err := s.CreateTurn(AgentTurn{
		UserID:             "u1",
		BotID:              "b1",
		ConversationID:     "c1",
		UserMessageIDs:     []string{"m1", "m2"},
		AggregatedUserText: "hello there",
		BotReplyText:       "hi",
	})
	if err != nil {
		t.Fatalf("CreateTurn failed: %v", err)
	}
	if turn.ID == "" {
		t.Fatalf("turn id should not be empty")
	}

	if err := s.UpsertCompact("c1", `{"summary":"compact"}`, turn.ID); err != nil {
		t.Fatalf("UpsertCompact failed: %v", err)
	}
	compact, ok := s.GetConversationCompact("c1")
	if !ok {
		t.Fatalf("compact not found")
	}
	if compact.UntilTurnID != turn.ID {
		t.Fatalf("compact until turn id = %s, want %s", compact.UntilTurnID, turn.ID)
	}

	item, err := s.UpsertMemoryItem(MemoryItem{
		UserID:     "u1",
		BotID:      "b1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "user likes tea",
		Importance: 80,
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("UpsertMemoryItem failed: %v", err)
	}
	if item.ID == "" {
		t.Fatalf("memory item id should not be empty")
	}
	got, ok := s.GetMemoryItem(item.ID)
	if !ok {
		t.Fatalf("memory item not found")
	}
	if got.Owner != "user" {
		t.Fatalf("memory owner = %s, want user", got.Owner)
	}
}

func TestTurnProviderSettings(t *testing.T) {
	s := NewStore()

	if err := s.SetSystemSetting("default_chat_provider_id", "minimax_m2_5"); err != nil {
		t.Fatalf("SetSystemSetting failed: %v", err)
	}
	if got, ok := s.GetSystemSetting("default_chat_provider_id"); !ok || got != "minimax_m2_5" {
		t.Fatalf("GetSystemSetting got=(%q,%v), want (minimax_m2_5,true)", got, ok)
	}

	if err := s.SetUserProviderSetting("u1", "provider-user", "model-user"); err != nil {
		t.Fatalf("SetUserProviderSetting failed: %v", err)
	}
	if providerID, model, ok := s.GetUserProviderSetting("u1"); !ok || providerID != "provider-user" || model != "model-user" {
		t.Fatalf("GetUserProviderSetting got=(%s,%s,%v)", providerID, model, ok)
	}

	if err := s.SetBotProviderOverride("u1", "b1", "provider-bot", "model-bot"); err != nil {
		t.Fatalf("SetBotProviderOverride failed: %v", err)
	}
	if providerID, model, ok := s.GetBotProviderOverride("u1", "b1"); !ok || providerID != "provider-bot" || model != "model-bot" {
		t.Fatalf("GetBotProviderOverride got=(%s,%s,%v)", providerID, model, ok)
	}
}
