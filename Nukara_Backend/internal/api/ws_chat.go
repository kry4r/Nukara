package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/store"
)

const wsIdleTimeout = 5 * time.Minute

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

// convSubscription tracks a persistent nanobot event consumer for one conversation.
type convSubscription struct {
	nanobotConvID string
	eventCh       <-chan agent.NanobotEvent
	detached      chan struct{}
	lastUserPrompt string // set by handleWSChatMessage for memory extraction
}

// sessionConsumers manages per-conversation event consumers for a WS session.
type sessionConsumers struct {
	mu     sync.Mutex
	active map[string]*convSubscription // keyed by nukara conv.ID
	server *Server
}

func newSessionConsumers(s *Server) *sessionConsumers {
	return &sessionConsumers{
		active: make(map[string]*convSubscription),
		server: s,
	}
}

// ensure starts a persistent event consumer for the conversation if not already running.
func (sc *sessionConsumers) ensure(userID string, conv store.Conversation, bot store.Bot) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if _, exists := sc.active[conv.ID]; exists {
		return
	}

	nanobotConvID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	eventCh := sc.server.agent.Subscribe(nanobotConvID)
	sub := &convSubscription{
		nanobotConvID: nanobotConvID,
		eventCh:       eventCh,
		detached:      make(chan struct{}),
	}
	sc.active[conv.ID] = sub

	go sc.server.consumeNanobotEvents(userID, conv, bot, sub)
}

// setLastUserPrompt updates the last user prompt for memory extraction.
func (sc *sessionConsumers) setLastUserPrompt(convID, prompt string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sub, ok := sc.active[convID]; ok {
		sub.lastUserPrompt = prompt
	}
}

// closeAll detaches all active consumers. Detached consumers continue saving
// messages to DB until the current reply completes, then self-cleanup.
func (sc *sessionConsumers) closeAll() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for id, sub := range sc.active {
		select {
		case <-sub.detached:
		default:
			close(sub.detached)
		}
		delete(sc.active, id)
	}
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
	consumers := newSessionConsumers(s)
	defer func() {
		consumers.closeAll()
		s.wsHub.unregister(session)
		_ = conn.Close()
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(wsIdleTimeout))

		text, err := conn.ReadText()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("[ws-chat] user=%s connection closed (EOF)", userID)
				return
			}
			log.Printf("[ws-chat] user=%s read error: %v", userID, err)
			return
		}

		var msg wsIncomingMessage
		if err := json.Unmarshal([]byte(text), &msg); err != nil {
			log.Printf("[ws-chat] user=%s invalid json: %s", userID, text[:min(len(text), 200)])
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "invalid json payload"})
			continue
		}

		log.Printf("[ws-chat] user=%s recv type=%s conv=%s", userID, msg.Type, msg.ConversationID)

		switch strings.TrimSpace(msg.Type) {
		case "message":
			s.handleWSChatMessage(userID, msg, consumers)
		case "typing_start", "typing_stop":
			s.handleWSTypingEvent(userID, msg)
		case "ping":
			_ = conn.WriteJSON(map[string]any{"type": "pong", "timestamp": time.Now().Unix()})
		default:
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "unsupported event type"})
		}
	}
}

func (s *Server) handleWSTypingEvent(userID string, message wsIncomingMessage) {
	conv, found := s.store.GetConversation(userID, strings.TrimSpace(message.ConversationID))
	if !found {
		return
	}
	bot, found := s.store.GetBot(userID, conv.BotID)
	if !found {
		return
	}
	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	s.agent.SendTypingEvent(convID, "default", message.Type)
}

