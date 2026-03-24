package memorygraph

import (
	"context"
	"strings"

	"nukara/backend/internal/store"
)

type GraphStore interface {
	CreateMemoryNode(node store.TemporalMemoryNode) (store.TemporalMemoryNode, error)
	UpdateMemoryNode(node store.TemporalMemoryNode) (store.TemporalMemoryNode, error)
	GetMemoryNode(nodeID string) (store.TemporalMemoryNode, bool)
	ListMemoryNodes(userID, botID string, filter store.TemporalMemoryNodeFilter) []store.TemporalMemoryNode
	CreateMemoryEdge(edge store.TemporalMemoryEdge) (store.TemporalMemoryEdge, error)
	ListMemoryEdges(nodeIDs []string, filter store.TemporalMemoryEdgeFilter) []store.TemporalMemoryEdge
	UpsertMemoryCard(card store.MemoryCard) (store.MemoryCard, error)
	ListMemoryCards(userID, botID string, filter store.MemoryCardFilter) []store.MemoryCard
	SaveActivationTrace(trace store.ActivationTrace) (store.ActivationTrace, error)
}

type ServiceDeps struct {
	Store            GraphStore
	EmbeddingService EmbeddingService
}

type Service struct {
	store            GraphStore
	embeddingService EmbeddingService
}

func NewService(deps ServiceDeps) *Service {
	return &Service{
		store:            deps.Store,
		embeddingService: deps.EmbeddingService,
	}
}

func (s *Service) IngestTurn(_ context.Context, in IngestTurnInput) (IngestTurnResult, error) {
	result := IngestTurnResult{}
	if s == nil || s.store == nil {
		return result, nil
	}
	now := effectiveNow(in.Now)
	itemNodes := MaterializeMemoryNodes(in, now)
	summaryFactNodes := BuildSessionSummaryFactNodes(in, now)
	persistedItemNodes, err := s.persistNodesWithMerge(itemNodes)
	if err != nil {
		return result, err
	}
	persistedSummaryFactNodes, err := s.persistNodesWithMerge(summaryFactNodes)
	if err != nil {
		return result, err
	}
	allNodes := append([]store.TemporalMemoryNode(nil), persistedItemNodes...)
	allNodes = append(allNodes, persistedSummaryFactNodes...)
	edges := BuildTurnEdges(allNodes)
	persistedEdges := make([]store.TemporalMemoryEdge, 0, len(edges))
	for _, edge := range edges {
		created, err := s.store.CreateMemoryEdge(edge)
		if err != nil {
			return result, err
		}
		persistedEdges = append(persistedEdges, created)
	}
	result.Nodes = append(result.Nodes, allNodes...)
	result.Edges = persistedEdges
	if summaryNode := BuildSessionSummaryNode(in, now); summaryNode != nil {
		storedNode, err := s.upsertSessionSummary(*summaryNode)
		if err != nil {
			return result, err
		}
		result.SessionSummary = &storedNode
		result.Nodes = append(result.Nodes, storedNode)
		summaryEdges := BuildSessionSummaryEdges(storedNode, persistedSummaryFactNodes)
		for _, edge := range summaryEdges {
			created, err := s.store.CreateMemoryEdge(edge)
			if err != nil {
				return result, err
			}
			result.Edges = append(result.Edges, created)
		}
	}

	// 异步生成 embeddings
	if s.embeddingService != nil {
		for _, node := range result.Nodes {
			go s.generateEmbedding(context.Background(), node.ID, node.Summary)
		}
	}

	return result, nil
}

func (s *Service) Recall(_ context.Context, in RecallInput) (RecallResult, error) {
	result := RecallResult{}
	if s == nil || s.store == nil {
		return result, nil
	}
	now := effectiveNow(in.Now)
	allNodes := s.store.ListMemoryNodes(in.UserID, in.BotID, store.TemporalMemoryNodeFilter{Status: "active", Limit: 128})
	if len(allNodes) == 0 {
		return result, nil
	}
	cue := BuildCue(in.QueryText, in.RecentTexts)
	seedLimit := in.ActivationLimit
	if seedLimit <= 0 {
		seedLimit = 6
	}
	seeds := SelectSeeds(cue, allNodes, SeedOptions{Limit: minInt(seedLimit, 6), Now: now})
	seeds = MergeSeeds(seeds, BaseRecallNodes(allNodes))
	edges := s.store.ListMemoryEdges(nil, store.TemporalMemoryEdgeFilter{Status: "active", Limit: 256})
	activated := BuildActivationSet(cue, seeds, allNodes, edges, ActivationOptions{Limit: seedLimit, MaxDepth: in.MaxDepth, Now: now})
	chains := BuildRecallChains(activated, edges)
	cards := AssemblePromptCards(activatedToNodes(activated), chains, in.CardBudget)
	selectedCardIDs := make([]string, 0, len(cards))
	persistedCards := make([]PromptCard, 0, len(cards))
	for _, card := range cards {
		storedCard, err := s.store.UpsertMemoryCard(store.MemoryCard{
			UserID:         in.UserID,
			BotID:          in.BotID,
			CardType:       card.CardType,
			Text:           card.Text,
			BackingNodeIDs: append([]string(nil), card.BackingNodeIDs...),
			FreshnessScore: 1,
		})
		if err != nil {
			return result, err
		}
		selectedCardIDs = append(selectedCardIDs, storedCard.ID)
		persistedCards = append(persistedCards, PromptCard{CardType: storedCard.CardType, Text: storedCard.Text, BackingNodeIDs: storedCard.BackingNodeIDs})
	}
	trace, err := s.store.SaveActivationTrace(store.ActivationTrace{
		UserID:           in.UserID,
		BotID:            in.BotID,
		ConversationID:   in.ConversationID,
		TurnID:           in.TurnID,
		CueJSON:          cue.QueryText,
		SeedNodeIDs:      extractNodeIDs(seeds),
		ActivatedNodeIDs: extractActivatedNodeIDs(activated),
		SelectedCardIDs:  selectedCardIDs,
	})
	if err != nil {
		return result, err
	}
	result.Cue = cue
	result.Seeds = seeds
	result.Activated = activated
	result.Chains = chains
	result.Cards = persistedCards
	result.Trace = trace
	return result, nil
}

