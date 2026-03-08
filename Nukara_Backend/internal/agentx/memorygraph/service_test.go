package memorygraph

import (
	"context"
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestService_IngestTurn_PersistsGraphNodesAndSessionSummary(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	st := store.NewStore()
	svc := NewService(ServiceDeps{Store: st})

	result, err := svc.IngestTurn(context.Background(), IngestTurnInput{
		UserID:         "u1",
		BotID:          "b1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		Now:            now,
		Items: []store.MemoryItem{
			{ID: "m1", Kind: "event", Content: "昨晚下班后去散步了", Status: "active", OccurredAt: now.Add(-2 * time.Hour), Importance: 80},
			{ID: "m2", Kind: "promise", Content: "这周要把摄影课报上", Status: "active", OccurredAt: now.Add(-90 * time.Minute), Importance: 85},
			{ID: "m3", Kind: "state_basis", Content: "今天有点累，但情绪稳定", Status: "active", OccurredAt: now.Add(-30 * time.Minute), Importance: 60},
			{ID: "m4", Kind: "user_fact", Content: "用户最近开始把散步当作下班后的固定习惯", Status: "active", OccurredAt: now.Add(-20 * time.Minute), Importance: 75},
			{ID: "m5", Kind: "user_fact", Content: "我喜欢被束缚", Status: "active", OccurredAt: now.Add(-10 * time.Minute), Importance: 70},
		},
		CompactJSON: `{"summary":"最近一直在聊下班后的散步和摄影课","facts":["散步让状态稳定","摄影课还没报名"]}`,
	})
	if err != nil {
		t.Fatalf("IngestTurn failed: %v", err)
	}
	if result.SessionSummary == nil {
		t.Fatal("expected session summary node")
	}
	stored := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{Limit: 20})
	assertNodeTypePresent(t, stored, "episode")
	assertNodeTypePresent(t, stored, "promise")
	assertNodeTypePresent(t, stored, "state_snapshot")
	assertNodeTypePresent(t, stored, "user_fact")
	assertNodeTypePresent(t, stored, "session_summary")
	if countNodeType(stored, "user_fact") != 1 {
		t.Fatalf("expected only low-risk user_fact to be promoted, got %d", countNodeType(stored, "user_fact"))
	}
}

func TestService_Recall_ReturnsCardsAndSavesTrace(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	st := store.NewStore()
	svc := NewService(ServiceDeps{Store: st})

	_, err := svc.IngestTurn(context.Background(), IngestTurnInput{
		UserID:         "u1",
		BotID:          "b1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		Now:            now,
		Items: []store.MemoryItem{
			{ID: "m1", Kind: "event", Content: "昨晚散步后想起这周要报名摄影课", Status: "active", OccurredAt: now.Add(-4 * time.Hour), Importance: 80},
			{ID: "m2", Kind: "promise", Content: "这周要把摄影课报上", Status: "active", OccurredAt: now.Add(-3 * time.Hour), Importance: 90},
			{ID: "m3", Kind: "state_basis", Content: "今天有点累，但还撑得住", Status: "active", OccurredAt: now.Add(-90 * time.Minute), Importance: 60},
		},
	})
	if err != nil {
		t.Fatalf("IngestTurn failed: %v", err)
	}
	_, err = st.CreateMemoryNode(store.TemporalMemoryNode{UserID: "u1", BotID: "b1", SessionID: "conv-1", NodeType: "self_model", Title: "最近的自我认知", Summary: "我最近把凌晨散步当作收束一天的方式。", Status: "active", OccurredAt: now.Add(-2 * time.Hour), ObservedAt: now, ValidFrom: now.Add(-2 * time.Hour)})
	if err != nil {
		t.Fatalf("CreateMemoryNode failed: %v", err)
	}

	result, err := svc.Recall(context.Background(), RecallInput{
		UserID:          "u1",
		BotID:           "b1",
		ConversationID:  "conv-1",
		TurnID:          "turn-2",
		QueryText:       "你还记得我这周没做完什么吗",
		RecentTexts:     []string{"昨晚散步后想起摄影课", "这周要把摄影课报上"},
		ActivationLimit: 6,
		MaxDepth:        2,
		CardBudget:      CardBudget{MaxChars: 160, MaxCards: 5},
		Now:             now,
	})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(result.Cards) == 0 {
		t.Fatal("expected recall cards")
	}
	if !hasCardType(result.Cards, "open_loops_card") {
		t.Fatal("expected open_loops_card in recall result")
	}
	traces := st.ListActivationTraces("u1", "b1", store.ActivationTraceFilter{ConversationID: "conv-1", Limit: 5})
	if len(traces) != 1 {
		t.Fatalf("expected one activation trace, got %d", len(traces))
	}
	if len(traces[0].SelectedCardIDs) == 0 {
		t.Fatal("expected activation trace to include selected card ids")
	}
}