func (s *Server) handleWSChatMessage(userID string, message wsIncomingMessage, consumers *sessionConsumers) {
	log.Printf("[ws-chat] handleWSChatMessage user=%s conv=%s content_type=%s", userID, message.ConversationID, message.Content.Type)

	conv, found := s.store.GetConversation(userID, strings.TrimSpace(message.ConversationID))
	if !found {
		log.Printf("[ws-chat] conversation not found: user=%s conv=%s", userID, message.ConversationID)
		s.wsHub.publishToUser(userID, map[string]any{"type": "error", "message": errConversationNotFound.Error()})
		return
	}
	bot, found := s.store.GetBot(userID, conv.BotID)
	if !found {
		log.Printf("[ws-chat] bot not found: user=%s bot=%s", userID, conv.BotID)
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

	s.wsHub.publishToUser(userID, map[string]any{
		"type":          "ack",
		"client_msg_id": fallback(message.ClientMessageID, userMessage.ID),
		"server_msg_id": userMessage.ID,
		"timestamp":     userMessage.CreatedAt.Unix(),
	})

	// Ensure persistent event consumer for this conversation
	consumers.ensure(userID, conv, bot)
	consumers.setLastUserPrompt(conv.ID, prompt)

	// Forward message to nanobot (non-blocking — aggregator will batch if needed)
	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	var userStatusStr string
	if us, ok := s.store.GetUserStatus(userID); ok && (us.Emoji != "" || us.Text != "") {
		userStatusStr = us.Emoji + " " + us.Text
	}
	wsDirectives := s.store.ListDirectives(userID, bot.ID, "active")
	sysCtx := agent.BuildSystemContext(bot, wsDirectives, userStatusStr)
	log.Printf("[ws-chat] forwarding to nanobot: convID=%s prompt=%s", convID, prompt[:min(len(prompt), 80)])
	if err := s.agent.SendChatMessage(convID, "default", prompt, sysCtx); err != nil {
		log.Printf("[ws-chat] send to nanobot failed: %v", err)
		s.wsHub.publishToUser(userID, map[string]any{"type": "error", "message": "send failed: " + err.Error()})
	} else {
		log.Printf("[ws-chat] forwarded to nanobot OK: convID=%s", convID)
	}
}

// consumeNanobotEvents reads events from nanobot and translates them to iOS WS protocol.
// When the client disconnects (sub.detached closed), the goroutine continues saving
// messages to DB (skipping WS publish) until the reply completes, then self-unsubscribes.
func (s *Server) consumeNanobotEvents(userID string, conv store.Conversation, bot store.Bot, sub *convSubscription) {
	var replyGroupID string
	var lastEmotion string
	var lastStatusEmoji, lastStatusText string
	var accumulatedReply string
	detached := false

	// Safety timeout so detached consumers don't leak if reply-end never arrives.
	timeout := make(chan struct{})
	defer func() {
		if detached {
			s.agent.Unsubscribe(sub.nanobotConvID, sub.eventCh)
			log.Printf("[ws-chat] detached consumer cleaned up: user=%s conv=%s", userID, conv.ID)
		}
	}()

	for {
		// Check detach status (non-blocking)
		if !detached {
			select {
			case <-sub.detached:
				detached = true
				log.Printf("[ws-chat] consumer detached, continuing for DB saves: user=%s conv=%s", userID, conv.ID)
				go func() {
					time.Sleep(2 * time.Minute)
					close(timeout)
				}()
			default:
			}
		}

		// Read next event, or bail on timeout
		var evt agent.NanobotEvent
		if detached {
			select {
			case e, ok := <-sub.eventCh:
				if !ok {
					return
				}
				evt = e
			case <-timeout:
				return
			}
		} else {
			e, ok := <-sub.eventCh
			if !ok {
				return
			}
			evt = e
		}

		// Helper: only publish to WS if client is still connected
		publish := func(payload map[string]any) {
			if !detached {
				s.wsHub.publishToUser(userID, payload)
			}
		}

		switch evt.Type {
		case agent.EventAck:
			// nanobot ack, already sent our own

		case agent.EventTyping:
			isTyping := evt.IsTyping != nil && *evt.IsTyping
			publish(map[string]any{
				"type":            "typing",
				"conversation_id": conv.ID,
				"is_typing":       isTyping,
			})

		case agent.EventBotWaiting:
			publish(map[string]any{
				"type":            "typing",
				"conversation_id": conv.ID,
				"is_typing":       true,
			})

		case agent.EventMultiReplyStart:
			replyGroupID = evt.ReplyGroupID
			publish(map[string]any{
				"type":            "multi_reply_start",
				"conversation_id": conv.ID,
				"reply_group_id":  replyGroupID,
				"count":           evt.Count,
			})

		case agent.EventMessage:
			text := ""
			if evt.Content != nil {
				text = evt.Content.Text
			}
			if strings.TrimSpace(text) == "" {
				continue
			}

			cleanText, sEmoji, sText := agent.ExtractStatus(text, "")
			cleanText, emotion := agent.ExtractEmotion(cleanText)
			if sEmoji == "" || sEmoji == "☕️" {
				sEmoji = agent.EmotionDefaultEmoji(emotion)
			}
			lastEmotion = emotion
			lastStatusEmoji = sEmoji
			lastStatusText = sText
			accumulatedReply += cleanText + "\n"

			// Always save to DB, even when detached
			msg, _ := s.store.SaveMessage(userID, store.Message{
				ConversationID: conv.ID,
				SenderType:     "bot",
				ContentType:    "text",
				Content:        store.MessageContent{Type: "text", Text: cleanText},
				EmotionTag:     emotion,
				ReplyGroupID:   evt.ReplyGroupID,
				Sequence:       evt.Sequence,
			})

			payload := wsMessageEvent(msg)
			if evt.ReplyGroupID != "" {
				payload["reply_group_id"] = evt.ReplyGroupID
				payload["sequence"] = evt.Sequence
			}
			publish(payload)

		case agent.EventMultiReplyEnd:
			if lastStatusEmoji == "" {
				lastStatusEmoji = agent.EmotionDefaultEmoji(lastEmotion)
			}
			if lastStatusText == "" {
				lastStatusText = "聊天中"
			}
			s.store.SaveBotStatus(userID, bot.ID, lastStatusEmoji, lastStatusText)

			publish(map[string]any{
				"type":            "multi_reply_end",
				"conversation_id": conv.ID,
				"reply_group_id":  replyGroupID,
				"count":           evt.Count,
			})
			publish(wsBotStatusEvent(conv.ID, lastStatusEmoji, lastStatusText))
			publish(map[string]any{
				"type":            "typing",
				"conversation_id": conv.ID,
				"is_typing":       false,
			})

			// Async memory extraction + impression update + periodic consolidation
			go func(uID string, b store.Bot, c store.Conversation, userPrompt, botReply string) {
				nbConvID := agent.NanobotConvID(uID, b.ID, c.ID)
				sysCtx := agent.BuildSystemContext(b, nil)

				existing := s.store.ListDirectives(uID, b.ID, "active")
				var existingContents []string
				for _, d := range existing {
					existingContents = append(existingContents, d.Content)
				}
				directives := s.agent.ExtractMemory(context.Background(), nbConvID, "default", userPrompt, strings.TrimSpace(botReply), existingContents, sysCtx)
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
			}(userID, bot, conv, sub.lastUserPrompt, accumulatedReply)

			replyGroupID = ""
			lastEmotion = ""
			lastStatusEmoji = ""
			lastStatusText = ""
			accumulatedReply = ""

			// If detached, reply is done — cleanup
			if detached {
				return
			}

		case agent.EventProactive:
			text := ""
			if evt.Content != nil {
				text = evt.Content.Text
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			// Always save to DB
			msg, _ := s.store.SaveMessage(userID, store.Message{
				ConversationID: conv.ID,
				SenderType:     "bot",
				ContentType:    "text",
				Content:        store.MessageContent{Type: "text", Text: text},
				IsProactive:    true,
				EmotionTag:     "gentle",
			})
			publish(wsProactiveEvent(msg))

		case agent.EventError:
			publish(map[string]any{
				"type":    "error",
				"message": evt.Message,
			})

		case agent.EventPong:
			// ignore
		}
	}
}
