package store

import (
	"testing"
	"time"
)

func TestTemporalMemoryGraphStore_CreateNodeAndEdge(t *testing.T) {
	s := NewStore()

	episode, err := s.CreateMemoryNode(TemporalMemoryNode{
		UserID:     "user-1",
		BotID:      "bot-1",
		SessionID:  "conv-1",
		NodeType:   "episode",
		Title:      "下班后散步",
		Summary:    "最近开始把凌晨散步当作下班后的固定习惯。",
		Status:     "active",
		OccurredAt: time.Now().UTC().Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create episode node: %v", err)
	}
	if episode.ID == "" {
		t.Fatal("expected episode node id")
	}

	summary, err := s.CreateMemoryNode(TemporalMemoryNode{
		UserID:     "user-1",
		BotID:      "bot-1",
		SessionID:  "conv-1",
		NodeType:   "session_summary",
		Title:      "会话摘要",
		Summary:    "这段会话里提到了新的下班习惯。",
		Status:     "active",
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create session summary node: %v", err)
	}

	edge, err := s.CreateMemoryEdge(TemporalMemoryEdge{
		SourceID:      episode.ID,
		TargetID:      summary.ID,
		EdgeType:      "summarizes",
		Weight:        1,
		Status:        "active",
		EvidenceCount: 1,
	})
	if err != nil {
		t.Fatalf("create edge: %v", err)
	}
	if edge.ID == "" {
		t.Fatal("expected edge id")
	}

	edges := s.ListMemoryEdges([]string{episode.ID}, TemporalMemoryEdgeFilter{Limit: 10})
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].EdgeType != "summarizes" {
		t.Fatalf("edge_type = %q", edges[0].EdgeType)
	}
}

func TestTemporalMemoryGraphStore_UpdateNodeValidity(t *testing.T) {
	s := NewStore()

	node, err := s.CreateMemoryNode(TemporalMemoryNode{
		UserID:     "user-1",
		BotID:      "bot-1",
		NodeType:   "habit",
		Title:      "凌晨散步",
		Summary:    "最近会在下班后散步。",
		Status:     "active",
		OccurredAt: time.Now().UTC().Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	validTo := time.Now().UTC()
	node.Status = "archived"
	node.ValidTo = &validTo
	updated, err := s.UpdateMemoryNode(node)
	if err != nil {
		t.Fatalf("update node: %v", err)
	}
	if updated.Status != "archived" {
		t.Fatalf("status = %q", updated.Status)
	}
	if updated.ValidTo == nil || !updated.ValidTo.Equal(validTo) {
		t.Fatalf("valid_to = %#v want %s", updated.ValidTo, validTo)
	}

	got, ok := s.GetMemoryNode(node.ID)
	if !ok {
		t.Fatal("expected updated node")
	}
	if got.Status != "archived" {
		t.Fatalf("stored status = %q", got.Status)
	}
}

func TestTemporalMemoryGraphStore_ListSessionSummaryNodes(t *testing.T) {
	s := NewStore()

	_, err := s.CreateMemoryNode(TemporalMemoryNode{
		UserID:    "user-1",
		BotID:     "bot-1",
		SessionID: "conv-1",
		NodeType:  "session_summary",
		Title:     "摘要一",
		Summary:   "第一段摘要",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("create node 1: %v", err)
	}
	_, err = s.CreateMemoryNode(TemporalMemoryNode{
		UserID:    "user-1",
		BotID:     "bot-1",
		SessionID: "conv-2",
		NodeType:  "session_summary",
		Title:     "摘要二",
		Summary:   "第二段摘要",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("create node 2: %v", err)
	}
	_, err = s.CreateMemoryNode(TemporalMemoryNode{
		UserID:   "user-1",
		BotID:    "bot-1",
		NodeType: "episode",
		Title:    "普通事件",
		Summary:  "不是摘要节点",
		Status:   "active",
	})
	if err != nil {
		t.Fatalf("create episode node: %v", err)
	}

	nodes := s.ListMemoryNodes("user-1", "bot-1", TemporalMemoryNodeFilter{
		NodeTypes: []string{"session_summary"},
		SessionID: "conv-1",
		Limit:     10,
	})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 session summary node, got %d", len(nodes))
	}
	if nodes[0].NodeType != "session_summary" {
		t.Fatalf("node_type = %q", nodes[0].NodeType)
	}
	if nodes[0].SessionID != "conv-1" {
		t.Fatalf("session_id = %q", nodes[0].SessionID)
	}
}
