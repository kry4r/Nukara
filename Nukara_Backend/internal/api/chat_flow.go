package api

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"strings"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/store"
)

var (
	errInvalidContent       = errors.New("invalid content")
	errConversationNotFound = errors.New("conversation not found")
	errBotNotFound          = errors.New("bot not found")
)

type chatMessageRequest struct {
	ConversationID  string
	ClientMessageID string
	Content         store.MessageContent
}

type chatMessageResult struct {
	Conversation store.Conversation
	Bot          store.Bot
	UserMessage  store.Message
	BotMessage   store.Message
	StatusEmoji  string
	StatusText   string
}

type botStatusValue struct {
	Emoji string
	Text  string
}

var botStatuses = []botStatusValue{
	{Emoji: "🙂", Text: "在线"},
	{Emoji: "💭", Text: "在想你"},
	{Emoji: "☕️", Text: "慢慢聊"},
	{Emoji: "🎵", Text: "听歌中"},
	{Emoji: "🌙", Text: "夜聊"},
	{Emoji: "✨", Text: "灵感中"},
	{Emoji: "📖", Text: "读书"},
	{Emoji: "🎨", Text: "创作中"},
}

func (s *Server) processChatMessage(userID string, req chatMessageRequest) (chatMessageResult, error) {
	conv, found := s.store.GetConversation(userID, strings.TrimSpace(req.ConversationID))
	if !found {
		return chatMessageResult{}, errConversationNotFound
	}

	bot, found := s.store.GetBot(userID, conv.BotID)
	if !found {
		return chatMessageResult{}, errBotNotFound
	}

	content, prompt, err := normalizeMessageContent(req.Content)
	if err != nil {
		return chatMessageResult{}, err
	}

	userMessage, ok := s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "user",
		ContentType:    content.Type,
		Content:        content,
	})
	if !ok {
		return chatMessageResult{}, errConversationNotFound
	}

	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	var userStatusStr string
	if us, ok := s.store.GetUserStatus(userID); ok && (us.Emoji != "" || us.Text != "") {
		userStatusStr = us.Emoji + " " + us.Text
	}
	directives := s.store.ListDirectives(userID, bot.ID, "active")
	sysCtx := agent.BuildSystemContext(bot, directives, userStatusStr)
	raw, err := s.agent.Chat(context.Background(), convID, "default", prompt, sysCtx)
	if err != nil {
		log.Printf("[chat_flow] agent.Chat failed: %v", err)
		raw = fmt.Sprintf("%s：我记住了你说的。要不要继续聊聊？", bot.Name)
	}
	reply, statusEmoji, statusText := agent.ExtractStatus(raw, "")
	reply, emotion := agent.ExtractEmotion(reply)
	if statusEmoji == "" || statusEmoji == "☕️" {
		statusEmoji = agent.EmotionDefaultEmoji(emotion)
	}
	botMessage, ok := s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "bot",
		ContentType:    "text",
		Content: store.MessageContent{
			Type: "text",
			Text: reply,
		},
		EmotionTag: emotion,
	})
	if !ok {
		return chatMessageResult{}, errConversationNotFound
	}

	s.store.SaveBotStatus(userID, bot.ID, statusEmoji, statusText)

	// Async memory extraction + impression update + periodic consolidation
	go func(uID string, b store.Bot, c store.Conversation, userPrompt, botReply string) {
		nbConvID := agent.NanobotConvID(uID, b.ID, c.ID)
		sysCtx := agent.BuildSystemContext(b, nil)

		existing := s.store.ListDirectives(uID, b.ID, "active")
		var existingContents []string
		for _, d := range existing {
			existingContents = append(existingContents, d.Content)
		}
		directives := s.agent.ExtractMemory(context.Background(), nbConvID, "default", userPrompt, botReply, existingContents, sysCtx)
		for _, e := range directives {
			if ok, _ := agent.ValidateDirective(e.Content); !ok {
				continue
			}
			switch e.Action {
			case "ADD", "UPDATE":
				if len(existing) < 20 {
					s.store.SaveDirective(store.Directive{
						UserID: uID, BotID: b.ID,
						Content: e.Content, Category: e.Category,
						Source: "conversation", Status: "active",
						OriginalMessage: userPrompt,
					})
				}
			case "REVOKE":
				for _, d := range existing {
					if d.Content == e.Content {
						s.store.RevokeDirective(uID, b.ID, d.ID)
					}
				}
			}
		}

		turnCount := s.store.IncrementTurnCount(uID, b.ID)
		if turnCount%3 == 0 {
			s.agent.UpdateImpression(context.Background(), nbConvID, "default", sysCtx)
		}
		if turnCount%20 == 0 {
			s.agent.ConsolidateMemory(context.Background(), nbConvID, "default", sysCtx)
		}
	}(userID, bot, conv, prompt, reply)

	return chatMessageResult{
		Conversation: conv,
		Bot:          bot,
		UserMessage:  userMessage,
		BotMessage:   botMessage,
		StatusEmoji:  statusEmoji,
		StatusText:   statusText,
	}, nil
}

