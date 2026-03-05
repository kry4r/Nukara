package agentx

type TurnRequest struct {
	UserID         string
	BotID          string
	ConversationID string
	AggregatedText string
	UserMessageIDs []string
	SystemContext  map[string]any
}

type StreamDelta struct {
	Delta string
}

type FinalTurn struct {
	Segments []FinalSegment
}

type FinalSegment struct {
	Text        string
	EmotionTag  string
	StatusEmoji string
	StatusText  string
}
