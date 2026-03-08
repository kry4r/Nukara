package subtasks

import (
	"encoding/json"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type memoryItemsEnvelope struct {
	Items []struct {
		MemoryID         string         `json:"memory_id"`
		Kind             string         `json:"kind"`
		Owner            string         `json:"owner"`
		Content          string         `json:"content"`
		Importance       int            `json:"importance"`
		OccurredAt       string         `json:"occurred_at"`
		Status           string         `json:"status"`
		Topics           []string       `json:"topics"`
		SemanticCategory string         `json:"semantic_category"`
		Stability        string         `json:"stability"`
		Entities         []store.Entity `json:"entities"`
		Relations        []struct {
			SourceEntityID string `json:"source_entity_id"`
			TargetEntityID string `json:"target_entity_id"`
			RelationType   string `json:"relation_type"`
			RoleHint       string `json:"role_hint"`
		} `json:"relations"`
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
		relations := make([]store.Relation, 0, len(item.Relations))
		for _, relation := range item.Relations {
			relations = append(relations, store.Relation{
				SourceEntityID: strings.TrimSpace(relation.SourceEntityID),
				TargetEntityID: strings.TrimSpace(relation.TargetEntityID),
				RelationType:   strings.TrimSpace(relation.RelationType),
				RoleHint:       strings.TrimSpace(relation.RoleHint),
			})
		}
		content := strings.TrimSpace(item.Content)
		semanticCategory := normalizeSemanticCategory(item.SemanticCategory, content, item.Kind)
		stability := normalizeMemoryStability(item.Stability, content)
		normalized := store.MemoryItem{
			ID:               strings.TrimSpace(item.MemoryID),
			Kind:             normalizeMemoryKind(item.Kind),
			Owner:            strings.TrimSpace(item.Owner),
			Content:          content,
			Importance:       item.Importance,
			OccurredAt:       occurredAt,
			Status:           normalizeMemoryStatus(item.Status),
			Topics:           append([]string(nil), item.Topics...),
			SemanticCategory: semanticCategory,
			Stability:        stability,
			Entities:         append([]store.Entity(nil), item.Entities...),
			Relations:        relations,
		}
		if normalized.Importance <= 0 {
			normalized.Importance = inferMemoryImportance(normalized.Kind, normalized.SemanticCategory, normalized.Content)
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
	case "fulfilled", "expired", "rejected", "archived", "active":
		return status
	default:
		return "active"
	}
}

func normalizeSemanticCategory(raw, content, kind string) string {
	category := strings.ToLower(strings.TrimSpace(raw))
	switch category {
	case "identity", "personality", "life_context", "expression_style", "taboos_and_preferences", "relationship":
		return category
	}
	content = strings.TrimSpace(content)
	kind = normalizeMemoryKind(kind)
	switch {
	case strings.Contains(content, "专业") || strings.Contains(content, "研究型") || strings.Contains(content, "不是纯金融"):
		return "identity"
	case strings.Contains(content, "猫") || strings.Contains(content, "爸妈") || strings.Contains(content, "父母") || strings.Contains(content, "住在"):
		return "life_context"
	case kind == "habit":
		return "expression_style"
	case kind == "user_fact":
		return "relationship"
	default:
		return "life_context"
	}
}

func normalizeMemoryStability(raw, content string) string {
	stability := strings.ToLower(strings.TrimSpace(raw))
	switch stability {
	case "stable", "temporary":
		return stability
	}
	for _, cue := range []string{"最近", "暂时", "这周", "今天", "刚", "刚刚", "目前", "这阵子"} {
		if strings.Contains(content, cue) {
			return "temporary"
		}
	}
	return "stable"
}

func inferMemoryImportance(kind, semanticCategory, content string) int {
	switch {
	case semanticCategory == "identity":
		return 85
	case semanticCategory == "taboos_and_preferences":
		return 82
	case kind == "promise":
		return 80
	case strings.Contains(content, "猫") || strings.Contains(content, "爸妈"):
		return 78
	default:
		return 70
	}
}

func shouldPersistMemory(kind, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	for _, lowValue := range []string{"你好", "你好呀", "早安", "晚安", "吃了吗", "我有点困", "我有点忙", "嗯", "哦", "好的", "知道了"} {
		if content == lowValue {
			return false
		}
	}
	if kind == "habit" {
		return len([]rune(content)) >= 4
	}
	return true
}
