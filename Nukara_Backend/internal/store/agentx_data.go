package store

import (
	"errors"
	"sort"
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

type BotRuntimeState struct {
	UserID          string
	BotID           string
	ActivityText    string
	BasisTags       []string
	SourceMemoryIDs []string
	UpdatedAt       time.Time
}

type PersonaChangeEvent struct {
	ID            string
	UserID        string
	BotID         string
	Field         string
	ChangeType    string
	ProposedValue string
	SourceTurnID  string
	Risk          string
	Status        string
	ReviewerNote  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
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

func (s *Store) ListMemoryItems(userID, botID string, limit int) []MemoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	items := make([]MemoryItem, 0, len(s.memoryItemsByID))
	for _, item := range s.memoryItemsByID {
		if strings.TrimSpace(item.UserID) != strings.TrimSpace(userID) || strings.TrimSpace(item.BotID) != strings.TrimSpace(botID) {
			continue
		}
		if strings.TrimSpace(item.Status) != "" && !strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Importance != items[j].Importance {
			return items[i].Importance > items[j].Importance
		}
		return items[i].OccurredAt.After(items[j].OccurredAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return append([]MemoryItem(nil), items...)
}

func (s *Store) UpsertBotRuntimeState(state BotRuntimeState) (BotRuntimeState, error) {
	state.UserID = strings.TrimSpace(state.UserID)
	state.BotID = strings.TrimSpace(state.BotID)
	state.ActivityText = strings.TrimSpace(state.ActivityText)
	if state.UserID == "" || state.BotID == "" || state.ActivityText == "" {
		return BotRuntimeState{}, errors.New("runtime state missing required fields")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	} else {
		state.UpdatedAt = state.UpdatedAt.UTC()
	}
	state.BasisTags = append([]string(nil), state.BasisTags...)
	state.SourceMemoryIDs = append([]string(nil), state.SourceMemoryIDs...)
	key := state.UserID + ":" + state.BotID
	s.runtimeStatesByKey[key] = state
	return state, nil
}

func (s *Store) GetBotRuntimeState(userID, botID string) (BotRuntimeState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.runtimeStatesByKey[strings.TrimSpace(userID)+":"+strings.TrimSpace(botID)]
	if !ok {
		return BotRuntimeState{}, false
	}
	state.BasisTags = append([]string(nil), state.BasisTags...)
	state.SourceMemoryIDs = append([]string(nil), state.SourceMemoryIDs...)
	return state, true
}

func (s *Store) CreatePersonaChangeEvent(event PersonaChangeEvent) (PersonaChangeEvent, error) {
	event.UserID = strings.TrimSpace(event.UserID)
	event.BotID = strings.TrimSpace(event.BotID)
	event.Field = strings.TrimSpace(event.Field)
	event.ChangeType = strings.TrimSpace(event.ChangeType)
	event.ProposedValue = strings.TrimSpace(event.ProposedValue)
	event.SourceTurnID = strings.TrimSpace(event.SourceTurnID)
	event.Risk = strings.TrimSpace(event.Risk)
	event.Status = strings.TrimSpace(event.Status)
	event.ReviewerNote = strings.TrimSpace(event.ReviewerNote)
	if event.UserID == "" || event.BotID == "" || event.Field == "" || event.ProposedValue == "" {
		return PersonaChangeEvent{}, errors.New("persona change event missing required fields")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if strings.TrimSpace(event.ID) == "" {
		event.ID = NewID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = event.CreatedAt
	} else {
		event.UpdatedAt = event.UpdatedAt.UTC()
	}
	if event.Status == "" {
		event.Status = "pending"
	}
	s.personaChangeEventsByID[event.ID] = event
	s.prunePersonaChangeEventsLocked(event.UserID, event.BotID, 20)
	return event, nil
}

func (s *Store) ListPersonaChangeEvents(userID, botID, status string, limit int) []PersonaChangeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	status = strings.TrimSpace(status)
	if limit <= 0 {
		limit = 20
	}
	items := make([]PersonaChangeEvent, 0, len(s.personaChangeEventsByID))
	for _, item := range s.personaChangeEventsByID {
		if strings.TrimSpace(item.UserID) != userID || strings.TrimSpace(item.BotID) != botID {
			continue
		}
		if status != "" && !strings.EqualFold(strings.TrimSpace(item.Status), status) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return append([]PersonaChangeEvent(nil), items...)
}

func (s *Store) UpdatePersonaChangeEventStatus(changeID, status, reviewerNote string) (PersonaChangeEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	changeID = strings.TrimSpace(changeID)
	status = strings.TrimSpace(status)
	item, ok := s.personaChangeEventsByID[changeID]
	if !ok || changeID == "" || status == "" {
		return PersonaChangeEvent{}, false
	}
	item.Status = status
	item.ReviewerNote = strings.TrimSpace(reviewerNote)
	item.UpdatedAt = time.Now().UTC()
	s.personaChangeEventsByID[changeID] = item
	return item, true
}

func (s *Store) prunePersonaChangeEventsLocked(userID, botID string, keep int) {
	if keep <= 0 {
		keep = 20
	}
	items := make([]PersonaChangeEvent, 0, len(s.personaChangeEventsByID))
	for _, item := range s.personaChangeEventsByID {
		if strings.TrimSpace(item.UserID) != strings.TrimSpace(userID) || strings.TrimSpace(item.BotID) != strings.TrimSpace(botID) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	for i := keep; i < len(items); i++ {
		delete(s.personaChangeEventsByID, items[i].ID)
	}
}
