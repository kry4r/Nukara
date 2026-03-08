package admin

import (
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
	})
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
