package memorygraph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

var highRiskUserFactCues = []string{
	"性", "性癖", "性偏好", "被束缚", "床上", "自慰", "隐私", "身份证", "住址", "银行卡", "宗教", "政治",
	"我是男", "我是女", "同性恋", "双性恋", "跨性别", "老公", "老婆", "男朋友", "女朋友",
}

type compactEnvelope struct {
	Summary string   `json:"summary"`
	Facts   []string `json:"facts"`
}

func MaterializeMemoryNodes(in IngestTurnInput, now time.Time) []store.TemporalMemoryNode {
	out := make([]store.TemporalMemoryNode, 0, len(in.Items))
	for idx, item := range in.Items {
		node, ok := materializeMemoryNode(in, item, idx, now)
		if !ok {
			continue
		}
		out = append(out, node)
	}
	return out
}

func BuildSessionSummaryNode(in IngestTurnInput, now time.Time) *store.TemporalMemoryNode {
	raw := strings.TrimSpace(in.CompactJSON)
	if raw == "" {
		return nil
	}
	var payload compactEnvelope
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	summary := strings.TrimSpace(payload.Summary)
	if summary == "" && len(payload.Facts) == 0 {
		return nil
	}
	body, _ := json.Marshal(payload)
	occurredAt := now
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	node := store.TemporalMemoryNode{
		ID:           sessionSummaryNodeID(in.ConversationID),
		UserID:       strings.TrimSpace(in.UserID),
		BotID:        strings.TrimSpace(in.BotID),
		SessionID:    strings.TrimSpace(in.ConversationID),
		NodeType:     "session_summary",
		Title:        "会话摘要",
		Summary:      buildSessionSummaryText(summary, payload.Facts),
		BodyJSON:     string(body),
		Status:       "active",
		OccurredAt:   occurredAt,
		ObservedAt:   now,
		ValidFrom:    occurredAt,
		SourceTurnID: strings.TrimSpace(in.TurnID),
		Salience:     0.66,
		Confidence:   0.82,
		Stability:    0.74,
	}
	return &node
}

func BuildTurnEdges(nodes []store.TemporalMemoryNode) []store.TemporalMemoryEdge {
	if len(nodes) < 2 {
		return nil
	}
	sorted := append([]store.TemporalMemoryNode(nil), nodes...)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].OccurredAt.Equal(sorted[j].OccurredAt) {
			return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
		}
		return sorted[i].ID < sorted[j].ID
	})
	edges := make([]store.TemporalMemoryEdge, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		edges = append(edges, store.TemporalMemoryEdge{
			ID:            fmt.Sprintf("edge:%s:%s", sorted[i-1].ID, sorted[i].ID),
			SourceID:      sorted[i].ID,
			TargetID:      sorted[i-1].ID,
			EdgeType:      "follows",
			Weight:        0.72,
			EvidenceCount: 1,
			Status:        "active",
		})
	}
	return edges
}

func BuildSessionSummaryEdges(summary store.TemporalMemoryNode, nodes []store.TemporalMemoryNode) []store.TemporalMemoryEdge {
	edges := make([]store.TemporalMemoryEdge, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			continue
		}
		edges = append(edges, store.TemporalMemoryEdge{
			ID:            fmt.Sprintf("edge:%s:%s", summary.ID, node.ID),
			SourceID:      summary.ID,
			TargetID:      node.ID,
			EdgeType:      "summarizes",
			Weight:        0.8,
			EvidenceCount: 1,
			Status:        "active",
		})
	}
	return edges
}

func normalizedText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nodeCorpus(node store.TemporalMemoryNode) string {
	return normalizedText(strings.TrimSpace(node.Title + " " + node.Summary))
}

func lexicalHits(text string, hints []string) int {
	text = normalizedText(text)
	if text == "" {
		return 0
	}
	hits := 0
	seen := map[string]struct{}{}
	for _, hint := range hints {
		hint = normalizedText(hint)
		if hint == "" {
			continue
		}
		if _, ok := seen[hint]; ok {
			continue
		}
		seen[hint] = struct{}{}
		if strings.Contains(text, hint) {
			hits++
		}
	}
	return hits
}

func uniqueIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func sortNodesByOccurredAsc(nodes []store.TemporalMemoryNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if !nodes[i].OccurredAt.Equal(nodes[j].OccurredAt) {
			return nodes[i].OccurredAt.Before(nodes[j].OccurredAt)
		}
		return nodes[i].UpdatedAt.Before(nodes[j].UpdatedAt)
	})
}