func normalizeMessageContent(raw store.MessageContent) (store.MessageContent, string, error) {
	contentType := strings.TrimSpace(raw.Type)
	if contentType == "" {
		contentType = "text"
	}

	content := store.MessageContent{Type: contentType}
	switch contentType {
	case "text":
		content.Text = strings.TrimSpace(raw.Text)
		if content.Text == "" {
			return store.MessageContent{}, "", errInvalidContent
		}
		return content, content.Text, nil
	case "image":
		content.ImageBase64 = strings.TrimSpace(raw.ImageBase64)
		if content.ImageBase64 == "" {
			return store.MessageContent{}, "", errInvalidContent
		}
		return content, "用户发送了一张图片", nil
	case "location":
		if raw.Latitude == nil || raw.Longitude == nil {
			return store.MessageContent{}, "", errInvalidContent
		}
		content.Latitude = raw.Latitude
		content.Longitude = raw.Longitude
		content.Name = strings.TrimSpace(raw.Name)
		if content.Name == "" {
			content.Name = "位置"
		}
		return content, "用户发送了位置：" + content.Name, nil
	default:
		return store.MessageContent{}, "", errInvalidContent
	}
}

// emotionStatusMap maps emotion tags to contextually appropriate statuses.
var emotionStatusMap = map[string][]botStatusValue{
	"happy":   {{Emoji: "😊", Text: "开心"}, {Emoji: "🎵", Text: "哼歌中"}, {Emoji: "✨", Text: "心情好"}},
	"excited": {{Emoji: "🤩", Text: "超兴奋"}, {Emoji: "✨", Text: "灵感爆发"}, {Emoji: "🎉", Text: "好开心"}},
	"love":    {{Emoji: "💕", Text: "心动中"}, {Emoji: "🥰", Text: "甜蜜"}, {Emoji: "💭", Text: "在想你"}},
	"sad":     {{Emoji: "🌧", Text: "有点低落"}, {Emoji: "💭", Text: "在想事情"}, {Emoji: "🤗", Text: "想陪你"}},
	"angry":   {{Emoji: "😤", Text: "有点生气"}, {Emoji: "💭", Text: "冷静中"}, {Emoji: "🍵", Text: "喝茶消气"}},
	"anxious": {{Emoji: "😟", Text: "有点担心"}, {Emoji: "💭", Text: "在想你"}, {Emoji: "🤗", Text: "想陪你"}},
	"gentle":  {{Emoji: "☕️", Text: "慢慢聊"}, {Emoji: "🌸", Text: "温柔模式"}, {Emoji: "📖", Text: "静静陪你"}},
}

func selectBotStatus(emotion, conversationID string) botStatusValue {
	// Try emotion-specific statuses first.
	if pool, ok := emotionStatusMap[emotion]; ok && len(pool) > 0 {
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(conversationID + emotion))
		return pool[int(hasher.Sum32())%len(pool)]
	}
	// Fallback to generic statuses.
	if len(botStatuses) == 0 {
		return botStatusValue{Emoji: "🙂", Text: "在线"}
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(conversationID))
	return botStatuses[int(hasher.Sum32())%len(botStatuses)]
}

func (s *Server) generateStarterMessage(userID string, bot store.Bot, convID string) string {
	nbConvID := agent.NanobotConvID(userID, bot.ID, convID)
	sysCtx := agent.BuildSystemContext(bot, nil)
	reply, err := s.agent.GenerateStarter(context.Background(), nbConvID, "default", sysCtx)
	if err != nil {
		log.Printf("[chat_flow] GenerateStarter failed: %v", err)
		return fmt.Sprintf("你好呀～我是%s，很高兴认识你。", bot.Name)
	}
	text, _ := agent.ExtractEmotion(reply)
	if strings.TrimSpace(text) == "" {
		return fmt.Sprintf("你好呀～我是%s，很高兴认识你。", bot.Name)
	}
	return text
}

func wsMessageEvent(msg store.Message) map[string]any {
	payload := map[string]any{
		"type":            "message",
		"msg_id":          msg.ID,
		"conversation_id": msg.ConversationID,
		"sender_type":     msg.SenderType,
		"content":         msg.Content,
		"is_proactive":    msg.IsProactive,
		"timestamp":       msg.CreatedAt.Unix(),
	}
	if msg.EmotionTag != "" {
		payload["emotion_tag"] = msg.EmotionTag
	}
	return payload
}

func wsBotStatusEvent(conversationID, emoji, text string) map[string]any {
	return map[string]any{
		"type":            "bot_status_update",
		"conversation_id": conversationID,
		"emoji":           emoji,
		"text":            text,
		"status": map[string]any{
			"emoji": emoji,
			"text":  text,
		},
	}
}

func wsProactiveEvent(msg store.Message) map[string]any {
	payload := map[string]any{
		"type":            "proactive_message",
		"conversation_id": msg.ConversationID,
		"msg_id":          msg.ID,
		"content":         msg.Content,
		"timestamp":       msg.CreatedAt.Unix(),
	}
	if msg.EmotionTag != "" {
		payload["emotion_context"] = msg.EmotionTag
	}
	return payload
}
