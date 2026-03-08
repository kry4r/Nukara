package subtasks

import (
	"context"
	"strings"
	"testing"

	"nukara/backend/internal/store"
)

type testStore struct {
	memoryItems    []store.MemoryItem
	compacts       []store.ConversationCompact
	personaCalls   []personaApplyInput
	personaChanges []store.PersonaChangeEvent
	runtimeState   store.BotRuntimeState
	updatedBot     store.Bot
	turnCount      int
}

func (s *testStore) UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error) {
	item.ID = "mem-" + item.Content
	s.memoryItems = append(s.memoryItems, item)
	return item, nil
}

func (s *testStore) ListMemoryItems(userID, botID string, limit int) []store.MemoryItem {
	items := make([]store.MemoryItem, 0, len(s.memoryItems))
	for _, item := range s.memoryItems {
		if item.UserID == userID && item.BotID == botID {
			items = append(items, item)
		}
	}
	return items
}

func (s *testStore) UpsertCompact(conversationID, compactJSON, untilTurnID string) error {
	s.compacts = append(s.compacts, store.ConversationCompact{
		ConversationID: conversationID,
		CompactJSON:    compactJSON,
		UntilTurnID:    untilTurnID,
	})
	return nil
}

func (s *testStore) GetBot(userID, botID string) (store.Bot, bool) {
	return s.updatedBot, true
}

func (s *testStore) ApplyBotPersonaPatch(userID, botID string, input personaApplyInput) (store.Bot, bool) {
	s.personaCalls = append(s.personaCalls, input)
	if len(input.IdentityAdds) > 0 {
		s.updatedBot.Identity += strings.Join(input.IdentityAdds, "；")
	}
	if len(input.PersonalityAdds) > 0 {
		s.updatedBot.Personality = append(s.updatedBot.Personality, input.PersonalityAdds...)
	}
	if len(input.ExpressionStyleAdds) > 0 {
		s.updatedBot.ExpressionStyle += strings.Join(input.ExpressionStyleAdds, "；")
	}
	if len(input.LifeContextAdds) > 0 {
		s.updatedBot.LifeContext += strings.Join(input.LifeContextAdds, "；")
	}
	if len(input.TaboosAndPreferencesAdds) > 0 {
		s.updatedBot.TaboosAndPreferences += strings.Join(input.TaboosAndPreferencesAdds, "；")
	}
	return s.updatedBot, true
}

func (s *testStore) IncrementTurnCount(userID, botID string) int {
	s.turnCount++
	return s.turnCount
}

func (s *testStore) UpsertBotRuntimeState(state store.BotRuntimeState) (store.BotRuntimeState, error) {
	s.runtimeState = state
	return state, nil
}

func (s *testStore) CreatePersonaChangeEvent(event store.PersonaChangeEvent) (store.PersonaChangeEvent, error) {
	if event.ID == "" {
		event.ID = "change-1"
	}
	s.personaChanges = append(s.personaChanges, event)
	return event, nil
}

func TestRunnerAutoAppliesStableIdentityFacts(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot", Identity: "朋友"}}
	var refreshedChanges []store.PersonaChangeEvent
	runner := NewRunner(RunnerDeps{
		Store: st,
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"kind":"self_fact","owner":"bot","content":"不是纯金融，更偏研究型","importance":88,"semantic_category":"identity","stability":"stable"}]}`, nil
		},
		SelfCognitionUpdater: func(_ context.Context, _ Input, bot store.Bot, changes []store.PersonaChangeEvent) (store.Bot, error) {
			refreshedChanges = append([]store.PersonaChangeEvent(nil), changes...)
			return bot, nil
		},
	})

	result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-1"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !result.PersonaUpdated {
		t.Fatalf("expected persona updated")
	}
	if len(st.personaCalls) != 1 {
		t.Fatalf("persona patch calls = %d, want 1", len(st.personaCalls))
	}
	if !strings.Contains(st.updatedBot.Identity, "偏研究型") {
		t.Fatalf("updated identity = %q", st.updatedBot.Identity)
	}
	if len(st.personaChanges) != 1 || st.personaChanges[0].Status != "accepted" {
		t.Fatalf("persona changes = %+v", st.personaChanges)
	}
	if st.personaChanges[0].Field != "identity" {
		t.Fatalf("change field = %q", st.personaChanges[0].Field)
	}
	if len(refreshedChanges) != 1 || refreshedChanges[0].Field != "identity" {
		t.Fatalf("refreshed changes = %+v", refreshedChanges)
	}
}

func TestRunnerSkipsTemporaryLifeContextForPersonaButKeepsAudit(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot", LifeContext: "住在东京"}}
	runner := NewRunner(RunnerDeps{
		Store: st,
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"kind":"self_fact","owner":"bot","content":"最近住在爸妈这边","importance":70,"semantic_category":"life_context","stability":"temporary"}]}`, nil
		},
	})

	result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-1"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.PersonaUpdated {
		t.Fatalf("temporary fact should not update persona")
	}
	if len(st.personaCalls) != 0 {
		t.Fatalf("persona patch should not be applied")
	}
	if st.updatedBot.LifeContext != "住在东京" {
		t.Fatalf("life context should stay static, got %q", st.updatedBot.LifeContext)
	}
	if len(st.personaChanges) != 1 || st.personaChanges[0].Status != "skipped" {
		t.Fatalf("persona changes = %+v", st.personaChanges)
	}
	if st.personaChanges[0].Field != "life_context" {
		t.Fatalf("change field = %q", st.personaChanges[0].Field)
	}
}

func TestRunnerDoesNotUseLegacyTurnCountTriggerWithoutStableFacts(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot"}}
	runner := NewRunner(RunnerDeps{Store: st})

	for i := 0; i < 3; i++ {
		result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-a"})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if result.PersonaUpdated {
			t.Fatalf("persona should not update without stable facts")
		}
	}
	if len(st.personaChanges) != 0 {
		t.Fatalf("unexpected persona changes = %+v", st.personaChanges)
	}
}
