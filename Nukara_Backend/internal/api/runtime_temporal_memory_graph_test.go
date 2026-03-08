package api

import (
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestBuildRuntimeContext_UsesTemporalMemoryCards(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("runtime-graph-1@example.com", "runtime-graph-1")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	bot := st.CreateBot(user.ID, store.Bot{Name: "苏子衿", Identity: "温柔朋友", Personality: []string{"细腻"}, ExpressionStyle: "口语化", LifeContext: "住在东京"})
	conv, found := st.FindConversationByBot(user.ID, bot.ID)
	if !found {
		t.Fatal("conversation not found")
	}
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: "self_model", Title: "最近的自我认知", Summary: "我最近把凌晨散步当作收束一天的方式。", Status: "active", OccurredAt: now.Add(-2 * time.Hour), ObservedAt: now, ValidFrom: now.Add(-2 * time.Hour)})
	mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: "state_snapshot", Title: "当前状态", Summary: "今天有点累，但情绪稳定。", Status: "active", OccurredAt: now.Add(-90 * time.Minute), ObservedAt: now, ValidFrom: now.Add(-90 * time.Minute)})
	mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: "promise", Title: "报名摄影课", Summary: "这周还没把摄影课报上。", Status: "active", OccurredAt: now.Add(-3 * time.Hour), ObservedAt: now, ValidFrom: now.Add(-3 * time.Hour)})
	mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: "session_summary", Title: "会话摘要", Summary: "最近一直在聊散步和摄影课。", Status: "active", OccurredAt: now.Add(-30 * time.Minute), ObservedAt: now, ValidFrom: now.Add(-30 * time.Minute)})
	mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: "episode", Title: "昨晚散步", Summary: "昨晚散步后又想起摄影课这件事。", Status: "active", OccurredAt: now.Add(-4 * time.Hour), ObservedAt: now, ValidFrom: now.Add(-4 * time.Hour)})

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	systemContext := agent.BuildSystemContext(bot, nil)
	prompt, _ := server.buildRuntimeContext(user.ID, bot.ID, conv.ID, "你还记得我这周没做完什么吗", nil, systemContext)

	if !strings.Contains(prompt, "【记忆卡片】") {
		t.Fatalf("expected temporal memory card section, got=%s", prompt)
	}
	if !strings.Contains(prompt, "待办：报名摄影课") {
		t.Fatalf("expected open loop card content, got=%s", prompt)
	}
	if !strings.Contains(prompt, "自我：我最近把凌晨散步当作收束一天的方式") {
		t.Fatalf("expected self card content, got=%s", prompt)
	}
}

func TestBuildRuntimeContext_UsesSessionSummaryBridge(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("runtime-graph-2@example.com", "runtime-graph-2")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	bot := st.CreateBot(user.ID, store.Bot{Name: "苏子衿", Identity: "温柔朋友"})
	conv, found := st.FindConversationByBot(user.ID, bot.ID)
	if !found {
		t.Fatal("conversation not found")
	}
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: "session_summary", Title: "会话摘要", Summary: "上一段对话里已经聊到你打算换工作节奏。", Status: "active", OccurredAt: now.Add(-20 * time.Minute), ObservedAt: now, ValidFrom: now.Add(-20 * time.Minute)})

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	prompt, _ := server.buildRuntimeContext(user.ID, bot.ID, conv.ID, "我们上次聊到哪了", nil, map[string]any{"persona": "温柔朋友"})
	if !strings.Contains(prompt, "衔接：上一段对话里已经聊到你打算换工作节奏") {
		t.Fatalf("expected session bridge card content, got=%s", prompt)
	}
}

func TestBuildRuntimeContext_BoundsPromptMemoryBudget(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("runtime-graph-3@example.com", "runtime-graph-3")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	bot := st.CreateBot(user.ID, store.Bot{Name: "苏子衿", Identity: "温柔朋友"})
	conv, found := st.FindConversationByBot(user.ID, bot.ID)
	if !found {
		t.Fatal("conversation not found")
	}
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	for i, node := range []store.TemporalMemoryNode{
		{NodeType: "self_model", Title: "最近的自我认知", Summary: "我最近把凌晨散步当作收束一天的方式，也把摄影课当作慢慢恢复表达欲的入口。", Status: "active"},
		{NodeType: "state_snapshot", Title: "当前状态", Summary: "今天事情很多，有点累，但还在尽量把节奏重新收好。", Status: "active"},
		{NodeType: "promise", Title: "报名摄影课", Summary: "这周还没把摄影课报上，心里一直挂着。", Status: "active"},
		{NodeType: "episode", Title: "昨晚散步", Summary: "昨晚散步时又想到最近为什么总拖着没报名。", Status: "active"},
		{NodeType: "session_summary", Title: "会话摘要", Summary: "最近一直在聊散步、摄影课和下班后的节奏。", Status: "active"},
	} {
		mustCreateNode(t, st, store.TemporalMemoryNode{UserID: user.ID, BotID: bot.ID, SessionID: conv.ID, NodeType: node.NodeType, Title: node.Title, Summary: node.Summary, Status: node.Status, OccurredAt: now.Add(time.Duration(-i-1) * time.Hour), ObservedAt: now, ValidFrom: now.Add(time.Duration(-i-1) * time.Hour)})
	}

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	prompt, _ := server.buildRuntimeContext(user.ID, bot.ID, conv.ID, "你还记得什么", nil, map[string]any{"persona": "温柔朋友"})
	memorySection := extractRuntimeSection(prompt, "【记忆卡片】")
	if memorySection == "" {
		t.Fatalf("expected memory card section, got=%s", prompt)
	}
	if len([]rune(memorySection)) > 220 {
		t.Fatalf("expected bounded memory card section, got %d runes", len([]rune(memorySection)))
	}
}

func mustCreateNode(t *testing.T, st *store.Store, node store.TemporalMemoryNode) store.TemporalMemoryNode {
	t.Helper()
	created, err := st.CreateMemoryNode(node)
	if err != nil {
		t.Fatalf("create memory node failed: %v", err)
	}
	return created
}

func extractRuntimeSection(prompt, title string) string {
	idx := strings.Index(prompt, title)
	if idx < 0 {
		return ""
	}
	rest := prompt[idx+len(title):]
	rest = strings.TrimPrefix(rest, "\n")
	parts := strings.Split(rest, "\n\n")
	if len(parts) == 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(parts[0])
}