func TestService_IngestTurn_MergesHighlySimilarFacts(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	st := store.NewStore()
	svc := NewService(ServiceDeps{Store: st})

	_, err := svc.IngestTurn(context.Background(), IngestTurnInput{
		UserID:         "u1",
		BotID:          "b1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		Now:            now,
		Items: []store.MemoryItem{{
			ID:               "m1",
			Kind:             "self_fact",
			Content:          "我养了一只猫叫小蜜",
			Status:           "active",
			SemanticCategory: "life_context",
			Stability:        "stable",
			OccurredAt:       now.Add(-2 * time.Hour),
		}},
	})
	if err != nil {
		t.Fatalf("first IngestTurn failed: %v", err)
	}

	_, err = svc.IngestTurn(context.Background(), IngestTurnInput{
		UserID:         "u1",
		BotID:          "b1",
		ConversationID: "conv-1",
		TurnID:         "turn-2",
		Now:            now.Add(time.Minute),
		Items: []store.MemoryItem{{
			ID:               "m2",
			Kind:             "self_fact",
			Content:          "我养了只猫，名字叫小蜜",
			Status:           "active",
			SemanticCategory: "life_context",
			Stability:        "stable",
			OccurredAt:       now.Add(-time.Hour),
		}},
	})
	if err != nil {
		t.Fatalf("second IngestTurn failed: %v", err)
	}

	stored := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{NodeTypes: []string{"episode"}, Limit: 10})
	if len(stored) != 1 {
		t.Fatalf("expected merged episode node, got %d", len(stored))
	}
	if stored[0].EvidenceCount != 2 {
		t.Fatalf("evidence_count = %d, want 2", stored[0].EvidenceCount)
	}
}

func TestService_IngestTurn_MaterializesSummaryFactNodes(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	st := store.NewStore()
	svc := NewService(ServiceDeps{Store: st})

	result, err := svc.IngestTurn(context.Background(), IngestTurnInput{
		UserID:         "u1",
		BotID:          "b1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		Now:            now,
		CompactJSON:    `{"summary":"最近一直在聊散步和摄影课","facts":["散步让状态稳定","摄影课还没报名"]}`,
	})
	if err != nil {
		t.Fatalf("IngestTurn failed: %v", err)
	}
	if result.SessionSummary == nil {
		t.Fatal("expected session summary node")
	}

	stored := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{Limit: 20})
	if countNodeType(stored, "session_summary") != 1 {
		t.Fatalf("expected one session_summary node, got %d", countNodeType(stored, "session_summary"))
	}
	if countNodeType(stored, "episode") != 2 {
		t.Fatalf("expected two materialized fact nodes, got %d", countNodeType(stored, "episode"))
	}
	edges := st.ListMemoryEdges([]string{result.SessionSummary.ID}, store.TemporalMemoryEdgeFilter{EdgeTypes: []string{"summarizes"}, Limit: 10})
	if len(edges) != 2 {
		t.Fatalf("expected two summarizes edges, got %d", len(edges))
	}
	for _, edge := range edges {
		if edge.SourceID != result.SessionSummary.ID {
			t.Fatalf("unexpected edge source: %+v", edge)
		}
	}
}

func TestService_Consolidate_PromotesHabitAndPersistsIt(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	st := store.NewStore()
	svc := NewService(ServiceDeps{Store: st})
	for idx, at := range []time.Time{now.Add(-72 * time.Hour), now.Add(-48 * time.Hour), now.Add(-24 * time.Hour)} {
		_, err := st.CreateMemoryNode(store.TemporalMemoryNode{UserID: "u1", BotID: "b1", SessionID: "conv-1", NodeType: "episode", Title: "凌晨散步", Summary: "又去凌晨散步了", Status: "active", OccurredAt: at, ObservedAt: now, ValidFrom: at, SourceTurnID: "turn-habit"})
		if err != nil {
			t.Fatalf("create episode %d failed: %v", idx, err)
		}
	}

	result, err := svc.Consolidate(context.Background(), ConsolidateInput{UserID: "u1", BotID: "b1", Now: now, MinOccurrences: 3})
	if err != nil {
		t.Fatalf("Consolidate failed: %v", err)
	}
	if result.PromotedHabit == nil {
		t.Fatal("expected promoted habit")
	}
	stored := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{NodeTypes: []string{"habit"}, Limit: 10})
	if len(stored) != 1 {
		t.Fatalf("expected persisted habit node, got %d", len(stored))
	}
}

func assertNodeTypePresent(t *testing.T, nodes []store.TemporalMemoryNode, want string) {
	t.Helper()
	for _, node := range nodes {
		if node.NodeType == want {
			return
		}
	}
	t.Fatalf("expected node type %q to be present", want)
}

func countNodeType(nodes []store.TemporalMemoryNode, want string) int {
	count := 0
	for _, node := range nodes {
		if node.NodeType == want {
			count++
		}
	}
	return count
}
