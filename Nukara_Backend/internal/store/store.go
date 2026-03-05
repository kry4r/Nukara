package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SMSCode struct {
	Phone     string
	Purpose   string
	Code      string
	ExpiresAt time.Time
}

type Bot struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"-"`
	Name                string    `json:"name"`
	Avatar              string    `json:"avatar,omitempty"`
	AvatarBase64        string    `json:"avatar_base64,omitempty"`
	Summary             string    `json:"summary"`
	Relationship        string    `json:"relationship,omitempty"`
	Role                string    `json:"role,omitempty"`
	SelfCognition       []string  `json:"self_cognition,omitempty"`
	PersonaPrompt       string    `json:"persona_prompt,omitempty"`
	PersonaVersion      int       `json:"persona_version,omitempty"`
	SpeakingStyle       string    `json:"speaking_style"`
	Background          string    `json:"background"`
	Traits              []string  `json:"traits"`
	Gender              string    `json:"gender"`
	ChatBackgroundStyle string    `json:"chat_background_style"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type BotState struct {
	UserID      string
	BotID       string
	StatusEmoji string
	StatusText  string
	TurnCount   int
	UpdatedAt   time.Time
}

type UserStatus struct {
	UserID    string    `json:"user_id"`
	Emoji     string    `json:"emoji"`
	Text      string    `json:"text"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MessageContent struct {
	Type        string   `json:"type"`
	Text        string   `json:"text,omitempty"`
	ImageBase64 string   `json:"image_base64,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Name        string   `json:"name,omitempty"`
}

type Message struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	SenderType     string         `json:"sender_type"`
	ContentType    string         `json:"content_type"`
	Content        MessageContent `json:"content"`
	IsProactive    bool           `json:"is_proactive"`
	EmotionTag     string         `json:"emotion_tag,omitempty"`
	ReplyGroupID   string         `json:"reply_group_id,omitempty"`
	Sequence       int            `json:"sequence,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type Conversation struct {
	ID                 string `json:"id"`
	UserID             string
	BotID              string    `json:"bot_id"`
	BotName            string    `json:"bot_name"`
	BotAvatar          string    `json:"bot_avatar,omitempty"`
	BotAvatarBase64    string    `json:"bot_avatar_base64,omitempty"`
	LastMessage        string    `json:"last_message"`
	LastMessageAt      time.Time `json:"last_message_at"`
	UnreadCount        int       `json:"unread_count"`
	IsProactiveMessage bool      `json:"is_proactive_message"`
}

type DeviceToken struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"device_token"`
	Platform  string    `json:"platform"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NotificationSettings struct {
	UserID           string    `json:"user_id"`
	ProactiveEnabled bool      `json:"proactive_enabled"`
	DNDStart         string    `json:"dnd_start,omitempty"`
	DNDEnd           string    `json:"dnd_end,omitempty"`
	Frequency        string    `json:"frequency"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ProactiveLog struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ConversationID string    `json:"conversation_id"`
	BotID          string    `json:"bot_id"`
	TriggerType    string    `json:"trigger_type"`
	Message        string    `json:"message"`
	SentByWS       bool      `json:"sent_by_ws"`
	SentByAPNs     bool      `json:"sent_by_apns"`
	CreatedAt      time.Time `json:"created_at"`
}

// EmotionContext tracks user emotion trends for a specific bot relationship.
type EmotionContext struct {
	RecentEmotions []string  `json:"recent_emotions"` // last batch emotions
	EmotionTrend   string    `json:"emotion_trend"`   // positive/negative/neutral
	LastTone       string    `json:"last_tone"`       // tone of last message
	UpdatedAt      time.Time `json:"updated_at"`
}

