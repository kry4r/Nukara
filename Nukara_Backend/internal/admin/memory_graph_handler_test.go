package admin

import (
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestBuildMemoryGraphResponseCreatesMemoryTopicNodesAndEdges(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	graph := buildMemoryGraphResponse([]store.MemoryItem{
		{
			ID:         "mem-1",
			UserID:     "user-1",
			BotID:      "bot-1",
			Owner:      "user",
			Content:    "喜欢深夜散步",
			Importance: 90,
			OccurredAt: now,
			Topics:     []string{"夜晚", "散步"},
		},
	}, memoryGraphContext{})
	if len(graph.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("edges = %d, want 2", len(graph.Edges))
	}
	if graph.Summary.MemoryCount != 1 || graph.Summary.TopicCount != 2 {
		t.Fatalf("unexpected summary: %+v", graph.Summary)
	}
}

func TestBuildTemporalMemoryGraphResponseIncludesTemporalNodesAndEdges(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	graph := buildTemporalMemoryGraphResponse([]store.TemporalMemoryNode{
		{
			ID:         "tm:turn-1:promise:m1",
			NodeType:   "promise",
			Title:      "摄影课",
			Summary:    "这周要把摄影课报上",
			Status:     "active",
			OccurredAt: now.Add(-time.Hour),
			Salience:   0.86,
		},
		{
			ID:         "session-summary:conv-1",
			NodeType:   "session_summary",
			Title:      "会话摘要",
			Summary:    "最近一直在聊散步和摄影课",
			Status:     "active",
			OccurredAt: now,
			Salience:   0.91,
		},
	}, []store.TemporalMemoryEdge{{
		ID:       "edge-1",
		SourceID: "tm:turn-1:promise:m1",
		TargetID: "session-summary:conv-1",
		EdgeType: "summarizes",
	}}, memoryGraphContext{})

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 graph nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 graph edge, got %d", len(graph.Edges))
	}
	if graph.Summary.MemoryCount != 2 {
		t.Fatalf("memory_count = %d, want 2", graph.Summary.MemoryCount)
	}
	if graph.Summary.GraphSource != "temporal_memory_graph" {
		t.Fatalf("graph_source = %q, want temporal_memory_graph", graph.Summary.GraphSource)
	}
	if graph.Nodes[0].Type != "memory" && graph.Nodes[1].Type != "memory" {
		t.Fatalf("expected temporal nodes to map to memory nodes: %+v", graph.Nodes)
	}
}

func TestBuildMemoryGraphResponseIncludesRuntimeProfileViews(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	graph := buildMemoryGraphResponse([]store.MemoryItem{
		{
			ID:         "mem-1",
			UserID:     "user-1",
			BotID:      "bot-1",
			Kind:       "promise",
			Owner:      "bot",
			Content:    "答应今晚把歌单发给你",
			Importance: 90,
			Status:     "active",
			OccurredAt: now,
			Topics:     []string{"歌单", "约定"},
		},
	}, memoryGraphContext{
		KindFilter:   "promise",
		StatusFilter: "active",
		RuntimeState: &store.BotRuntimeState{
			UserID:       "user-1",
			BotID:        "bot-1",
			ActivityText: "刚下晚班，在回去路上",
			BasisTags:    []string{"self_fact"},
			UpdatedAt:    now,
		},
		RecentImpressions: []store.MemoryItem{{
			ID:         "mem-2",
			Kind:       "impression",
			Owner:      "bot",
			Content:    "你给我的感觉是细腻、可靠。",
			Importance: 88,
			Status:     "active",
			OccurredAt: now,
		}},
		RecentChanges: []store.PersonaChangeEvent{{
			ID:            "chg-1",
			Field:         "identity",
			ChangeType:    "append",
			ProposedValue: "其实是夜班护士",
			Status:        "accepted",
			CreatedAt:     now,
			UpdatedAt:     now,
		}},
		PendingPersonaChanges: []store.PersonaChangeEvent{{
			ID:            "chg-2",
			Field:         "life_context",
			ChangeType:    "append",
			ProposedValue: "最近换到夜班节奏",
			Status:        "pending",
			CreatedAt:     now,
			UpdatedAt:     now,
		}},
	})

	if graph.RuntimeState == nil || graph.RuntimeState.ActivityText != "刚下晚班，在回去路上" {
		t.Fatalf("unexpected runtime_state: %+v", graph.RuntimeState)
	}
	if len(graph.RecentImpressions) != 1 || graph.RecentImpressions[0].Kind != "impression" || graph.RecentImpressions[0].Content != "你给我的感觉是细腻、可靠。" {
		t.Fatalf("unexpected recent_impressions: %+v", graph.RecentImpressions)
	}
	if len(graph.RecentChanges) != 1 || graph.RecentChanges[0].Status != "accepted" {
		t.Fatalf("unexpected recent_changes: %+v", graph.RecentChanges)
	}
	if strings.TrimSpace(graph.RecentChanges[0].SummaryText) == "" {
		t.Fatalf("expected summary_text on recent_changes: %+v", graph.RecentChanges[0])
	}
	if len(graph.PendingPersonaChanges) != 1 || graph.PendingPersonaChanges[0].Status != "pending" {
		t.Fatalf("unexpected pending changes: %+v", graph.PendingPersonaChanges)
	}
	if graph.Summary.KindFilter != "promise" || graph.Summary.StatusFilter != "active" {
		t.Fatalf("unexpected summary filters: %+v", graph.Summary)
	}
}
