package subtasks

import (
	"encoding/json"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type memoryItemsEnvelope struct {
	Items []struct {
		Kind       string   `json:"kind"`
		Owner      string   `json:"owner"`
		Content    string   `json:"content"`
		Importance int      `json:"importance"`
		OccurredAt string   `json:"occurred_at"`
		Status     string   `json:"status"`
		Topics     []string `json:"topics"`
	} `json:"items"`
}

func ParseMemoryItems(raw string) ([]store.MemoryItem, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var payload memoryItemsEnvelope
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	items := make([]store.MemoryItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		occurredAt := time.Time{}
		if strings.TrimSpace(item.OccurredAt) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(item.OccurredAt))
			if err == nil {
				occurredAt = parsed
			}
		}
		items = append(items, store.MemoryItem{
			Kind:       strings.TrimSpace(item.Kind),
			Owner:      strings.TrimSpace(item.Owner),
			Content:    strings.TrimSpace(item.Content),
			Importance: item.Importance,
			OccurredAt: occurredAt,
			Status:     strings.TrimSpace(item.Status),
			Topics:     append([]string(nil), item.Topics...),
		})
	}
	return items, nil
}
