package memorygraph

import (
	"time"

	"nukara/backend/internal/store"
)

type Cue struct {
	QueryText   string
	TimeHints   []string
	EntityHints []string
}

type ActivationOptions struct {
	Limit    int
	MaxDepth int
	Now      time.Time
}

type ActivatedNode struct {
	Node   store.TemporalMemoryNode
	Score  float64
	Depth  int
	Reason string
}

type RecallChain struct {
	Anchor         store.TemporalMemoryNode
	Timeline       []store.TemporalMemoryNode
	WhyRelevant    string
	ChainType      string
	BackingNodeIDs []string
}

type PromptCard struct {
	CardType       string
	Text           string
	BackingNodeIDs []string
}

type CardBudget struct {
	MaxChars int
	MaxCards int
}

type IngestTurnInput struct {
	UserID         string
	BotID          string
	ConversationID string
	TurnID         string
	Items          []store.MemoryItem
	CompactJSON    string
	Now            time.Time
}

type IngestTurnResult struct {
	Nodes          []store.TemporalMemoryNode
	Edges          []store.TemporalMemoryEdge
	SessionSummary *store.TemporalMemoryNode
}

type RecallInput struct {
	UserID          string
	BotID           string
	ConversationID  string
	TurnID          string
	QueryText       string
	RecentTexts     []string
	ActivationLimit int
	MaxDepth        int
	CardBudget      CardBudget
	Now             time.Time
}

type RecallResult struct {
	Cue       Cue
	Seeds     []store.TemporalMemoryNode
	Activated []ActivatedNode
	Chains    []RecallChain
	Cards     []PromptCard
	Trace     store.ActivationTrace
}

type ConsolidateInput struct {
	UserID         string
	BotID          string
	ConversationID string
	Now            time.Time
	MinOccurrences int
}

type ConsolidationOptions struct {
	Now            time.Time
	MinOccurrences int
}

type ConsolidationResult struct {
	PromotedHabit  *store.TemporalMemoryNode
	BackingNodeIDs []string
}
