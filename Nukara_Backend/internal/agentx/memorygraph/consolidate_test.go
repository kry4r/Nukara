package memorygraph

import (
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestConsolidate_PromotesRepeatedEpisodesToHabit(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	nodes := []store.TemporalMemoryNode{
		{ID: "ep-1", UserID: "u1", BotID: "b1", NodeType: "episode", Title: "凌晨散步", Summary: "周一凌晨散步了半小时。", Status: "active", OccurredAt: now.Add(-72 * time.Hour)},
		{ID: "ep-2", UserID: "u1", BotID: "b1", NodeType: "episode", Title: "凌晨散步", Summary: "周三凌晨散步了四十分钟。", Status: "active", OccurredAt: now.Add(-36 * time.Hour)},
		{ID: "ep-3", UserID: "u1", BotID: "b1", NodeType: "episode", Title: "凌晨散步", Summary: "周四凌晨散步后心情稳定了。", Status: "active", OccurredAt: now.Add(-12 * time.Hour)},
		{ID: "ep-other", UserID: "u1", BotID: "b1", NodeType: "episode", Title: "深夜看电影", Summary: "昨晚又看了一部电影。", Status: "active", OccurredAt: now.Add(-6 * time.Hour)},
	}

	result := PromoteRepeatedEpisodesToHabit(nodes, ConsolidationOptions{Now: now, MinOccurrences: 3})
	if result.PromotedHabit == nil {
		t.Fatal("expected repeated episodes to produce a habit node")
	}
	if got := result.PromotedHabit.NodeType; got != "habit" {
		t.Fatalf("expected promoted node type habit, got %q", got)
	}
	if got := result.PromotedHabit.Title; got != "凌晨散步" {
		t.Fatalf("expected habit title to keep repeated motif, got %q", got)
	}
	if len(result.BackingNodeIDs) != 3 {
		t.Fatalf("expected 3 backing episode ids, got %d", len(result.BackingNodeIDs))
	}
}
