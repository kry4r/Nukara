package subtasks

import (
	"context"
	"testing"

	"nukara/backend/internal/agentx/memorygraph"
	"nukara/backend/internal/store"
)

func TestRunner_WritesEpisodePromiseAndStateNodes(t *testing.T) {
	st := store.NewStore()
	runner := NewRunner(RunnerDeps{
		Store:       st,
		MemoryGraph: memorygraph.NewService(memorygraph.ServiceDeps{Store: st}),
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[
				{"memory_id":"m1","kind":"event","owner":"bot","content":"昨晚下班后去散步了","importance":80},
				{"memory_id":"m2","kind":"promise","owner":"bot","content":"这周要把摄影课报上","importance":90},
				{"memory_id":"m3","kind":"state_basis","owner":"bot","content":"今天有点累，但情绪稳定","importance":60},
				{"memory_id":"m4","kind":"user_fact","owner":"user","content":"我最近开始把散步当作下班后的固定习惯","importance":75}
			]}`,
				nil
		},
	})

	if _, err := runner.Run(context.Background(), Input{UserID: "u1", BotID: "b1", ConversationID: "conv-1", TurnID: "turn-1"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	nodes := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{Limit: 20})
	assertTemporalNodeType(t, nodes, "episode")
	assertTemporalNodeType(t, nodes, "promise")
	assertTemporalNodeType(t, nodes, "state_snapshot")
	assertTemporalNodeType(t, nodes, "user_fact")
}

func TestRunner_DoesNotPromoteHighRiskUserFact(t *testing.T) {
	st := store.NewStore()
	runner := NewRunner(RunnerDeps{
		Store:       st,
		MemoryGraph: memorygraph.NewService(memorygraph.ServiceDeps{Store: st}),
		MemoryExtractor: func(context.Context, Input) (string, error) {
			return `{"items":[{"memory_id":"m1","kind":"user_fact","owner":"user","content":"我喜欢被束缚","importance":80}]}`,
				nil
		},
	})

	if _, err := runner.Run(context.Background(), Input{UserID: "u1", BotID: "b1", ConversationID: "conv-1", TurnID: "turn-1"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	nodes := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{Limit: 20})
	if hasTemporalNodeType(nodes, "user_fact") {
		t.Fatal("expected high-risk user fact not to be promoted as user_fact node")
	}
	assertTemporalNodeType(t, nodes, "episode")
}

func TestRunner_MaterializesSessionSummaryNode(t *testing.T) {
	st := store.NewStore()
	runner := NewRunner(RunnerDeps{
		Store:       st,
		MemoryGraph: memorygraph.NewService(memorygraph.ServiceDeps{Store: st}),
		CompactUpdater: func(context.Context, Input) (string, error) {
			return `{"summary":"最近一直在聊散步和摄影课","facts":["散步让状态稳定","摄影课还没报名"]}`,
				nil
		},
	})

	if _, err := runner.Run(context.Background(), Input{UserID: "u1", BotID: "b1", ConversationID: "conv-1", TurnID: "turn-1"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	nodes := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{NodeTypes: []string{"session_summary"}, Limit: 10})
	if len(nodes) != 1 {
		t.Fatalf("expected one session_summary node, got %d", len(nodes))
	}
	if nodes[0].SessionID != "conv-1" {
		t.Fatalf("session summary session_id = %q", nodes[0].SessionID)
	}
	facts := st.ListMemoryNodes("u1", "b1", store.TemporalMemoryNodeFilter{NodeTypes: []string{"episode"}, Limit: 10})
	if len(facts) != 2 {
		t.Fatalf("expected two materialized summary fact nodes, got %d", len(facts))
	}
	edges := st.ListMemoryEdges([]string{nodes[0].ID}, store.TemporalMemoryEdgeFilter{EdgeTypes: []string{"summarizes"}, Limit: 10})
	if len(edges) != 2 {
		t.Fatalf("expected two summarizes edges, got %d", len(edges))
	}
}

func assertTemporalNodeType(t *testing.T, nodes []store.TemporalMemoryNode, want string) {
	t.Helper()
	if !hasTemporalNodeType(nodes, want) {
		t.Fatalf("expected temporal node type %q", want)
	}
}

func hasTemporalNodeType(nodes []store.TemporalMemoryNode, want string) bool {
	for _, node := range nodes {
		if node.NodeType == want {
			return true
		}
	}
	return false
}