type Directive struct {
	ID              string    `json:"id"`
	UserID          string    `json:"-"`
	BotID           string    `json:"bot_id"`
	Content         string    `json:"content"`
	Category        string    `json:"category"`
	Source          string    `json:"source"`
	Status          string    `json:"status"`
	OriginalMessage string    `json:"original_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Metrics struct {
	RequestsTotal       int `json:"requests_total"`
	ActiveWSConnections int `json:"active_websocket_connections"`
	ProactiveSentTotal  int `json:"proactive_message_sent"`
}

type presenceState struct {
	wsExpiresAt   time.Time
	lastUserMsgAt time.Time
}

type Store struct {
	mu sync.RWMutex

	usersByPhone map[string]User
	usersByID    map[string]User
	smsCodes     map[string]SMSCode

	botsByID       map[string]Bot
	botsByUser     map[string][]string
	botStatesByKey map[string]BotState

	conversationsByID   map[string]Conversation
	conversationsByUser map[string][]string
	messagesByConv      map[string][]Message

	deviceTokenByUser map[string]DeviceToken
	notifByUser       map[string]NotificationSettings
	userStatusByUser  map[string]UserStatus
	proactiveLogs     []ProactiveLog
	directivesByBot   map[string][]Directive

	emotionBuffers       map[string][]string       // key: userID:botID
	emotionCtxs          map[string]EmotionContext // key: userID:botID
	presenceByUser       map[string]presenceState
	userProviderSettings map[string]providerSetting
	botProviderOverrides map[string]providerSetting
	systemSettings       map[string]string
	agentTurnsByID       map[string]AgentTurn
	compactsByConv       map[string]ConversationCompact
	memoryItemsByID      map[string]MemoryItem

	metrics Metrics
}

func NewStore() *Store {
	s := &Store{
		usersByPhone:         map[string]User{},
		usersByID:            map[string]User{},
		smsCodes:             map[string]SMSCode{},
		botsByID:             map[string]Bot{},
		botsByUser:           map[string][]string{},
		botStatesByKey:       map[string]BotState{},
		conversationsByID:    map[string]Conversation{},
		conversationsByUser:  map[string][]string{},
		messagesByConv:       map[string][]Message{},
		deviceTokenByUser:    map[string]DeviceToken{},
		notifByUser:          map[string]NotificationSettings{},
		userStatusByUser:     map[string]UserStatus{},
		proactiveLogs:        []ProactiveLog{},
		directivesByBot:      map[string][]Directive{},
		emotionBuffers:       map[string][]string{},
		emotionCtxs:          map[string]EmotionContext{},
		presenceByUser:       map[string]presenceState{},
		userProviderSettings: map[string]providerSetting{},
		botProviderOverrides: map[string]providerSetting{},
		systemSettings:       map[string]string{},
		agentTurnsByID:       map[string]AgentTurn{},
		compactsByConv:       map[string]ConversationCompact{},
		memoryItemsByID:      map[string]MemoryItem{},
	}

	// seed dev user so the preset phone 13800138000 works for login
	devUser := User{
		ID:        NewID(),
		Phone:     "13800138000",
		Nickname:  "测试用户",
		CreatedAt: time.Now().UTC(),
	}
	s.usersByPhone[devUser.Phone] = devUser
	s.usersByID[devUser.ID] = devUser
	s.notifByUser[devUser.ID] = NotificationSettings{
		UserID:           devUser.ID,
		ProactiveEnabled: true,
		Frequency:        "normal",
		UpdatedAt:        time.Now().UTC(),
	}
	s.systemSettings["default_chat_provider_id"] = "minimax_m2_5"
	s.systemSettings["default_chat_model"] = "MiniMax-M2.5"
	s.systemSettings["embedding_provider_id"] = "minimax_m2_5"
	s.systemSettings["embedding_model"] = "MiniMax-M2.5"

	return s
}

func (s *Store) IncrementRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.RequestsTotal++
}

func (s *Store) SnapshotMetrics() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func (s *Store) SetWSConnections(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.ActiveWSConnections = count
}

func (s *Store) TouchWSPresence(userID string, ttl time.Duration) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.presenceByUser[userID]
	st.wsExpiresAt = time.Now().UTC().Add(ttl)
	s.presenceByUser[userID] = st
}

func (s *Store) IsUserWSOnline(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.presenceByUser[userID]
	if !ok {
		return false
	}
	if st.wsExpiresAt.IsZero() {
		return false
	}
	now := time.Now().UTC()
	if now.After(st.wsExpiresAt) {
		if st.lastUserMsgAt.IsZero() {
			delete(s.presenceByUser, userID)
		} else {
			st.wsExpiresAt = time.Time{}
			s.presenceByUser[userID] = st
		}
		return false
	}
	return true
}

func (s *Store) SetLastUserMessageAt(userID string, at time.Time) {
	userID = strings.TrimSpace(userID)
	if userID == "" || at.IsZero() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.presenceByUser[userID]
	st.lastUserMsgAt = at.UTC()
	s.presenceByUser[userID] = st
}

func (s *Store) GetLastUserMessageAt(userID string) (time.Time, bool) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return time.Time{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.presenceByUser[userID]
	if !ok || st.lastUserMsgAt.IsZero() {
		return time.Time{}, false
	}
	if !st.wsExpiresAt.IsZero() && time.Now().UTC().After(st.wsExpiresAt) {
		st.wsExpiresAt = time.Time{}
		s.presenceByUser[userID] = st
	}
	return st.lastUserMsgAt, true
}

func (s *Store) SaveSMSCode(phone, purpose, code string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.smsCodes[phone+":"+purpose] = SMSCode{Phone: phone, Purpose: purpose, Code: code, ExpiresAt: time.Now().Add(ttl)}
}

func (s *Store) ValidateSMSCode(phone, purpose, code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.smsCodes[phone+":"+purpose]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return false
	}
	return entry.Code == code
}

func (s *Store) FindUserByPhone(phone string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByPhone[phone]
	return user, ok
}

func (s *Store) FindUserByID(id string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByID[id]
	return user, ok
}

func (s *Store) CreateUser(phone, nickname string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.usersByPhone[phone]; exists {
		return User{}, errors.New("phone already registered")
	}
	id := NewID()
	user := User{ID: id, Phone: phone, Nickname: nickname, CreatedAt: time.Now().UTC()}
	s.usersByPhone[phone] = user
	s.usersByID[id] = user
	s.notifByUser[id] = NotificationSettings{UserID: id, ProactiveEnabled: true, Frequency: "normal", UpdatedAt: time.Now().UTC()}
	return user, nil
}

func (s *Store) UpsertDeviceToken(userID, token, platform string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceTokenByUser[userID] = DeviceToken{UserID: userID, Token: token, Platform: platform, UpdatedAt: time.Now().UTC()}
}

func (s *Store) GetDeviceToken(userID string) (DeviceToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.deviceTokenByUser[userID]
	return token, ok
}

func (s *Store) UpdateNotificationSettings(userID string, input NotificationSettings) NotificationSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	input.UserID = userID
	input.UpdatedAt = time.Now().UTC()
	if input.Frequency == "" {
		input.Frequency = "normal"
	}
	s.notifByUser[userID] = input
	return input
}

func (s *Store) GetNotificationSettings(userID string) NotificationSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	settings, ok := s.notifByUser[userID]
	if !ok {
		return NotificationSettings{UserID: userID, ProactiveEnabled: true, Frequency: "normal", UpdatedAt: time.Now().UTC()}
	}
	return settings
}

func (s *Store) CreateBot(userID string, bot Bot) Bot {
	s.mu.Lock()
	defer s.mu.Unlock()

	bot.ID = NewID()
	bot.UserID = userID
	bot.CreatedAt = time.Now().UTC()
	bot.UpdatedAt = bot.CreatedAt
	if bot.ChatBackgroundStyle == "" {
		bot.ChatBackgroundStyle = "lightPaper"
	}
	if bot.PersonaVersion <= 0 {
		bot.PersonaVersion = 1
	}
	if strings.TrimSpace(bot.Relationship) == "" {
		bot.Relationship = strings.TrimSpace(bot.Summary)
	}
	if strings.TrimSpace(bot.Role) == "" {
		bot.Role = strings.TrimSpace(bot.Background)
	}
	s.botsByID[bot.ID] = bot
	s.botsByUser[userID] = append(s.botsByUser[userID], bot.ID)

	s.botStatesByKey[userID+":"+bot.ID] = BotState{UserID: userID, BotID: bot.ID, StatusEmoji: "🙂", StatusText: "在线", UpdatedAt: time.Now().UTC()}

	conv := Conversation{
		ID:              NewID(),
		UserID:          userID,
		BotID:           bot.ID,
		BotName:         bot.Name,
		BotAvatar:       bot.Avatar,
		BotAvatarBase64: bot.AvatarBase64,
		LastMessage:     "",
		LastMessageAt:   time.Now().UTC(),
	}
	s.conversationsByID[conv.ID] = conv
	s.conversationsByUser[userID] = append(s.conversationsByUser[userID], conv.ID)

	return bot
}

func (s *Store) ListBots(userID string) []Bot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.botsByUser[userID]
	out := make([]Bot, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.botsByID[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) GetBot(userID, botID string) (Bot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bot, ok := s.botsByID[botID]
	if !ok || bot.UserID != userID {
		return Bot{}, false
	}
	return bot, true
}

func (s *Store) GetBotState(userID, botID string) (BotState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.botStatesByKey[userID+":"+botID]
	return state, ok
}

func (s *Store) UpdateBot(userID, botID string, patch Bot) (Bot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bot, ok := s.botsByID[botID]
	if !ok || bot.UserID != userID {
		return Bot{}, false
	}

	if strings.TrimSpace(patch.Name) != "" {
		bot.Name = strings.TrimSpace(patch.Name)
	}
	if patch.Summary != "" {
		bot.Summary = strings.TrimSpace(patch.Summary)
		bot.Relationship = strings.TrimSpace(patch.Summary)
	}
	if patch.Relationship != "" {
		bot.Relationship = strings.TrimSpace(patch.Relationship)
	}
	if patch.SpeakingStyle != "" {
		bot.SpeakingStyle = strings.TrimSpace(patch.SpeakingStyle)
	}
	if patch.Background != "" {
		bot.Background = strings.TrimSpace(patch.Background)
		bot.Role = strings.TrimSpace(patch.Background)
	}
	if patch.Role != "" {
		bot.Role = strings.TrimSpace(patch.Role)
	}
	if patch.Gender != "" {
		bot.Gender = patch.Gender
	}
	if len(patch.Traits) > 0 {
		bot.Traits = patch.Traits
	}
	bot.UpdatedAt = time.Now().UTC()
	s.botsByID[botID] = bot

	return bot, true
}

func (s *Store) AppendBotPersona(userID, botID string, speakingAdds, backgroundAdds, traitAdds []string, gender *string) (Bot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bot, ok := s.botsByID[botID]
	if !ok || bot.UserID != userID {
		return Bot{}, false
	}

	bot.SpeakingStyle = strings.Join(dedup(append(splitSegments(bot.SpeakingStyle), speakingAdds...)), "|")
	bot.Background = strings.Join(dedup(append(splitSegments(bot.Background), backgroundAdds...)), "|")
	bot.Role = bot.Background
	bot.Traits = dedup(append(bot.Traits, traitAdds...))
	if gender != nil && *gender != "" {
		bot.Gender = *gender
	}
	bot.UpdatedAt = time.Now().UTC()
	s.botsByID[botID] = bot

	return bot, true
}

type PersonaPatchInput struct {
	Relationship      string
	Role              string
	SelfCognitionAdds []string
	SpeakingStyleAdds []string
	TraitAdds         []string
	Gender            *string
	PersonaPrompt     string
}

func (s *Store) ApplyBotPersonaPatch(userID, botID string, input PersonaPatchInput) (Bot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bot, ok := s.botsByID[botID]
	if !ok || bot.UserID != userID {
		return Bot{}, false
	}
	if strings.TrimSpace(input.Relationship) != "" {
		bot.Relationship = strings.TrimSpace(input.Relationship)
		bot.Summary = bot.Relationship
	}
	if strings.TrimSpace(input.Role) != "" {
		bot.Role = strings.TrimSpace(input.Role)
		bot.Background = bot.Role
	}
	if len(input.SelfCognitionAdds) > 0 {
		bot.SelfCognition = dedup(append(bot.SelfCognition, input.SelfCognitionAdds...))
	}
	if len(input.SpeakingStyleAdds) > 0 {
		bot.SpeakingStyle = strings.Join(dedup(append(splitSegments(bot.SpeakingStyle), input.SpeakingStyleAdds...)), "|")
	}
	if len(input.TraitAdds) > 0 {
		bot.Traits = dedup(append(bot.Traits, input.TraitAdds...))
	}
	if input.Gender != nil && strings.TrimSpace(*input.Gender) != "" {
		bot.Gender = strings.TrimSpace(*input.Gender)
	}
	if strings.TrimSpace(input.PersonaPrompt) != "" {
		bot.PersonaPrompt = strings.TrimSpace(input.PersonaPrompt)
	}
	bot.PersonaVersion++
	if bot.PersonaVersion <= 0 {
		bot.PersonaVersion = 1
	}
	bot.UpdatedAt = time.Now().UTC()
	s.botsByID[botID] = bot
	return bot, true
}

func (s *Store) ListConversations(userID string) []Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.conversationsByUser[userID]
	out := make([]Conversation, 0, len(ids))
	for _, id := range ids {
		conv := s.conversationsByID[id]
		out = append(out, conv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastMessageAt.After(out[j].LastMessageAt) })
	return out
}

func (s *Store) FindConversationByBot(userID, botID string) (Conversation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.conversationsByUser[userID]
	for _, id := range ids {
		conv := s.conversationsByID[id]
		if conv.BotID == botID {
			return conv, true
		}
	}
	return Conversation{}, false
}

func (s *Store) EnsureConversation(userID, botID, botName, botAvatar, botAvatarBase64 string) Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.conversationsByUser[userID]
	for _, id := range ids {
		conv := s.conversationsByID[id]
		if conv.BotID == botID {
			return conv
		}
	}
	conv := Conversation{
		ID:              NewID(),
		UserID:          userID,
		BotID:           botID,
		BotName:         botName,
		BotAvatar:       botAvatar,
		BotAvatarBase64: botAvatarBase64,
		LastMessage:     "",
		LastMessageAt:   time.Now().UTC(),
	}
	s.conversationsByID[conv.ID] = conv
	s.conversationsByUser[userID] = append(s.conversationsByUser[userID], conv.ID)
	return conv
}

func (s *Store) GetConversation(userID, conversationID string) (Conversation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.conversationsByID[conversationID]
	if !ok || conv.UserID != userID {
		return Conversation{}, false
	}
	return conv, true
}

func (s *Store) ListMessages(userID, conversationID string, limit int) ([]Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.conversationsByID[conversationID]
	if !ok || conv.UserID != userID {
		return nil, false
	}
	messages := s.messagesByConv[conversationID]
	if limit <= 0 || len(messages) <= limit {
		return append([]Message{}, messages...), true
	}
	return append([]Message{}, messages[len(messages)-limit:]...), true
}

func (s *Store) MarkConversationRead(userID, conversationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	conv, ok := s.conversationsByID[conversationID]
	if !ok || conv.UserID != userID {
		return false
	}
	conv.UnreadCount = 0
	s.conversationsByID[conversationID] = conv
	return true
}

func (s *Store) SaveMessage(userID string, message Message) (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversationsByID[message.ConversationID]
	if !ok || conv.UserID != userID {
		return Message{}, false
	}

	if strings.TrimSpace(message.ID) == "" {
		message.ID = NewID()
	}
	message.CreatedAt = time.Now().UTC()
	if message.ContentType == "" {
		message.ContentType = message.Content.Type
	}
	s.messagesByConv[message.ConversationID] = append(s.messagesByConv[message.ConversationID], message)

	conv.LastMessage = previewText(message)
	conv.LastMessageAt = message.CreatedAt
	conv.IsProactiveMessage = message.IsProactive
	if message.SenderType == "bot" {
		conv.UnreadCount++
	}
	s.conversationsByID[message.ConversationID] = conv

	return message, true
}

func (s *Store) SaveBotStatus(userID, botID, emoji, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.botStatesByKey[userID+":"+botID]
	existing.UserID = userID
	existing.BotID = botID
	existing.StatusEmoji = emoji
	existing.StatusText = text
	existing.UpdatedAt = time.Now().UTC()
	s.botStatesByKey[userID+":"+botID] = existing
}

func (s *Store) IncrementTurnCount(userID, botID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + ":" + botID
	st := s.botStatesByKey[key]
	st.UserID = userID
	st.BotID = botID
	st.TurnCount++
	st.UpdatedAt = time.Now().UTC()
	s.botStatesByKey[key] = st
	return st.TurnCount
}

func (s *Store) SaveUserStatus(userID, emoji, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userStatusByUser[userID] = UserStatus{UserID: userID, Emoji: emoji, Text: text, UpdatedAt: time.Now().UTC()}
}

func (s *Store) GetUserStatus(userID string) (UserStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.userStatusByUser[userID]
	return st, ok
}

func (s *Store) AddProactiveLog(log ProactiveLog) ProactiveLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.ID = NewID()
	log.CreatedAt = time.Now().UTC()
	s.proactiveLogs = append([]ProactiveLog{log}, s.proactiveLogs...)
	s.metrics.ProactiveSentTotal++
	return log
}

func (s *Store) ListProactiveLogs(userID string, limit int) []ProactiveLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	out := make([]ProactiveLog, 0, limit)
	for _, entry := range s.proactiveLogs {
		if userID == "" || entry.UserID == userID {
			out = append(out, entry)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func (s *Store) ListAllUserIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.usersByID))
	for id := range s.usersByID {
		ids = append(ids, id)
	}
	return ids
}

func (s *Store) SaveDirective(d Directive) Directive {
	s.mu.Lock()
	defer s.mu.Unlock()
	d.ID = NewID()
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	if d.Status == "" {
		d.Status = "active"
	}
	key := d.UserID + ":" + d.BotID
	s.directivesByBot[key] = append(s.directivesByBot[key], d)
	return d
}

func (s *Store) ListDirectives(userID, botID, status string) []Directive {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := userID + ":" + botID
	var out []Directive
	for _, d := range s.directivesByBot[key] {
		if status == "" || d.Status == status {
			out = append(out, d)
		}
	}
	return out
}

func (s *Store) RevokeDirective(userID, botID, directiveID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + ":" + botID
	for i, d := range s.directivesByBot[key] {
		if d.ID == directiveID {
			s.directivesByBot[key][i].Status = "revoked"
			s.directivesByBot[key][i].UpdatedAt = time.Now().UTC()
			return true
		}
	}
	return false
}

func (s *Store) AppendEmotionBuffer(userID, botID, text string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + ":" + botID
	s.emotionBuffers[key] = append(s.emotionBuffers[key], text)
	return len(s.emotionBuffers[key])
}

func (s *Store) GetEmotionBuffer(userID, botID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.emotionBuffers[userID+":"+botID]...)
}

func (s *Store) ClearEmotionBuffer(userID, botID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.emotionBuffers, userID+":"+botID)
}

func (s *Store) SaveEmotionContext(userID, botID string, ctx EmotionContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx.UpdatedAt = time.Now().UTC()
	s.emotionCtxs[userID+":"+botID] = ctx
}

func (s *Store) GetEmotionContext(userID, botID string) (EmotionContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.emotionCtxs[userID+":"+botID]
	return ctx, ok
}

func previewText(msg Message) string {
	if msg.Content.Type == "image" {
		return "[图片]"
	}
	if msg.Content.Type == "location" {
		if msg.Content.Name != "" {
			return "📍" + msg.Content.Name
		}
		return "📍位置"
	}
	return msg.Content.Text
}

func splitSegments(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func dedup(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func NewID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
