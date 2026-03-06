package api

import (
	"context"
	"strings"
	"testing"

	agentxmemory "nukara/backend/internal/agentx/memory"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

type fakeSemanticRecall struct {
	items []store.MemoryItem
}

func (f fakeSemanticRecall) Build(_ context.Context, in agentxmemory.RecallInput) ([]store.MemoryItem, error) {
	return append([]store.MemoryItem(nil), f.items...), nil
}

func TestBuildRuntimeContextUsesSemanticRecallResults(t *testing.T) {
	st := store.NewStore()
	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	server.SetMemoryServices(nil, fakeSemanticRecall{items: []store.MemoryItem{{
		ID:         "mem-1",
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户喜欢在下雨天听爵士乐",
		Importance: 90,
		Status:     "active",
		Topics:     []string{"爵士乐", "下雨天"},
	}}})

	systemPrompt, _ := server.buildRuntimeContext("user-1", "bot-1", "conv-1", "你还记得我下雨天会做什么吗", nil, map[string]any{"persona": "温柔朋友"})
	if !strings.Contains(systemPrompt, "用户喜欢在下雨天听爵士乐") {
		t.Fatalf("semantic memory missing from system prompt: %s", systemPrompt)
	}
}
