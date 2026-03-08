package store

import "time"

// DataStore captures the server-facing persistence contract.
// In-memory and postgres-backed stores both implement this interface.
type DataStore interface {
	IncrementRequests()
	SnapshotMetrics() Metrics
	SetWSConnections(count int)
	TouchWSPresence(userID string, ttl time.Duration)
	IsUserWSOnline(userID string) bool
	SetLastUserMessageAt(userID string, at time.Time)
	GetLastUserMessageAt(userID string) (time.Time, bool)

	SaveEmailCode(email, purpose, code string, ttl time.Duration)
	ValidateEmailCode(email, purpose, code string) bool
	FindUserByEmail(email string) (User, bool)
	FindUserByID(id string) (User, bool)
	CreateUser(email, nickname string) (User, error)

	UpsertDeviceToken(userID, token, platform string)
	GetDeviceToken(userID string) (DeviceToken, bool)
	UpdateNotificationSettings(userID string, input NotificationSettings) NotificationSettings
	GetNotificationSettings(userID string) NotificationSettings

	CreateBot(userID string, bot Bot) Bot
	ListBots(userID string) []Bot
	GetBot(userID, botID string) (Bot, bool)
	GetBotState(userID, botID string) (BotState, bool)
	UpdateBot(userID, botID string, patch Bot) (Bot, bool)
	AppendBotPersona(userID, botID string, speakingAdds, backgroundAdds, traitAdds []string, gender *string) (Bot, bool)
	ApplyBotPersonaPatch(userID, botID string, input PersonaPatchInput) (Bot, bool)

	ListConversations(userID string) []Conversation
	FindConversationByBot(userID, botID string) (Conversation, bool)
	EnsureConversation(userID, botID, botName, botAvatar, botAvatarBase64 string) Conversation
	GetConversation(userID, conversationID string) (Conversation, bool)
	ListMessages(userID, conversationID string, limit int) ([]Message, bool)
	MarkConversationRead(userID, conversationID string) bool
	SaveMessage(userID string, message Message) (Message, bool)
	SaveBotStatus(userID, botID, emoji, text string)
	IncrementTurnCount(userID, botID string) int

	SaveUserStatus(userID, emoji, text string)
	GetUserStatus(userID string) (UserStatus, bool)

	AddProactiveLog(log ProactiveLog) ProactiveLog
	ListProactiveLogs(userID string, limit int) []ProactiveLog

	SaveDirective(d Directive) Directive
	ListDirectives(userID, botID, status string) []Directive
	RevokeDirective(userID, botID, directiveID string) bool
	SetUserProviderSetting(userID, providerID, model string) error
	GetUserProviderSetting(userID string) (providerID, model string, ok bool)
	SetBotProviderOverride(userID, botID, providerID, model string) error
	GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool)
	SetSystemSetting(key, value string) error
	GetSystemSetting(key string) (value string, ok bool)
	CreateTurn(turn AgentTurn) (AgentTurn, error)
	UpsertCompact(conversationID, compactJSON, untilTurnID string) error
	GetConversationCompact(conversationID string) (ConversationCompact, bool)
	UpsertMemoryItem(item MemoryItem) (MemoryItem, error)
	GetMemoryItem(memoryID string) (MemoryItem, bool)
	ListMemoryItems(userID, botID string, limit int) []MemoryItem

	// Emotion tracking
	AppendEmotionBuffer(userID, botID, text string) int // returns buffer length
	GetEmotionBuffer(userID, botID string) []string
	ClearEmotionBuffer(userID, botID string)
	SaveEmotionContext(userID, botID string, ctx EmotionContext)
	GetEmotionContext(userID, botID string) (EmotionContext, bool)

	// ListAllUserIDs returns all registered user IDs for scheduler scanning.
	ListAllUserIDs() []string
}