func latestNode(nodes []store.TemporalMemoryNode) *store.TemporalMemoryNode {
	if len(nodes) == 0 {
		return nil
	}
	best := nodes[0]
	for _, node := range nodes[1:] {
		if node.OccurredAt.After(best.OccurredAt) || (node.OccurredAt.Equal(best.OccurredAt) && node.UpdatedAt.After(best.UpdatedAt)) {
			best = node
		}
	}
	return &best
}

func effectiveNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func materializeMemoryNode(in IngestTurnInput, item store.MemoryItem, idx int, now time.Time) (store.TemporalMemoryNode, bool) {
	content := strings.TrimSpace(item.Content)
	if content == "" {
		return store.TemporalMemoryNode{}, false
	}
	nodeType := mapMemoryKindToNodeType(item.Kind)
	if nodeType == "" {
		return store.TemporalMemoryNode{}, false
	}
	summary := content
	if item.Kind == "user_fact" && isHighRiskUserFact(content) {
		nodeType = "episode"
		summary = "用户明确提到：" + content
	}
	occurredAt := item.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}
	status := strings.TrimSpace(item.Status)
	if status == "" {
		status = "active"
	}
	node := store.TemporalMemoryNode{
		ID:           materializedNodeID(in.TurnID, item.ID, nodeType, idx),
		UserID:       strings.TrimSpace(in.UserID),
		BotID:        strings.TrimSpace(in.BotID),
		SessionID:    strings.TrimSpace(in.ConversationID),
		NodeType:     nodeType,
		Title:        deriveNodeTitle(nodeType, summary),
		Summary:      summary,
		Status:       status,
		OccurredAt:   occurredAt,
		ObservedAt:   now,
		ValidFrom:    occurredAt,
		SourceTurnID: strings.TrimSpace(in.TurnID),
		Salience:     importanceScore(item.Importance),
		Confidence:   0.72,
		Stability:    initialStability(nodeType),
	}
	return node, true
}

func mapMemoryKindToNodeType(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "event", "fact", "self_fact", "habit":
		return "episode"
	case "promise":
		return "promise"
	case "state_basis":
		return "state_snapshot"
	case "user_fact":
		return "user_fact"
	default:
		return ""
	}
}

func isHighRiskUserFact(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	for _, cue := range highRiskUserFactCues {
		if cue != "" && strings.Contains(content, cue) {
			return true
		}
	}
	return false
}

func deriveNodeTitle(nodeType, content string) string {
	prefix := map[string]string{
		"episode":         "经历",
		"promise":         "待办",
		"state_snapshot":  "状态",
		"user_fact":       "用户信息",
		"session_summary": "会话摘要",
	}[nodeType]
	trimmed := trimToRuneLimit(strings.TrimSpace(content), 14)
	if prefix == "" {
		return trimmed
	}
	if trimmed == "" {
		return prefix
	}
	return prefix + "：" + trimmed
}

func buildSessionSummaryText(summary string, facts []string) string {
	summary = strings.TrimSpace(summary)
	parts := make([]string, 0, 2)
	if summary != "" {
		parts = append(parts, summary)
	}
	factParts := make([]string, 0, minInt(len(facts), 2))
	for _, fact := range facts {
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		factParts = append(factParts, fact)
		if len(factParts) >= 2 {
			break
		}
	}
	if len(factParts) > 0 {
		parts = append(parts, "要点："+strings.Join(factParts, "；"))
	}
	return strings.Join(parts, "；")
}

func materializedNodeID(turnID, memoryID, nodeType string, idx int) string {
	turnID = strings.TrimSpace(turnID)
	memoryID = strings.TrimSpace(memoryID)
	nodeType = strings.TrimSpace(nodeType)
	if memoryID != "" {
		return fmt.Sprintf("tm:%s:%s:%s", turnID, nodeType, memoryID)
	}
	if turnID != "" {
		return fmt.Sprintf("tm:%s:%s:%d", turnID, nodeType, idx)
	}
	return store.NewID()
}

func sessionSummaryNodeID(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return store.NewID()
	}
	return "session-summary:" + conversationID
}

func importanceScore(value int) float64 {
	if value <= 0 {
		return 0.45
	}
	if value >= 100 {
		return 1
	}
	return float64(value) / 100
}

func initialStability(nodeType string) float64 {
	switch nodeType {
	case "state_snapshot":
		return 0.3
	case "promise":
		return 0.8
	case "user_fact":
		return 0.7
	default:
		return 0.55
	}
}
