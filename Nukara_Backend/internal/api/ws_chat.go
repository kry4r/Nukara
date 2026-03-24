package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/postprocess"
	"nukara/backend/internal/agentx/subtasks"
	"nukara/backend/internal/store"
)

const wsIdleTimeout = 5 * time.Minute
const wsPresenceTTL = 90 * time.Second

type wsIncomingMessage struct {
	Type            string `json:"type"`
	ConversationID  string `json:"conversation_id"`
	ClientMessageID string `json:"client_msg_id"`
	Content         struct {
		Type        string   `json:"type"`
		Text        string   `json:"text"`
		ImageBase64 string   `json:"image_base64"`
		Latitude    *float64 `json:"latitude"`
		Longitude   *float64 `json:"longitude"`
		Name        string   `json:"name"`
	} `json:"content"`
}

func (s *Server) handleWSChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authUserID(w, r)
	if !ok {
		return
	}

	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	session := s.wsHub.register(userID, conn)
	s.store.TouchWSPresence(userID, wsPresenceTTL)
	aggregator := newMessageAggregator(s.wsQueue)
	defer func() {
		aggregator.closeAll()
		s.wsHub.unregister(session)
		_ = conn.Close()
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
		text, err := conn.ReadText()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("[ws-chat] user=%s connection closed", userID)
				return
			}
			log.Printf("[ws-chat] user=%s read error: %v", userID, err)
			return
		}

		var msg wsIncomingMessage
		if err := json.Unmarshal([]byte(text), &msg); err != nil {
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "invalid json payload"})
			continue
		}

		if _, exists := s.store.FindUserByID(userID); !exists {
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "session invalidated"})
			return
		}

		switch strings.TrimSpace(msg.Type) {
		case "message":
			s.handleWSChatMessage(userID, msg, aggregator)
		case "typing_start", "typing_stop":
			s.store.TouchWSPresence(userID, wsPresenceTTL)
			s.handleWSTypingEvent(userID, msg)
		case "ping":
			s.store.TouchWSPresence(userID, wsPresenceTTL)
			_ = conn.WriteJSON(map[string]any{"type": "pong", "timestamp": time.Now().Unix()})
		default:
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "unsupported event type"})
		}
	}
}

func (s *Server) handleWSTypingEvent(_ string, _ wsIncomingMessage) {
	// Typing signals are currently advisory only.
}

func (s *Server) handleWSChatMessage(userID string, message wsIncomingMessage, aggregator *messageAggregator) {
	conv, found := s.store.GetConversation(userID, strings.TrimSpace(message.ConversationID))
	if !found {
		s.wsHub.publishToUser(userID, map[string]any{"type": "error", "message": errConversationNotFound.Error()})
		return
	}
	bot, found := s.store.GetBot(userID, conv.BotID)
	if !found {
		s.wsHub.publishToUser(userID, map[string]any{"type": "error", "message": errBotNotFound.Error()})
		return
	}

	content, prompt, err := normalizeMessageContent(store.MessageContent{
		Type:        message.Content.Type,
		Text:        message.Content.Text,
		ImageBase64: message.Content.ImageBase64,
		Latitude:    message.Content.Latitude,
		Longitude:   message.Content.Longitude,
		Name:        message.Content.Name,
	})
	if err != nil {
		s.wsHub.publishToUser(userID, map[string]any{"type": "error", "message": err.Error()})
		return
	}

	userMessage, ok := s.store.SaveMessage(userID, store.Message{
		ConversationID: conv.ID,
		SenderType:     "user",
		ContentType:    content.Type,
		Content:        content,
	})
	if !ok {
		s.wsHub.publishToUser(userID, map[string]any{"type": "error", "message": errConversationNotFound.Error()})
		return
	}

	s.store.TouchWSPresence(userID, wsPresenceTTL)
	s.store.SetLastUserMessageAt(userID, userMessage.CreatedAt)

	s.wsHub.publishToUser(userID, map[string]any{
		"type":          "ack",
		"client_msg_id": fallback(message.ClientMessageID, userMessage.ID),
		"server_msg_id": userMessage.ID,
		"timestamp":     userMessage.CreatedAt.Unix(),
	})

	s.bufferAndAnalyzeEmotion(userID, bot, conv, prompt)

	var userStatusStr string
	if us, ok := s.store.GetUserStatus(userID); ok && (us.Emoji != "" || us.Text != "") {
		userStatusStr = us.Emoji + " " + us.Text
	}
	directives := s.store.ListDirectives(userID, bot.ID, "active")
	sysCtx := agent.BuildSystemContext(bot, directives, userStatusStr)
	aggregator.add(aggregateInput{
		UserID:        userID,
		Conversation:  conv,
		Bot:           bot,
		Prompt:        prompt,
		UserMessageID: userMessage.ID,
		SystemContext: sysCtx,
	})
}

