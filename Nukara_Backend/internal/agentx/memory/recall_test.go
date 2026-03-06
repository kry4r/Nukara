package memory

import (
	"context"
	"testing"
)

func TestRecallBuilderEmbedsQueryThenUsesVectorIndexAndTopicExpansion(t *testing.T) {
	embedder := &fakeEmbedder{vector: []float64{0.9, 0.1, 0.4}}
	vectorIndex := &fakeVectorIndex{searchItems: []QdrantSearchResult{{
		ID:         "mem-1",
		Content:    "用户喜欢咖啡",
		Topics:     []string{"咖啡"},
		Owner:      "user",
		Kind:       "fact",
		Importance: 82,
	}}}
	topicGraph := &recallTopicGraph{}

	builder := NewRecallBuilder(RecallDeps{
		Embedder:    embedder,
		VectorIndex: vectorIndex,
		TopicGraph:  topicGraph,
	})

	items, err := builder.Build(context.Background(), RecallInput{
		UserID:     "u1",
		BotID:      "b1",
		QueryText:  "你记得我喜欢喝什么吗",
		Limit:      4,
		WithExpand: true,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(embedder.inputs) != 1 || embedder.inputs[0] != "你记得我喜欢喝什么吗" {
		t.Fatalf("embed inputs = %v", embedder.inputs)
	}
	if len(vectorIndex.searchVector) != 3 {
		t.Fatalf("search vector = %v", vectorIndex.searchVector)
	}
	if len(items) < 2 {
		t.Fatalf("expected qdrant item + expanded topic, got=%d", len(items))
	}
	if items[0].Content != "用户喜欢咖啡" {
		t.Fatalf("first recalled item = %q", items[0].Content)
	}
	if items[1].Content != "相关主题：咖啡豆" {
		t.Fatalf("expanded topic item = %q", items[1].Content)
	}
}

type recallTopicGraph struct{}

func (recallTopicGraph) ExpandTopics(_ context.Context, seeds []string, limit int) ([]TopicEdge, error) {
	if len(seeds) != 1 || seeds[0] != "咖啡" {
		return nil, nil
	}
	return []TopicEdge{{Name: "咖啡豆", Weight: 0.8}}, nil
}

func (recallTopicGraph) WriteMemoryTopicLinks(_ context.Context, _ string, _ []string) error {
	return nil
}
