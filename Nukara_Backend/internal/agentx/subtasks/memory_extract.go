package subtasks

import (
	"encoding/json"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type memoryItemsEnvelope struct {
	Items []struct {
		MemoryID   string   `json:"memory_id"`
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
		normalized := store.MemoryItem{
			ID:         strings.TrimSpace(item.MemoryID),
			Kind:       normalizeMemoryKind(item.Kind),
			Owner:      strings.TrimSpace(item.Owner),
			Content:    strings.TrimSpace(item.Content),
			Importance: item.Importance,
			OccurredAt: occurredAt,
			Status:     normalizeMemoryStatus(item.Status),
			Topics:     append([]string(nil), item.Topics...),
		}
		if !shouldPersistMemory(normalized.Kind, normalized.Content) {
			continue
		}
		items = append(items, normalized)
	}
	return items, nil
}

func normalizeMemoryKind(raw string) string {
	kind := strings.ToLower(strings.TrimSpace(raw))
	switch kind {
	case "event", "promise", "self_fact", "user_fact", "habit", "state_basis", "fact":
		return kind
	default:
		return "fact"
	}
}

func normalizeMemoryStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "fulfilled", "expired", "rejected", "pending_confirm", "archived", "active":
		return status
	default:
		return "active"
	}
}

func shouldPersistMemory(kind, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	for _, lowValue := range []string{"你好", "你好呀", "早安", "晚安", "吃了吗", "我有点困", "我有点忙"} {
		if content == lowValue {
			return false
		}
	}
	if kind == "user_fact" {
		for _, keyword := range []string{"考试", "下周", "明天", "记住", "讨厌", "喜欢", "别半夜", "过敏", "猫", "工作", "计划", "习惯", "固定", "散步", "作息", "下班后", "开始把"} {
			if strings.Contains(content, keyword) {
				return true
			}
		}
		return false
	}
	if kind == "self_fact" || kind == "habit" {
		for _, keyword := range []string{"总是", "最近", "习惯", "凌晨", "每周", "通常", "刚", "正在", "回去路上", "值晚班", "下班"} {
			if strings.Contains(content, keyword) {
				return true
			}
		}
		return false
	}
	return true
}
