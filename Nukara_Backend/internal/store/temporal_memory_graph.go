package store

import (
	"errors"
	"sort"
	"strings"
	"time"
)

type TemporalMemoryNode struct {
	ID               string
	UserID           string
	BotID            string
	SessionID        string
	NodeType         string
	Title            string
	Summary          string
	BodyJSON         string
	Salience         float64
	AffectWeight     float64
	Confidence       float64
	Stability        float64
	Status           string
	OccurredAt       time.Time
	ObservedAt       time.Time
	ValidFrom        time.Time
	ValidTo          *time.Time
	LastAccessedAt   time.Time
	SourceTurnID     string
	SourceKind       string
	SemanticCategory string
	StabilityLabel   string
	MergeKey         string
	EvidenceCount    int
	Entities         []Entity
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TemporalMemoryNodeFilter struct {
	IDs       []string
	NodeTypes []string
	SessionID string
	Status    string
	Limit     int
}

type TemporalMemoryEdge struct {
	ID            string
	SourceID      string
	TargetID      string
	EdgeType      string
	Weight        float64
	EvidenceCount int
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TemporalMemoryEdgeFilter struct {
	EdgeTypes []string
	Status    string
	Limit     int
}

type MemoryCard struct {
	ID             string
	UserID         string
	BotID          string
	CardType       string
	Text           string
	BackingNodeIDs []string
	FreshnessScore float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type MemoryCardFilter struct {
	CardTypes []string
	Limit     int
}

type ActivationTrace struct {
	ID               string
	UserID           string
	BotID            string
	ConversationID   string
	TurnID           string
	CueJSON          string
	SeedNodeIDs      []string
	ActivatedNodeIDs []string
	SelectedCardIDs  []string
	ResponseExcerpt  string
	CreatedAt        time.Time
}

type ActivationTraceFilter struct {
	ConversationID string
	TurnID         string
	Limit          int
}

func (s *Store) CreateMemoryNode(node TemporalMemoryNode) (TemporalMemoryNode, error) {
	node.UserID = strings.TrimSpace(node.UserID)
	node.BotID = strings.TrimSpace(node.BotID)
	node.SessionID = strings.TrimSpace(node.SessionID)
	node.NodeType = strings.TrimSpace(node.NodeType)
	node.Title = strings.TrimSpace(node.Title)
	node.Summary = strings.TrimSpace(node.Summary)
	node.Status = strings.TrimSpace(node.Status)
	node.SourceTurnID = strings.TrimSpace(node.SourceTurnID)
	node.SourceKind = strings.TrimSpace(node.SourceKind)
	node.SemanticCategory = strings.TrimSpace(node.SemanticCategory)
	node.StabilityLabel = strings.TrimSpace(node.StabilityLabel)
	node.MergeKey = strings.TrimSpace(node.MergeKey)
	if node.UserID == "" || node.BotID == "" || node.NodeType == "" || node.Summary == "" {
		return TemporalMemoryNode{}, errors.New("temporal memory node missing required fields")
	}
	if node.Status == "" {
		node.Status = "active"
	}
	now := time.Now().UTC()
	if node.ID == "" {
		node.ID = NewID()
	}
	if node.OccurredAt.IsZero() {
		node.OccurredAt = now
	}
	if node.ObservedAt.IsZero() {
		node.ObservedAt = now
	}
	if node.ValidFrom.IsZero() {
		node.ValidFrom = node.OccurredAt
	}
	if node.EvidenceCount <= 0 {
		node.EvidenceCount = 1
	}
	node.Entities = append([]Entity(nil), node.Entities...)
	node.CreatedAt = now
	node.UpdatedAt = now
	return s.UpdateMemoryNode(node)
}

func (s *Store) UpdateMemoryNode(node TemporalMemoryNode) (TemporalMemoryNode, error) {
	node.ID = strings.TrimSpace(node.ID)
	node.UserID = strings.TrimSpace(node.UserID)
	node.BotID = strings.TrimSpace(node.BotID)
	node.SessionID = strings.TrimSpace(node.SessionID)
	node.NodeType = strings.TrimSpace(node.NodeType)
	node.Title = strings.TrimSpace(node.Title)
	node.Summary = strings.TrimSpace(node.Summary)
	node.Status = strings.TrimSpace(node.Status)
	node.SourceTurnID = strings.TrimSpace(node.SourceTurnID)
	node.SourceKind = strings.TrimSpace(node.SourceKind)
	node.SemanticCategory = strings.TrimSpace(node.SemanticCategory)
	node.StabilityLabel = strings.TrimSpace(node.StabilityLabel)
	node.MergeKey = strings.TrimSpace(node.MergeKey)
	if node.ID == "" || node.UserID == "" || node.BotID == "" || node.NodeType == "" || node.Summary == "" {
		return TemporalMemoryNode{}, errors.New("temporal memory node missing required fields")
	}
	if node.Status == "" {
		node.Status = "active"
	}
	if node.EvidenceCount <= 0 {
		node.EvidenceCount = 1
	}
	node.Entities = append([]Entity(nil), node.Entities...)
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.memoryNodesByID[node.ID]; ok {
		if node.CreatedAt.IsZero() {
			node.CreatedAt = existing.CreatedAt
		}
		if node.OccurredAt.IsZero() {
			node.OccurredAt = existing.OccurredAt
		}
		if node.ObservedAt.IsZero() {
			node.ObservedAt = existing.ObservedAt
		}
		if node.ValidFrom.IsZero() {
			node.ValidFrom = existing.ValidFrom
		}
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	if node.OccurredAt.IsZero() {
		node.OccurredAt = now
	}
	if node.ObservedAt.IsZero() {
		node.ObservedAt = now
	}
	if node.ValidFrom.IsZero() {
		node.ValidFrom = node.OccurredAt
	}
	node.UpdatedAt = now
	s.memoryNodesByID[node.ID] = cloneTemporalMemoryNode(node)
	return cloneTemporalMemoryNode(node), nil
}

func (s *Store) GetMemoryNode(nodeID string) (TemporalMemoryNode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.memoryNodesByID[strings.TrimSpace(nodeID)]
	if !ok {
		return TemporalMemoryNode{}, false
	}
	return cloneTemporalMemoryNode(node), true
}

func (s *Store) ListMemoryNodes(userID, botID string, filter TemporalMemoryNodeFilter) []TemporalMemoryNode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	idSet := toLowerSet(filter.IDs)
	typeSet := toLowerSet(filter.NodeTypes)
	status := strings.TrimSpace(filter.Status)
	sessionID := strings.TrimSpace(filter.SessionID)
	out := make([]TemporalMemoryNode, 0, len(s.memoryNodesByID))
	for _, node := range s.memoryNodesByID {
		if strings.TrimSpace(node.UserID) != strings.TrimSpace(userID) || strings.TrimSpace(node.BotID) != strings.TrimSpace(botID) {
			continue
		}
		if len(idSet) > 0 {
			if _, ok := idSet[strings.ToLower(strings.TrimSpace(node.ID))]; !ok {
				continue
			}
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(node.NodeType))]; !ok {
				continue
			}
		}
		if sessionID != "" && strings.TrimSpace(node.SessionID) != sessionID {
			continue
		}
		if status != "" && !strings.EqualFold(strings.TrimSpace(node.Status), status) {
			continue
		}
		out = append(out, cloneTemporalMemoryNode(node))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].OccurredAt.After(out[j].OccurredAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) CreateMemoryEdge(edge TemporalMemoryEdge) (TemporalMemoryEdge, error) {
	edge.SourceID = strings.TrimSpace(edge.SourceID)
	edge.TargetID = strings.TrimSpace(edge.TargetID)
	edge.EdgeType = strings.TrimSpace(edge.EdgeType)
	edge.Status = strings.TrimSpace(edge.Status)
	if edge.SourceID == "" || edge.TargetID == "" || edge.EdgeType == "" {
		return TemporalMemoryEdge{}, errors.New("temporal memory edge missing required fields")
	}
	if edge.Status == "" {
		edge.Status = "active"
	}
	now := time.Now().UTC()
	if edge.ID == "" {
		edge.ID = NewID()
	}
	if edge.Weight == 0 {
		edge.Weight = 1
	}
	if edge.EvidenceCount <= 0 {
		edge.EvidenceCount = 1
	}
	edge.CreatedAt = now
	edge.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memoryEdgesByID[edge.ID] = edge
	return edge, nil
}

func (s *Store) ListMemoryEdges(nodeIDs []string, filter TemporalMemoryEdgeFilter) []TemporalMemoryEdge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	nodeSet := toLowerSet(nodeIDs)
	typeSet := toLowerSet(filter.EdgeTypes)
	status := strings.TrimSpace(filter.Status)
	out := make([]TemporalMemoryEdge, 0, len(s.memoryEdgesByID))
	for _, edge := range s.memoryEdgesByID {
		if len(nodeSet) > 0 {
			if _, ok := nodeSet[strings.ToLower(edge.SourceID)]; !ok {
				if _, ok := nodeSet[strings.ToLower(edge.TargetID)]; !ok {
					continue
				}
			}
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(edge.EdgeType))]; !ok {
				continue
			}
		}
		if status != "" && !strings.EqualFold(strings.TrimSpace(edge.Status), status) {
			continue
		}
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return append([]TemporalMemoryEdge(nil), out...)
}

