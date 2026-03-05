package store

import (
	"errors"
	"strings"
	"time"
)

type AgentTurn struct {
	ID                 string
	UserID             string
	BotID              string
	ConversationID     string
	UserMessageIDs     []string
	AggregatedUserText string
	BotReplyText       string
	CreatedAt          time.Time
}

type ConversationCompact struct {
	ConversationID string
	CompactJSON    string
	UntilTurnID    string
	UpdatedAt      time.Time
}

type MemoryItem struct {
	ID         string
	UserID     string
	BotID      string
	Kind       string
	Owner      string
	Content    string
	Importance int
	OccurredAt time.Time
	Status     string
	Topics     []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type providerSetting struct {
	ProviderID string
	Model      string
}

func (s *Store) SetUserProviderSetting(userID, providerID, model string) error {
	userID = strings.TrimSpace(userID)
	providerID = strings.TrimSpace(providerID)
	if userID == "" || providerID == "" {
		return errors.New("userID and providerID are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userProviderSettings[userID] = providerSetting{
		ProviderID: providerID,
		Model:      strings.TrimSpace(model),
	}
	return nil
}

func (s *Store) GetUserProviderSetting(userID string) (providerID, model string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	setting, ok := s.userProviderSettings[strings.TrimSpace(userID)]
	if !ok {
		return "", "", false
	}
	return setting.ProviderID, setting.Model, true
}

func (s *Store) SetBotProviderOverride(userID, botID, providerID, model string) error {
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	providerID = strings.TrimSpace(providerID)
	if userID == "" || botID == "" || providerID == "" {
		return errors.New("userID, botID and providerID are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.botProviderOverrides[userID+":"+botID] = providerSetting{
		ProviderID: providerID,
		Model:      strings.TrimSpace(model),
	}
	return nil
}

func (s *Store) GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	setting, ok := s.botProviderOverrides[strings.TrimSpace(userID)+":"+strings.TrimSpace(botID)]
	if !ok {
		return "", "", false
	}
	return setting.ProviderID, setting.Model, true
}

func (s *Store) SetSystemSetting(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("system setting key is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systemSettings[key] = strings.TrimSpace(value)
	return nil
}

func (s *Store) GetSystemSetting(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.systemSettings[strings.TrimSpace(key)]
	return value, ok
}

func (s *Store) CreateTurn(turn AgentTurn) (AgentTurn, error) {
	if strings.TrimSpace(turn.UserID) == "" || strings.TrimSpace(turn.BotID) == "" || strings.TrimSpace(turn.ConversationID) == "" {
		return AgentTurn{}, errors.New("turn missing required ids")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(turn.ID) == "" {
		turn.ID = NewID()
	}
	turn.CreatedAt = time.Now().UTC()
	turn.UserMessageIDs = append([]string(nil), turn.UserMessageIDs...)
	s.agentTurnsByID[turn.ID] = turn
	return turn, nil
}

func (s *Store) UpsertCompact(conversationID, compactJSON, untilTurnID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return errors.New("conversation id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactsByConv[conversationID] = ConversationCompact{
		ConversationID: conversationID,
		CompactJSON:    strings.TrimSpace(compactJSON),
		UntilTurnID:    strings.TrimSpace(untilTurnID),
		UpdatedAt:      time.Now().UTC(),
	}
	return nil
}

func (s *Store) GetConversationCompact(conversationID string) (ConversationCompact, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	compact, ok := s.compactsByConv[strings.TrimSpace(conversationID)]
	return compact, ok
}

func (s *Store) UpsertMemoryItem(item MemoryItem) (MemoryItem, error) {
	if strings.TrimSpace(item.UserID) == "" || strings.TrimSpace(item.BotID) == "" || strings.TrimSpace(item.Content) == "" {
		return MemoryItem{}, errors.New("memory item missing required fields")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if strings.TrimSpace(item.ID) == "" {
		item.ID = NewID()
		item.CreatedAt = now
	} else if existing, ok := s.memoryItemsByID[item.ID]; ok {
		item.CreatedAt = existing.CreatedAt
	}
	if item.OccurredAt.IsZero() {
		item.OccurredAt = now
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "active"
	}
	item.UpdatedAt = now
	item.Topics = append([]string(nil), item.Topics...)
	s.memoryItemsByID[item.ID] = item
	return item, nil
}

func (s *Store) GetMemoryItem(memoryID string) (MemoryItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.memoryItemsByID[strings.TrimSpace(memoryID)]
	return item, ok
}
