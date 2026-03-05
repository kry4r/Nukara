package subtasks

import (
	"context"
	"testing"

	"nukara/backend/internal/store"
)

type testStore struct {
	memoryItems   []store.MemoryItem
	compacts      []store.ConversationCompact
	personaCalls  []personaApplyInput
	updatedBot    store.Bot
}

func (s *testStore) UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error) {
	item.ID = "mem-" + item.Content
	s.memoryItems = append(s.memoryItems, item)
	return item, nil
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
	s.updatedBot.SelfCognition = append(s.updatedBot.SelfCognition, input.SelfCognitionAdds...)
	if input.Relationship != "" {
		s.updatedBot.Relationship = input.Relationship
	}
	return s.updatedBot, true
}

func TestRunnerAppliesValidatedPatchAndPersistsOutputs(t *testing.T) {
	st := &testStore{
		updatedBot: store.Bot{
			ID:           "bot-1",
			UserID:       "user-1",
			Name:         "bot",
			Relationship: "朋友",
		},
	}
	runner := NewRunner(RunnerDeps{
		Store: st,
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"kind":"fact","owner":"user","content":"用户喜欢喝茶","importance":70}]}`, nil
		},
		CompactUpdater: func(context.Context, Input) (string, error) {
			return `{"summary":"最近在聊喝茶","facts":["用户喜欢喝茶"]}`, nil
		},
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"relationship":"更亲近的朋友","self_cognition_adds":["我会记得用户偏好"],"speaking_style_adds":["更自然"],"trait_adds":["细致"],"gender":"female"}`, nil
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
	if len(st.personaCalls) != 1 {
		t.Fatalf("persona patch not applied")
	}
	if !result.PersonaUpdated {
		t.Fatalf("expected persona updated")
	}
	if result.PatchSummary == "" {
		t.Fatalf("expected patch summary")
	}
}

func TestRunnerRejectsInvalidPersonaPatch(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1"}}
	runner := NewRunner(RunnerDeps{
		Store: st,
		PersonaIterator: func(context.Context, Input) (string, error) {
			return `{"gender":"invalid","self_cognition_adds":["x"]}`, nil
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