func (s *Server) processQueuedTurn(turn queuedTurn) {
	if s.runtime == nil {
		s.wsHub.publishToUser(turn.UserID, map[string]any{"type": "error", "message": "chat runtime unavailable"})
		return
	}

	replyID := fmt.Sprintf("reply-%d", time.Now().UnixNano())
	s.wsHub.publishToUser(turn.UserID, map[string]any{
		"type":            "typing",
		"conversation_id": turn.Conversation.ID,
		"is_typing":       true,
	})
	s.wsHub.publishToUser(turn.UserID, map[string]any{
		"type":            "stream_start",
		"reply_id":        replyID,
		"conversation_id": turn.Conversation.ID,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	deltaCh, finalCh, err := s.runtime.StreamTurn(ctx, s.newTurnRequest(
		turn.UserID,
		turn.Bot.ID,
		turn.Conversation.ID,
		turn.AggregatedText,
		turn.UserMessageIDs,
		turn.SystemContext,
	))
	if err != nil {
		s.wsHub.publishToUser(turn.UserID, map[string]any{"type": "error", "message": err.Error()})
		s.wsHub.publishToUser(turn.UserID, map[string]any{
			"type":            "typing",
			"conversation_id": turn.Conversation.ID,
			"is_typing":       false,
		})
		return
	}

	streamSanitizer := postprocess.NewStreamSanitizer()
	chunkCount := 0
	for delta := range deltaCh {
		visibleDelta := streamSanitizer.Push(delta.Delta)
		if strings.TrimSpace(visibleDelta) == "" {
			continue
		}
		chunkCount++
		s.wsHub.publishToUser(turn.UserID, map[string]any{
			"type":            "stream_chunk",
			"reply_id":        replyID,
			"conversation_id": turn.Conversation.ID,
			"delta":           visibleDelta,
		})
	}

	finalTurn, ok := <-finalCh
	if !ok {
		finalTurn = agentx.FinalTurn{}
	}
	if len(finalTurn.Segments) == 0 {
		finalTurn.Segments = []agentx.FinalSegment{{
			Text:        "我在呢，刚刚有点走神了，我们继续聊。",
			EmotionTag:  "gentle",
			StatusEmoji: "💭",
			StatusText:  "在想你",
		}}
	}
	log.Printf("[ws-stream] user=%s conv=%s chunks=%d segments=%d", turn.UserID, turn.Conversation.ID, chunkCount, len(finalTurn.Segments))

	s.wsHub.publishToUser(turn.UserID, map[string]any{
		"type":            "stream_end",
		"reply_id":        replyID,
		"conversation_id": turn.Conversation.ID,
	})

	persistedSegments := make([]string, 0, len(finalTurn.Segments))
	for _, segment := range finalTurn.Segments {
		for _, split := range postprocess.SplitSegments(segment.Text, 80) {
			safeText := postprocess.SanitizeVisible(split)
			if strings.TrimSpace(safeText) == "" {
				continue
			}
			msg, _ := s.store.SaveMessage(turn.UserID, store.Message{
				ConversationID: turn.Conversation.ID,
				SenderType:     "bot",
				ContentType:    "text",
				Content:        store.MessageContent{Type: "text", Text: safeText},
				EmotionTag:     segment.EmotionTag,
			})
			persistedSegments = append(persistedSegments, safeText)
			payload := wsMessageEvent(msg)
			payload["reply_id"] = replyID
			s.wsHub.publishToUser(turn.UserID, payload)
		}
	}

	last := finalTurn.Segments[len(finalTurn.Segments)-1]
	s.store.SaveBotStatus(turn.UserID, turn.Bot.ID, last.StatusEmoji, last.StatusText)
	createdTurn, err := s.store.CreateTurn(store.AgentTurn{
		UserID:             turn.UserID,
		BotID:              turn.Bot.ID,
		ConversationID:     turn.Conversation.ID,
		UserMessageIDs:     turn.UserMessageIDs,
		AggregatedUserText: turn.AggregatedText,
		BotReplyText:       strings.Join(persistedSegments, "\n"),
	})
	if err != nil {
		log.Printf("[ws-chat] create turn failed: %v", err)
	} else {
		go s.runTurnSubtasks(turn, createdTurn.ID, strings.Join(persistedSegments, "\n"))
	}
	s.wsHub.publishToUser(turn.UserID, map[string]any{
		"type":            "typing",
		"conversation_id": turn.Conversation.ID,
		"is_typing":       false,
	})
}

func (s *Server) runTurnSubtasks(turn queuedTurn, turnID, botReplyText string) {
	if s.subtasks == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := s.subtasks.Run(ctx, subtasks.Input{
		UserID:         turn.UserID,
		BotID:          turn.Bot.ID,
		ConversationID: turn.Conversation.ID,
		TurnID:         turnID,
		UserText:       turn.AggregatedText,
		BotText:        botReplyText,
	})
	if err != nil {
		log.Printf("[ws-chat] subtasks failed: user=%s conv=%s err=%v", turn.UserID, turn.Conversation.ID, err)
		return
	}
	if result.PersonaUpdated {
		s.wsHub.publishToUser(turn.UserID, map[string]any{
			"type":      "bot_persona_updated",
			"bot_id":    turn.Bot.ID,
			"patch":     result.Patch,
			"summary":   result.PatchSummary,
			"timestamp": time.Now().UTC().Unix(),
		})
	}
	if result.SavedMemoryCount > 0 {
		log.Printf("[ws-chat] memory saved: user=%s bot=%s count=%d", turn.UserID, turn.Bot.ID, result.SavedMemoryCount)
		s.wsHub.publishToUser(turn.UserID, map[string]any{
			"type":         "bot_memory_saved",
			"bot_id":       turn.Bot.ID,
			"memory_count": result.SavedMemoryCount,
			"timestamp":    time.Now().UTC().Unix(),
		})
	}
}
