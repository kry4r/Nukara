package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/subtasks"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

type stubSubtaskRuntime struct {
	lastReq agentx.TurnRequest
}

func (s *stubSubtaskRuntime) StreamTurn(_ context.Context, req agentx.TurnRequest) (<-chan agentx.StreamDelta, <-chan agentx.FinalTurn, error) {
	s.lastReq = req
	deltaCh := make(chan agentx.StreamDelta)
	finalCh := make(chan agentx.FinalTurn, 1)
	go func() {
		defer close(deltaCh)
		defer close(finalCh)
		text := `{"items":[]}`
		switch {
		case strings.Contains(req.AggregatedText, "[system:memory_extract_json]"):
			text = `{"items":[{"memory_id":"mem-drink-1","kind":"fact","owner":"user","content":"用户现在更喜欢绿茶，不太喝乌龙茶","importance":88,"status":"active","topics":["绿茶","乌龙茶","偏好"]}]}`
		case strings.Contains(req.AggregatedText, "[system:compact_update_json]"):
			text = `{"summary":"用户修正了饮品偏好","facts":["用户现在更喜欢绿茶"]}`
		case strings.Contains(req.AggregatedText, "[system:persona_iterate_json]"):
			text = `{"identity_adds":[],"personality_adds":[],"expression_style_adds":[],"life_context_adds":[],"taboos_and_preferences_adds":[]}`
		}
		finalCh <- agentx.FinalTurn{Segments: []agentx.FinalSegment{{Text: text}}}
	}()
	return deltaCh, finalCh, nil
}

func TestSubtasksUseRuntimeWhenAgentNilAndCanUpdateMemory(t *testing.T) {
	st := store.NewStore()
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		ID:         "mem-drink-1",
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户喜欢乌龙茶",
		Importance: 72,
		Status:     "active",
		Topics:     []string{"乌龙茶", "偏好"},
	}); err != nil {
		t.Fatalf("seed memory failed: %v", err)
	}

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	runtime := &stubSubtaskRuntime{}
	server.SetChatRuntime(runtime)

	if _, err := server.subtasks.Run(context.Background(), subtasks.Input{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		UserText:       "其实我现在更喜欢绿茶，不太喝乌龙了。",
		BotText:        "好，我以后按你现在的口味记。",
	}); err != nil {
		t.Fatalf("subtasks run failed: %v", err)
	}

	items := st.ListMemoryItems("user-1", "bot-1", 10)
	if len(items) != 1 {
		t.Fatalf("memory items = %d, want 1", len(items))
	}
	if items[0].ID != "mem-drink-1" {
		t.Fatalf("memory id = %s, want mem-drink-1", items[0].ID)
	}
	if items[0].Content != "用户现在更喜欢绿茶，不太喝乌龙茶" {
		t.Fatalf("memory content = %q", items[0].Content)
	}
	if items[0].Importance != 88 {
		t.Fatalf("memory importance = %d, want 88", items[0].Importance)
	}
	if got, ok := st.GetConversationCompact("conv-1"); !ok || !strings.Contains(got.CompactJSON, "绿茶") {
		t.Fatalf("compact not updated, got=(%v,%+v)", ok, got)
	}
	if runtime.lastReq.Purpose != "subtask" {
		t.Fatalf("runtime purpose = %q, want subtask", runtime.lastReq.Purpose)
	}
}

func TestBuildMemoryCandidateContextUsesTemporalGraphRecall(t *testing.T) {
	st := store.NewStore()
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	if _, err := st.CreateMemoryNode(store.TemporalMemoryNode{UserID: "user-1", BotID: "bot-1", SessionID: "conv-1", NodeType: "promise", Title: "报名摄影课", Summary: "这周还没把摄影课报上。", Status: "active", OccurredAt: now.Add(-2 * time.Hour), ObservedAt: now, ValidFrom: now.Add(-2 * time.Hour)}); err != nil {
		t.Fatalf("create promise node failed: %v", err)
	}
	if _, err := st.CreateMemoryNode(store.TemporalMemoryNode{UserID: "user-1", BotID: "bot-1", SessionID: "conv-1", NodeType: "episode", Title: "昨晚散步", Summary: "昨晚散步时又想到摄影课。", Status: "active", OccurredAt: now.Add(-3 * time.Hour), ObservedAt: now, ValidFrom: now.Add(-3 * time.Hour)}); err != nil {
		t.Fatalf("create episode node failed: %v", err)
	}

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	contextText := server.buildMemoryCandidateContext("user-1", "bot-1", "你还记得我这周没做完什么吗", "我记得你还有摄影课没报名")
	if !strings.Contains(contextText, "type=promise") {
		t.Fatalf("expected promise node in candidate context, got=%s", contextText)
	}
	if !strings.Contains(contextText, "content=这周还没把摄影课报上") {
		t.Fatalf("expected node summary in candidate context, got=%s", contextText)
	}
}
