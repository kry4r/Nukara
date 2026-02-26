package store

import "time"

// DataStore captures the server-facing persistence contract.
// In-memory and postgres-backed stores both implement this interface.
type DataStore interface {
	IncrementRequests()
	SnapshotMetrics() Metrics
	SetWSConnections(count int)

	SaveSMSCode(phone, purpose, code string, ttl time.Duration)
	ValidateSMSCode(phone, purpose, code string) bool
	FindUserByPhone(phone string) (User, bool)
	CreateUser(phone, nickname string) (User, error)

	UpsertDeviceToken(userID, token, platform string)
	GetDeviceToken(userID string) (DeviceToken, bool)
	UpdateNotificationSettings(userID string, input NotificationSettings) NotificationSettings
	GetNotificationSettings(userID string) NotificationSettings

	CreateBot(userID string, bot Bot) Bot
	ListBots(userID string) []Bot
	GetBot(userID, botID string) (Bot, bool)
	GetBotState(userID, botID string) (BotState, bool)
	AppendBotPersona(userID, botID string, speakingAdds, backgroundAdds, traitAdds []string, gender *string) (Bot, bool)

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

	// ListAllUserIDs returns all registered user IDs for scheduler scanning.
	ListAllUserIDs() []string
}
