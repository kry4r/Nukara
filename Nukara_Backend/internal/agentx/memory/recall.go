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
	Qdrant *QdrantClient
	Neo4j  *Neo4jClient
}

type RecallBuilder struct {
	qdrant *QdrantClient
	neo4j  *Neo4jClient
}

func NewRecallBuilder(deps RecallDeps) *RecallBuilder {
	return &RecallBuilder{
		qdrant: deps.Qdrant,
		neo4j:  deps.Neo4j,
	}
}

func (b *RecallBuilder) Build(ctx context.Context, in RecallInput) ([]store.MemoryItem, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 6
	}

	qdrantItems, err := b.qdrant.Search(ctx, in.UserID, in.BotID, in.QueryText, limit)
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
			Kind:       "fact",
			Owner:      "shared",
			Content:    content,
			Importance: 50,
			OccurredAt: time.Now().UTC(),
			Status:     "active",
			Topics:     append([]string(nil), item.Topics...),
		})
	}

	if in.WithExpand && b.neo4j != nil && len(seedTopics) > 0 {
		topicEdges, expandErr := b.neo4j.ExpandTopics(ctx, dedup(seedTopics), limit)
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
