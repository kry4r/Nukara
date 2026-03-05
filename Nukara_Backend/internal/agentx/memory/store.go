package memory

import (
	"context"
	"time"

	"nukara/backend/internal/store"
)

type Store struct {
	dataStore interface {
		UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error)
	}
}

func NewStore(ds interface {
	UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error)
}) *Store {
	return &Store{dataStore: ds}
}

func (s *Store) SaveFact(_ context.Context, userID, botID, content, owner string, importance int, occurredAt time.Time) (store.MemoryItem, error) {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return s.dataStore.UpsertMemoryItem(store.MemoryItem{
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