func (s *Service) Consolidate(_ context.Context, in ConsolidateInput) (ConsolidationResult, error) {
	result := ConsolidationResult{}
	if s == nil || s.store == nil {
		return result, nil
	}
	nodes := s.store.ListMemoryNodes(in.UserID, in.BotID, store.TemporalMemoryNodeFilter{Status: "active", Limit: 256})
	result = PromoteRepeatedEpisodesToHabit(nodes, ConsolidationOptions{Now: in.Now, MinOccurrences: in.MinOccurrences})
	if result.PromotedHabit == nil {
		return result, nil
	}
	created, err := s.store.CreateMemoryNode(*result.PromotedHabit)
	if err != nil {
		return ConsolidationResult{}, err
	}
	result.PromotedHabit = &created
	for _, nodeID := range result.BackingNodeIDs {
		_, err := s.store.CreateMemoryEdge(store.TemporalMemoryEdge{SourceID: created.ID, TargetID: nodeID, EdgeType: "supported_by", Weight: 0.9, Status: "active"})
		if err != nil {
			return ConsolidationResult{}, err
		}
	}
	return result, nil
}

func (s *Service) upsertSessionSummary(node store.TemporalMemoryNode) (store.TemporalMemoryNode, error) {
	if existing, ok := s.store.GetMemoryNode(node.ID); ok {
		node.CreatedAt = existing.CreatedAt
		if node.OccurredAt.IsZero() {
			node.OccurredAt = existing.OccurredAt
		}
		if node.ValidFrom.IsZero() {
			node.ValidFrom = existing.ValidFrom
		}
		return s.store.UpdateMemoryNode(node)
	}
	return s.store.CreateMemoryNode(node)
}

func (s *Service) persistNodesWithMerge(nodes []store.TemporalMemoryNode) ([]store.TemporalMemoryNode, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	persisted := make([]store.TemporalMemoryNode, 0, len(nodes))
	for _, node := range nodes {
		stored, err := s.upsertMemoryFact(node)
		if err != nil {
			return nil, err
		}
		persisted = append(persisted, stored)
	}
	return persisted, nil
}

func (s *Service) upsertMemoryFact(node store.TemporalMemoryNode) (store.TemporalMemoryNode, error) {
	existing := s.store.ListMemoryNodes(node.UserID, node.BotID, store.TemporalMemoryNodeFilter{NodeTypes: []string{node.NodeType}, Status: "active", Limit: 128})
	if match, matchedNode, ok := findBestSimilarityNode(node, existing); ok && match.ShouldMerge {
		merged := mergeNode(matchedNode, node)
		return s.store.UpdateMemoryNode(merged)
	}
	return s.store.CreateMemoryNode(node)
}

func activatedToNodes(activated []ActivatedNode) []store.TemporalMemoryNode {
	nodes := make([]store.TemporalMemoryNode, 0, len(activated))
	for _, item := range activated {
		nodes = append(nodes, item.Node)
	}
	return nodes
}

func extractNodeIDs(nodes []store.TemporalMemoryNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if id := strings.TrimSpace(node.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueIDs(ids)
}

func extractActivatedNodeIDs(nodes []ActivatedNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if id := strings.TrimSpace(node.Node.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueIDs(ids)
}

func (s *Service) generateEmbedding(ctx context.Context, nodeID, text string) {
	if s.embeddingService == nil || strings.TrimSpace(text) == "" {
		return
	}

	embedding, err := s.embeddingService.EmbedText(ctx, text)
	if err != nil {
		return
	}

	if graphStore, ok := s.store.(interface {
		UpsertEmbedding(nodeID string, embedding []float64, model string) error
	}); ok {
		_ = graphStore.UpsertEmbedding(nodeID, embedding, "")
	}
}
