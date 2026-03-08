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

func TestRunnerAppliesValidatedLowRiskPatchAndPersistsOutputs(t *testing.T) {
	st := &testStore{
		updatedBot: store.Bot{
			ID:          "bot-1",
			UserID:      "user-1",
			Name:        "bot",
			Identity:    "朋友",
			LifeContext: "住在东京",
		},
		turnCount: 2,
	}
	var refreshedBot store.Bot
	var refreshedChanges []store.PersonaChangeEvent
	runner := NewRunner(RunnerDeps{
		Store: st,
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"kind":"fact","owner":"user","content":"用户喜欢喝茶","importance":70}]}`, nil
		},
		CompactUpdater: func(context.Context, Input) (string, error) {
			return `{"summary":"最近在聊喝茶","facts":["用户喜欢喝茶"]}`, nil
		},
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"life_context_adds":["最近开始把凌晨散步当作下班后的固定习惯"]}`, nil
		},
		SelfCognitionUpdater: func(_ context.Context, _ Input, bot store.Bot, changes []store.PersonaChangeEvent) (store.Bot, error) {
			refreshedChanges = append([]store.PersonaChangeEvent(nil), changes...)
			bot.SelfCognition = []string{"我最近会在下班后用凌晨散步让自己慢慢沉下来。"}
			st.updatedBot = bot
			refreshedBot = bot
			return bot, nil
		},
	})

	result, err := runner.Run(context.Background(), Input{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		UserText:       "我喜欢喝茶",
		BotText:        "记住啦",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(st.memoryItems) != 1 {
		t.Fatalf("memory items persisted = %d, want 1", len(st.memoryItems))
	}
	if len(st.compacts) != 1 || st.compacts[0].UntilTurnID != "turn-1" {
		t.Fatalf("compact upsert mismatch: %+v", st.compacts)
	}
	if len(st.personaCalls) != 0 {
		t.Fatalf("low-risk patch should not mutate static persona directly")
	}
	if !result.PersonaUpdated {
		t.Fatalf("expected persona updated")
	}
	if result.PatchSummary == "" {
		t.Fatalf("expected patch summary")
	}
	if st.updatedBot.LifeContext != "住在东京" {
		t.Fatalf("life context should stay static, got %q", st.updatedBot.LifeContext)
	}
	if len(refreshedBot.SelfCognition) != 1 || !strings.Contains(refreshedBot.SelfCognition[0], "凌晨散步") {
		t.Fatalf("expected self cognition refreshed, got %#v", refreshedBot.SelfCognition)
	}
	if len(st.personaChanges) != 1 {
		t.Fatalf("accepted persona changes = %d, want 1", len(st.personaChanges))
	}
	if st.personaChanges[0].Status != "accepted" {
		t.Fatalf("accepted change status = %q", st.personaChanges[0].Status)
	}
	if st.personaChanges[0].Field != "life_context" {
		t.Fatalf("accepted change field = %q", st.personaChanges[0].Field)
	}
	if len(refreshedChanges) != 1 || refreshedChanges[0].ProposedValue != "最近开始把凌晨散步当作下班后的固定习惯" {
		t.Fatalf("refreshed changes = %+v", refreshedChanges)
	}
}

func TestRunnerRejectsInvalidPersonaPatch(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1"}, turnCount: 2}
	runner := NewRunner(RunnerDeps{
		Store: st,
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"identity_adds":[""]}`, nil
		},
	})

	result, err := runner.Run(context.Background(), Input{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
	})
	if err != nil {
		t.Fatalf("Run should not fail on invalid patch: %v", err)
	}
	if result.PersonaUpdated {
		t.Fatalf("invalid patch should be ignored")
	}
	if len(st.personaCalls) != 0 {
		t.Fatalf("invalid patch should not be applied")
	}
}

func TestRunnerOnlyIteratesPersonaEveryThreeTurns(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot"}}
	runner := NewRunner(RunnerDeps{
		Store: st,
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"expression_style_adds":["我会慢慢更懂你"]}`, nil
		},
	})

	for i := 0; i < 2; i++ {
		result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-a"})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if result.PersonaUpdated {
			t.Fatalf("persona should not update on turn %d", i+1)
		}
	}

	result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-3"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !result.PersonaUpdated {
		t.Fatalf("persona should update on third turn")
	}
	if len(st.personaCalls) != 0 {
		t.Fatalf("low-risk persona should not patch static bot directly, got %d calls", len(st.personaCalls))
	}
}

func TestRunnerIteratesPersonaOnImportantMemoryTurns(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot"}}
	runner := NewRunner(RunnerDeps{
		Store: st,
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"kind":"user_fact","owner":"user","content":"用户对猫毛过敏","importance":90}]}`, nil
		},
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"expression_style_adds":["会把重要提醒记得更牢"]}`, nil
		},
	})

	result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-1", UserText: "我对猫毛过敏", BotText: "我记住了"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !result.PersonaUpdated {
		t.Fatalf("important memory turn should trigger persona update")
	}
	if len(st.personaCalls) != 0 {
		t.Fatalf("low-risk persona should not patch static bot directly, got %d calls", len(st.personaCalls))
	}
	if len(st.personaChanges) != 1 || st.personaChanges[0].Status != "accepted" {
		t.Fatalf("accepted persona changes = %+v", st.personaChanges)
	}
}

func TestRunnerCreatesPendingPersonaChangeForHighRiskPatch(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot", Identity: "朋友"}, turnCount: 2}
	runner := NewRunner(RunnerDeps{
		Store: st,
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"identity_adds":["其实我是医生"]}`, nil
		},
	})

	result, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-3"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.PersonaUpdated {
		t.Fatalf("high-risk patch should not auto-apply")
	}
	if len(st.personaCalls) != 0 {
		t.Fatalf("high-risk patch should not be applied directly")
	}
	if len(st.personaChanges) != 1 {
		t.Fatalf("persona changes = %d, want 1", len(st.personaChanges))
	}
	if st.personaChanges[0].Field != "identity" {
		t.Fatalf("change field = %q", st.personaChanges[0].Field)
	}
	if st.personaChanges[0].Status != "pending" {
		t.Fatalf("change status = %q", st.personaChanges[0].Status)
	}
}
