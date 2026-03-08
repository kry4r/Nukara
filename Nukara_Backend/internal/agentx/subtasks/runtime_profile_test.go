package subtasks

import (
	"context"
	"testing"

	"nukara/backend/internal/store"
)

func TestRunnerUpdatesRuntimeStateFromUsefulSelfFact(t *testing.T) {
	st := &testStore{updatedBot: store.Bot{ID: "bot-1", UserID: "user-1", Name: "bot"}}
	runner := NewRunner(RunnerDeps{
		Store: st,
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"kind":"self_fact","owner":"bot","content":"刚下晚班，在回去路上","importance":80}]}`, nil
		},
	})

	if _, err := runner.Run(context.Background(), Input{UserID: "user-1", BotID: "bot-1", ConversationID: "conv-1", TurnID: "turn-1"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if st.runtimeState.ActivityText != "刚下晚班，在回去路上" {
		t.Fatalf("runtime activity = %q", st.runtimeState.ActivityText)
	}
}