func (s *Store) UpsertMemoryCard(card MemoryCard) (MemoryCard, error) {
	card.UserID = strings.TrimSpace(card.UserID)
	card.BotID = strings.TrimSpace(card.BotID)
	card.CardType = strings.TrimSpace(card.CardType)
	card.Text = strings.TrimSpace(card.Text)
	if card.UserID == "" || card.BotID == "" || card.CardType == "" || card.Text == "" {
		return MemoryCard{}, errors.New("memory card missing required fields")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(card.ID) == "" {
		card.ID = NewID()
		card.CreatedAt = now
	} else if existing, ok := s.memoryCardsByID[card.ID]; ok {
		card.CreatedAt = existing.CreatedAt
	}
	card.UpdatedAt = now
	card.BackingNodeIDs = append([]string(nil), card.BackingNodeIDs...)
	s.memoryCardsByID[card.ID] = card
	return card, nil
}

func (s *Store) ListMemoryCards(userID, botID string, filter MemoryCardFilter) []MemoryCard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	typeSet := toLowerSet(filter.CardTypes)
	out := make([]MemoryCard, 0, len(s.memoryCardsByID))
	for _, card := range s.memoryCardsByID {
		if strings.TrimSpace(card.UserID) != strings.TrimSpace(userID) || strings.TrimSpace(card.BotID) != strings.TrimSpace(botID) {
			continue
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(card.CardType))]; !ok {
				continue
			}
		}
		clone := card
		clone.BackingNodeIDs = append([]string(nil), card.BackingNodeIDs...)
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) SaveActivationTrace(trace ActivationTrace) (ActivationTrace, error) {
	trace.UserID = strings.TrimSpace(trace.UserID)
	trace.BotID = strings.TrimSpace(trace.BotID)
	trace.ConversationID = strings.TrimSpace(trace.ConversationID)
	trace.TurnID = strings.TrimSpace(trace.TurnID)
	trace.CueJSON = strings.TrimSpace(trace.CueJSON)
	if trace.UserID == "" || trace.BotID == "" {
		return ActivationTrace{}, errors.New("activation trace missing required fields")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(trace.ID) == "" {
		trace.ID = NewID()
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = now
	}
	trace.SeedNodeIDs = append([]string(nil), trace.SeedNodeIDs...)
	trace.ActivatedNodeIDs = append([]string(nil), trace.ActivatedNodeIDs...)
	trace.SelectedCardIDs = append([]string(nil), trace.SelectedCardIDs...)
	s.activationTracesByID[trace.ID] = trace
	return trace, nil
}

func (s *Store) ListActivationTraces(userID, botID string, filter ActivationTraceFilter) []ActivationTrace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	conversationID := strings.TrimSpace(filter.ConversationID)
	turnID := strings.TrimSpace(filter.TurnID)
	out := make([]ActivationTrace, 0, len(s.activationTracesByID))
	for _, trace := range s.activationTracesByID {
		if strings.TrimSpace(trace.UserID) != strings.TrimSpace(userID) || strings.TrimSpace(trace.BotID) != strings.TrimSpace(botID) {
			continue
		}
		if conversationID != "" && strings.TrimSpace(trace.ConversationID) != conversationID {
			continue
		}
		if turnID != "" && strings.TrimSpace(trace.TurnID) != turnID {
			continue
		}
		clone := trace
		clone.SeedNodeIDs = append([]string(nil), trace.SeedNodeIDs...)
		clone.ActivatedNodeIDs = append([]string(nil), trace.ActivatedNodeIDs...)
		clone.SelectedCardIDs = append([]string(nil), trace.SelectedCardIDs...)
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) DeleteMemoryNode(nodeID, userID, botID string) error {
	nodeID = strings.TrimSpace(nodeID)
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.memoryNodesByID[nodeID]
	if !ok {
		return errors.New("memory node not found")
	}
	if node.UserID != userID || node.BotID != botID {
		return errors.New("memory node not found")
	}
	delete(s.memoryNodesByID, nodeID)
	return nil
}

func cloneTemporalMemoryNode(node TemporalMemoryNode) TemporalMemoryNode {
	clone := node
	if node.ValidTo != nil {
		value := node.ValidTo.UTC()
		clone.ValidTo = &value
	}
	clone.Entities = append([]Entity(nil), node.Entities...)
	return clone
}

func toLowerSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}
	return out
}
