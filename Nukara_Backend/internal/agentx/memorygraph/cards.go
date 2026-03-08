package memorygraph

import (
	"sort"
	"strings"

	"nukara/backend/internal/store"
)

func AssemblePromptCards(nodes []store.TemporalMemoryNode, chains []RecallChain, budget CardBudget) []PromptCard {
	maxChars := budget.MaxChars
	if maxChars <= 0 {
		maxChars = 420
	}
	maxCards := budget.MaxCards
	if maxCards <= 0 {
		maxCards = 6
	}
	byType := map[string][]store.TemporalMemoryNode{}
	for _, node := range nodes {
		nodeType := strings.ToLower(strings.TrimSpace(node.NodeType))
		byType[nodeType] = append(byType[nodeType], node)
	}
	for key := range byType {
		sort.Slice(byType[key], func(i, j int) bool {
			if !byType[key][i].OccurredAt.Equal(byType[key][j].OccurredAt) {
				return byType[key][i].OccurredAt.After(byType[key][j].OccurredAt)
			}
			return byType[key][i].UpdatedAt.After(byType[key][j].UpdatedAt)
		})
	}
	candidates := make([]PromptCard, 0, 6)
	if node := latestNode(byType["self_model"]); node != nil {
		candidates = append(candidates, PromptCard{CardType: "self_card", Text: compactCardText("自我", node.Summary, 34), BackingNodeIDs: []string{node.ID}})
	}
	if node := latestNode(byType["state_snapshot"]); node != nil {
		candidates = append(candidates, PromptCard{CardType: "state_card", Text: compactCardText("状态", node.Summary, 28), BackingNodeIDs: []string{node.ID}})
	}
	if promises := activePromises(byType["promise"]); len(promises) > 0 {
		ids := make([]string, 0, len(promises))
		parts := make([]string, 0, minInt(len(promises), 2))
		for _, node := range promises {
			ids = append(ids, node.ID)
			label := strings.TrimSpace(node.Title)
			if label == "" {
				label = strings.TrimSpace(node.Summary)
			}
			if label == "" {
				continue
			}
			parts = append(parts, label)
			if len(parts) >= 2 {
				break
			}
		}
		if len(parts) > 0 {
			candidates = append(candidates, PromptCard{CardType: "open_loops_card", Text: compactCardText("待办", strings.Join(parts, "；"), 30), BackingNodeIDs: uniqueIDs(ids)})
		}
	}
	if node := latestNode(byType["user_fact"]); node != nil {
		candidates = append(candidates, PromptCard{CardType: "user_card", Text: compactCardText("用户", node.Summary, 28), BackingNodeIDs: []string{node.ID}})
	}
	if len(chains) > 0 {
		chain := chains[0]
		parts := make([]string, 0, len(chain.Timeline))
		for _, node := range chain.Timeline {
			label := strings.TrimSpace(node.Title)
			if label == "" {
				label = strings.TrimSpace(node.Summary)
			}
			if label == "" {
				continue
			}
			parts = append(parts, label)
			if len(parts) >= 2 {
				break
			}
		}
		if len(parts) > 0 {
			candidates = append(candidates, PromptCard{CardType: "episode_chain_card", Text: compactCardText("经历", strings.Join(parts, " → "), 32), BackingNodeIDs: uniqueIDs(chain.BackingNodeIDs)})
		}
	}
	if node := latestNode(byType["session_summary"]); node != nil {
		candidates = append(candidates, PromptCard{CardType: "session_bridge_card", Text: compactCardText("衔接", node.Summary, 28), BackingNodeIDs: []string{node.ID}})
	}

	cards := make([]PromptCard, 0, len(candidates))
	remaining := maxChars
	for _, candidate := range candidates {
		if len(cards) >= maxCards || remaining <= 0 {
			break
		}
		trimmed := trimToRuneLimit(strings.TrimSpace(candidate.Text), remaining)
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == "" {
			continue
		}
		candidate.Text = trimmed
		cards = append(cards, candidate)
		remaining -= len([]rune(candidate.Text))
	}
	return cards
}

func compactCardText(prefix string, content string, limit int) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	return prefix + "：" + trimToRuneLimit(text, limit)
}

func trimToRuneLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func activePromises(nodes []store.TemporalMemoryNode) []store.TemporalMemoryNode {
	out := make([]store.TemporalMemoryNode, 0, len(nodes))
	for _, node := range nodes {
		if strings.EqualFold(strings.TrimSpace(node.Status), "resolved") {
			continue
		}
		out = append(out, node)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
