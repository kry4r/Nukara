package memory

import (
	"context"
	"log"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type Embedder interface {
	Embed(ctx context.Context, input string) ([]float64, error)
}

type VectorIndex interface {
	Search(ctx context.Context, userID, botID, query string, vector []float64, limit int) ([]QdrantSearchResult, error)
	Upsert(ctx context.Context, points []map[string]any) error
}

type TopicGraph interface {
	ExpandTopics(ctx context.Context, seeds []string, limit int) ([]TopicEdge, error)
	WriteMemoryTopicLinks(ctx context.Context, memoryID string, topics []string) error
}

type StoreOption func(*Store)

type Store struct {
	dataStore interface {
		UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error)
		ListMemoryItems(userID, botID string, limit int) []store.MemoryItem
	}
	embedder    Embedder
	vectorIndex VectorIndex
	topicGraph  TopicGraph
}

func WithEmbedder(embedder Embedder) StoreOption {
	return func(s *Store) { s.embedder = embedder }
}

func WithVectorIndex(index VectorIndex) StoreOption {
	return func(s *Store) { s.vectorIndex = index }
}

func WithTopicGraph(graph TopicGraph) StoreOption {
	return func(s *Store) { s.topicGraph = graph }
}

func NewStore(ds interface {
	UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error)
	ListMemoryItems(userID, botID string, limit int) []store.MemoryItem
}, opts ...StoreOption) *Store {
	s := &Store{dataStore: ds}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func (s *Store) Save(ctx context.Context, item store.MemoryItem) (store.MemoryItem, error) {
	if s == nil || s.dataStore == nil {
		return store.MemoryItem{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UTC()
	item.UserID = strings.TrimSpace(item.UserID)
	item.BotID = strings.TrimSpace(item.BotID)
	item.ID = strings.TrimSpace(item.ID)
	item.Kind = firstNonEmpty(strings.TrimSpace(item.Kind), "fact")
	item.Owner = firstNonEmpty(strings.TrimSpace(item.Owner), "shared")
	item.Content = strings.TrimSpace(item.Content)
	item.Status = firstNonEmpty(strings.TrimSpace(item.Status), "active")
	item.Topics = dedupNonEmpty(item.Topics)
	if item.OccurredAt.IsZero() {
		item.OccurredAt = now
	}

	if existing, ok := s.findExisting(item); ok {
		if item.ID == "" {
			item.ID = existing.ID
		}
		if item.Importance < existing.Importance {
			item.Importance = existing.Importance
		}
		if item.OccurredAt.Before(existing.OccurredAt) {
			item.OccurredAt = existing.OccurredAt
		}
		item.Topics = dedupNonEmpty(append(existing.Topics, item.Topics...))
		if item.Content == "" {
			item.Content = existing.Content
		}
		if item.Owner == "" {
			item.Owner = existing.Owner
		}
		if item.Kind == "" {
			item.Kind = existing.Kind
		}
		if item.Status == "" {
			item.Status = existing.Status
		}
	}

	saved, err := s.dataStore.UpsertMemoryItem(item)
	if err != nil {
		return store.MemoryItem{}, err
	}
	s.syncExternal(ctx, saved)
	return saved, nil
}

func (s *Store) SaveFact(ctx context.Context, userID, botID, content, owner string, importance int, occurredAt time.Time) (store.MemoryItem, error) {
	return s.Save(ctx, store.MemoryItem{
		UserID:     userID,
		BotID:      botID,
		Kind:       "fact",
		Owner:      owner,
		Content:    content,
		Importance: importance,
		OccurredAt: occurredAt,
		Status:     "active",
	})
}

func (s *Store) syncExternal(ctx context.Context, item store.MemoryItem) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.vectorIndex != nil && s.embedder != nil && strings.TrimSpace(item.Content) != "" {
		vector, err := s.embedder.Embed(ctx, item.Content)
		if err != nil {
			log.Printf("[memory] embed memory failed: id=%s err=%v", item.ID, err)
		} else if len(vector) > 0 {
			point := map[string]any{
				"id": item.ID,
				"vector": vector,
				"payload": map[string]any{
					"user_id":     item.UserID,
					"bot_id":      item.BotID,
					"kind":        item.Kind,
					"owner":       item.Owner,
					"content":     item.Content,
					"importance":  item.Importance,
					"status":      item.Status,
					"topics":      item.Topics,
					"occurred_at": item.OccurredAt.UTC().Format(time.RFC3339),
				},
			}
			if err := s.vectorIndex.Upsert(ctx, []map[string]any{point}); err != nil {
				log.Printf("[memory] qdrant upsert failed: id=%s err=%v", item.ID, err)
			}
		}
	}
	if s.topicGraph != nil && strings.TrimSpace(item.ID) != "" && len(item.Topics) > 0 {
		if err := s.topicGraph.WriteMemoryTopicLinks(ctx, item.ID, item.Topics); err != nil {
			log.Printf("[memory] neo4j topic link failed: id=%s err=%v", item.ID, err)
		}
	}
}

func (s *Store) findExisting(item store.MemoryItem) (store.MemoryItem, bool) {
	items := s.dataStore.ListMemoryItems(item.UserID, item.BotID, 200)
	if item.ID != "" {
		for _, existing := range items {
			if strings.TrimSpace(existing.ID) == item.ID {
				return existing, true
			}
		}
	}
	needle := normalizeMemoryText(item.Content)
	if needle == "" {
		return store.MemoryItem{}, false
	}
	for _, existing := range items {
		if normalizeMemoryText(existing.Content) != needle {
			continue
		}
		if !sameOrEmpty(item.Owner, existing.Owner) {
			continue
		}
		if !sameOrEmpty(item.Kind, existing.Kind) {
			continue
		}
		return existing, true
	}
	return store.MemoryItem{}, false
}

func normalizeMemoryText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func dedupNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func sameOrEmpty(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left == "" || right == "" || strings.EqualFold(left, right)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
