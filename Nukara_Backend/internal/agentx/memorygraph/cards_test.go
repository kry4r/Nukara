package memorygraph

import (
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestCards_AssembleFixedBudgetPromptCards(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	selfNode := store.TemporalMemoryNode{ID: "self", NodeType: "self_model", Summary: "我最近更把凌晨散步当作一种稳定的收束方式。", Status: "active", OccurredAt: now.Add(-6 * time.Hour)}
	stateNode := store.TemporalMemoryNode{ID: "state", NodeType: "state_snapshot", Summary: "今天白天忙，但情绪平稳。", Status: "active", OccurredAt: now.Add(-2 * time.Hour)}
	promiseNode := store.TemporalMemoryNode{ID: "promise", NodeType: "promise", Summary: "还记着要报名摄影课。", Status: "active", OccurredAt: now.Add(-24 * time.Hour)}
	userNode := store.TemporalMemoryNode{ID: "user", NodeType: "user_fact", Summary: "用户最近在重新整理下班后的生活节奏。", Status: "active", OccurredAt: now.Add(-4 * time.Hour)}
	oldEpisode := store.TemporalMemoryNode{ID: "episode-old", NodeType: "episode", Summary: "前晚散步时想清楚了最近为什么总想晚点回家。", Status: "active", OccurredAt: now.Add(-30 * time.Hour)}
	recentEpisode := store.TemporalMemoryNode{ID: "episode-recent", NodeType: "episode", Summary: "昨晚散步后决定这周先把摄影课报上。", Status: "active", OccurredAt: now.Add(-3 * time.Hour)}

	chains := []RecallChain{{
		Anchor:         promiseNode,
		Timeline:       []store.TemporalMemoryNode{oldEpisode, recentEpisode},
		WhyRelevant:    "散步与待办之间形成了连续的自我提醒。",
		ChainType:      "recent_life",
		BackingNodeIDs: []string{oldEpisode.ID, recentEpisode.ID, promiseNode.ID},
	}}

	cards := AssemblePromptCards(
		[]store.TemporalMemoryNode{selfNode, stateNode, promiseNode, userNode, oldEpisode, recentEpisode},
		chains,
		CardBudget{MaxChars: 120, MaxCards: 4},
	)

	if len(cards) == 0 {
		t.Fatal("expected prompt cards")
	}
	if total := totalCardChars(cards); total > 120 {
		t.Fatalf("expected cards to stay within budget, got %d chars", total)
	}
	if !hasCardType(cards, "self_card") {
		t.Fatal("expected self_card to be included")
	}
	if !hasCardType(cards, "open_loops_card") {
		t.Fatal("expected open_loops_card to be included")
	}
	if len(cards) > 4 {
		t.Fatalf("expected card count to stay bounded, got %d", len(cards))
	}
	for _, card := range cards {
		if strings.TrimSpace(card.Text) == "" {
			t.Fatal("expected card text to be non-empty")
		}
	}
}

func totalCardChars(cards []PromptCard) int {
	total := 0
	for _, card := range cards {
		total += len([]rune(card.Text))
	}
	return total
}

func hasCardType(cards []PromptCard, want string) bool {
	for _, card := range cards {
		if card.CardType == want {
			return true
		}
	}
	return false
}
