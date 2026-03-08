package agentx

import "nukara/backend/internal/agentx/llm"

type TurnRequest struct {
	UserID                 string
	BotID                  string
	ConversationID         string
	ProviderConversationID string
	AggregatedText         string
	UserMessageIDs         []string
	SystemContext          map[string]any
	SystemPrompt           string
	Purpose                string
	History                []llm.ChatMessage
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
