package memory

import (
	"encoding/json"

	"nukara/backend/internal/store"
)

type CompactService struct {
	dataStore interface {
		UpsertCompact(conversationID, compactJSON, untilTurnID string) error
	}
}

func NewCompactService(ds interface {
	UpsertCompact(conversationID, compactJSON, untilTurnID string) error
}) *CompactService {
	return &CompactService{dataStore: ds}
}

func (c *CompactService) Upsert(conversationID, compactJSON, untilTurnID string) error {
	return c.dataStore.UpsertCompact(conversationID, compactJSON, untilTurnID)
}

func BuildCompactPayload(summary string, keyFacts []store.MemoryItem) string {
	payload := map[string]any{
		"summary": summary,
		"facts":   keyFacts,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return `{"summary":"","facts":[]}`
	}
	return string(raw)
}
