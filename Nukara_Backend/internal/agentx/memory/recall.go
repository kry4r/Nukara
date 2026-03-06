package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type RecallInput struct {
	UserID     string
	BotID      string
	QueryText  string
	Limit      int
	WithExpand bool
}

type RecallDeps struct {
	Embedder    Embedder
	VectorIndex VectorIndex
	TopicGraph  TopicGraph
}

type RecallBuilder struct {
	embedder    Embedder
	vectorIndex VectorIndex
	topicGraph  TopicGraph
}

func NewRecallBuilder(deps RecallDeps) *RecallBuilder {
	return &RecallBuilder{
		embedder:    deps.Embedder,
		vectorIndex: deps.VectorIndex,
		topicGraph:  deps.TopicGraph,
	}
}

func (b *RecallBuilder) Build(ctx context.Context, in RecallInput) ([]store.MemoryItem, error) {
	if b == nil || b.vectorIndex == nil || b.embedder == nil {
		return nil, nil
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 6
	}
	vector, err := b.embedder.Embed(ctx, in.QueryText)
	if err != nil {
		return nil, err
	}
	if len(vector) == 0 {
		return nil, nil
	}

	qdrantItems, err := b.vectorIndex.Search(ctx, in.UserID, in.BotID, in.QueryText, vector, limit)
	if err != nil {
		return nil, err
	}

	out := make([]store.MemoryItem, 0, limit)
	seen := map[string]struct{}{}
	seedTopics := make([]string, 0, len(qdrantItems))

	for _, item := range qdrantItems {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		if _, ok := seen[content]; ok {
			continue
		}
		seen[content] = struct{}{}
		seedTopics = append(seedTopics, item.Topics...)
		out = append(out, store.MemoryItem{
			ID:         item.ID,
			UserID:     in.UserID,
			BotID:      in.BotID,
			Kind:       firstNonEmpty(item.Kind, "fact"),
			Owner:      firstNonEmpty(item.Owner, "shared"),
			Content:    content,
			Importance: maxInt(item.Importance, 50),
			OccurredAt: time.Now().UTC(),
			Status:     firstNonEmpty(item.Status, "active"),
			Topics:     append([]string(nil), item.Topics...),
		})
	}

	if in.WithExpand && b.topicGraph != nil && len(seedTopics) > 0 {
		topicEdges, expandErr := b.topicGraph.ExpandTopics(ctx, dedup(seedTopics), limit)
		if expandErr != nil {
			return out, nil
		}
		sort.Slice(topicEdges, func(i, j int) bool {
			return topicEdges[i].Weight > topicEdges[j].Weight
		})
		for _, edge := range topicEdges {
			content := "相关主题：" + edge.Name
			if _, ok := seen[content]; ok {
				continue
			}
			seen[content] = struct{}{}
			out = append(out, store.MemoryItem{
				ID:         "topic-" + edge.Name,
				UserID:     in.UserID,
				BotID:      in.BotID,
				Kind:       "topic",
				Owner:      "shared",
				Content:    content,
				Importance: int(edge.Weight * 100),
				OccurredAt: time.Now().UTC(),
				Status:     "active",
				Topics:     []string{edge.Name},
			})
			if len(out) >= limit {
				break
			}
		}
	}

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func dedup(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func maxInt(values ...int) int {
	best := 0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return best
}
