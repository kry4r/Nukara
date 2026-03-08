package memorygraph

import (
	"fmt"
	"sort"
	"strings"

	"nukara/backend/internal/store"
)

func PromoteRepeatedEpisodesToHabit(nodes []store.TemporalMemoryNode, opts ConsolidationOptions) ConsolidationResult {
	minOccurrences := opts.MinOccurrences
	if minOccurrences <= 0 {
		minOccurrences = 3
	}
	now := effectiveNow(opts.Now)
	type motifGroup struct {
		key   string
		nodes []store.TemporalMemoryNode
	}
	groups := map[string][]store.TemporalMemoryNode{}
	for _, node := range nodes {
		if !strings.EqualFold(strings.TrimSpace(node.NodeType), "episode") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(node.Status), "active") {
			continue
		}
		key := canonicalMotif(node)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], node)
	}
	candidates := make([]motifGroup, 0, len(groups))
	for key, grouped := range groups {
		if len(grouped) < minOccurrences {
			continue
		}
		sortNodesByOccurredAsc(grouped)
		candidates = append(candidates, motifGroup{key: key, nodes: grouped})
	}
	if len(candidates) == 0 {
		return ConsolidationResult{}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i].nodes) != len(candidates[j].nodes) {
			return len(candidates[i].nodes) > len(candidates[j].nodes)
		}
		left := candidates[i].nodes[len(candidates[i].nodes)-1].OccurredAt
		right := candidates[j].nodes[len(candidates[j].nodes)-1].OccurredAt
		return left.After(right)
	})
	best := candidates[0]
	latest := best.nodes[len(best.nodes)-1]
	backingIDs := make([]string, 0, len(best.nodes))
	for _, node := range best.nodes {
		backingIDs = append(backingIDs, node.ID)
	}
	habit := store.TemporalMemoryNode{
		UserID:     latest.UserID,
		BotID:      latest.BotID,
		SessionID:  latest.SessionID,
		NodeType:   "habit",
		Title:      best.key,
		Summary:    fmt.Sprintf("最近已连续 %d 次出现“%s”，可视为稳定习惯。", len(best.nodes), best.key),
		Status:     "active",
		OccurredAt: latest.OccurredAt,
		ObservedAt: now,
		ValidFrom:  best.nodes[0].OccurredAt,
		Salience:   0.72,
		Stability:  0.8,
		Confidence: 0.75,
	}
	return ConsolidationResult{PromotedHabit: &habit, BackingNodeIDs: backingIDs}
}

func canonicalMotif(node store.TemporalMemoryNode) string {
	if title := strings.TrimSpace(node.Title); title != "" {
		return title
	}
	summary := []rune(strings.TrimSpace(node.Summary))
	if len(summary) == 0 {
		return ""
	}
	if len(summary) > 12 {
		summary = summary[:12]
	}
	return string(summary)
}
