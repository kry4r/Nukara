package api

import (
	"context"
	"strings"
	"testing"

	"nukara/backend/internal/agentx/memorygraph"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

type fakeTemporalRecall struct {
	result memorygraph.RecallResult
}

func (f fakeTemporalRecall) Recall(_ context.Context, _ memorygraph.RecallInput) (memorygraph.RecallResult, error) {
	return f.result, nil
}

func TestBuildRuntimeContextUsesInjectedTemporalRecallResults(t *testing.T) {
	st := store.NewStore()
	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	server.SetTemporalMemoryRecall(fakeTemporalRecall{result: memorygraph.RecallResult{Cards: []memorygraph.PromptCard{{
		CardType: "user_card",
		Text:     "用户：用户喜欢在下雨天听爵士乐",
	}}}})

	systemPrompt, _ := server.buildRuntimeContext("user-1", "bot-1", "conv-1", "你还记得我下雨天会做什么吗", nil, map[string]any{"persona": "温柔朋友"})
	if !strings.Contains(systemPrompt, "用户喜欢在下雨天听爵士乐") {
		t.Fatalf("temporal memory missing from system prompt: %s", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "【记忆卡片】") {
		t.Fatalf("expected memory card section, got=%s", systemPrompt)
	}
}
