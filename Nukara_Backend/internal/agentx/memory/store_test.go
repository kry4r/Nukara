package memory

import (
	"context"
	"testing"

	"nukara/backend/internal/store"
)

func TestStoreSaveUpdatesExistingMemoryByID(t *testing.T) {
	st := store.NewStore()
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		ID:         "mem-drink-1",
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户喜欢乌龙茶",
		Importance: 70,
		Status:     "active",
		Topics:     []string{"乌龙茶", "偏好"},
	}); err != nil {
		t.Fatalf("seed memory failed: %v", err)
	}

	tool := NewStore(st)
	item, err := tool.Save(context.Background(), store.MemoryItem{
		ID:         "mem-drink-1",
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户现在更喜欢绿茶，不太喝乌龙茶",
		Importance: 88,
		Status:     "active",
		Topics:     []string{"绿茶", "乌龙茶", "偏好"},
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if item.ID != "mem-drink-1" {
		t.Fatalf("memory id = %s, want mem-drink-1", item.ID)
	}

	items := st.ListMemoryItems("user-1", "bot-1", 10)
	if len(items) != 1 {
		t.Fatalf("memory items = %d, want 1", len(items))
	}
	if items[0].Content != "用户现在更喜欢绿茶，不太喝乌龙茶" {
		t.Fatalf("memory content = %q", items[0].Content)
	}
}

func TestStoreSaveDeduplicatesExactContent(t *testing.T) {
	st := store.NewStore()
	tool := NewStore(st)

	first, err := tool.Save(context.Background(), store.MemoryItem{
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户喜欢喝茶",
		Importance: 65,
		Status:     "active",
		Topics:     []string{"喝茶"},
	})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	second, err := tool.Save(context.Background(), store.MemoryItem{
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户喜欢喝茶",
		Importance: 90,
		Status:     "active",
		Topics:     []string{"茶", "喝茶"},
	})
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate content should reuse id, got %s and %s", first.ID, second.ID)
	}

	items := st.ListMemoryItems("user-1", "bot-1", 10)
	if len(items) != 1 {
		t.Fatalf("memory items = %d, want 1", len(items))
	}
	if items[0].Importance != 90 {
		t.Fatalf("memory importance = %d, want 90", items[0].Importance)
	}
	if len(items[0].Topics) != 2 {
		t.Fatalf("memory topics = %v, want 2 merged topics", items[0].Topics)
	}
}

type fakeEmbedder struct {
	inputs []string
	vector []float64
}

func (f *fakeEmbedder) Embed(_ context.Context, input string) ([]float64, error) {
	f.inputs = append(f.inputs, input)
	return append([]float64(nil), f.vector...), nil
}

type fakeVectorIndex struct {
	points       []map[string]any
	searchVector []float64
	searchItems  []QdrantSearchResult
}

func (f *fakeVectorIndex) Upsert(_ context.Context, points []map[string]any) error {
	f.points = append(f.points, points...)
	return nil
}

func (f *fakeVectorIndex) Search(_ context.Context, _ string, _ string, _ string, vector []float64, _ int) ([]QdrantSearchResult, error) {
	f.searchVector = append([]float64(nil), vector...)
	return append([]QdrantSearchResult(nil), f.searchItems...), nil
}

type fakeTopicGraph struct {
	memoryID string
	topics   []string
}

func (f *fakeTopicGraph) ExpandTopics(_ context.Context, _ []string, _ int) ([]TopicEdge, error) {
	return nil, nil
}

func (f *fakeTopicGraph) WriteMemoryTopicLinks(_ context.Context, memoryID string, topics []string) error {
	f.memoryID = memoryID
	f.topics = append([]string(nil), topics...)
	return nil
}

func TestStoreSaveSyncsToVectorIndexAndTopicGraph(t *testing.T) {
	st := store.NewStore()
	embedder := &fakeEmbedder{vector: []float64{0.1, 0.2, 0.3}}
	vectorIndex := &fakeVectorIndex{}
	topicGraph := &fakeTopicGraph{}
	tool := NewStore(st, WithEmbedder(embedder), WithVectorIndex(vectorIndex), WithTopicGraph(topicGraph))

	item, err := tool.Save(context.Background(), store.MemoryItem{
		UserID:     "user-1",
		BotID:      "bot-1",
		Kind:       "fact",
		Owner:      "user",
		Content:    "用户最喜欢周末去海边散步",
		Importance: 86,
		Status:     "active",
		Topics:     []string{"海边", "散步", "周末"},
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if len(embedder.inputs) != 1 || embedder.inputs[0] != "用户最喜欢周末去海边散步" {
		t.Fatalf("embed inputs = %v", embedder.inputs)
	}
	if len(vectorIndex.points) != 1 {
		t.Fatalf("vector upsert points = %d, want 1", len(vectorIndex.points))
	}
	if topicGraph.memoryID != item.ID {
		t.Fatalf("topic graph memory id = %s, want %s", topicGraph.memoryID, item.ID)
	}
	if len(topicGraph.topics) != 3 {
		t.Fatalf("topic graph topics = %v", topicGraph.topics)
	}
}
