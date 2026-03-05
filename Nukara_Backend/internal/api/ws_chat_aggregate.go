package api

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"nukara/backend/internal/store"
)

const (
	aggregateWindow      = 3 * time.Second
	hardAggregateLimit   = 8 * time.Second
	maxAggregateMessages = 10
)

type aggregateInput struct {
	UserID        string
	Conversation  store.Conversation
	Bot           store.Bot
	Prompt        string
	UserMessageID string
	SystemContext map[string]any
}

type messageAggregator struct {
	mu      sync.Mutex
	buffers map[string]*aggregateBuffer
	queue   *wsConversationQueue
}

type aggregateBuffer struct {
	userID         string
	conversation   store.Conversation
	bot            store.Bot
	prompts        []string
	userMessageIDs []string
	systemContext  map[string]any
	timer          *time.Timer
	hardTimer      *time.Timer
}

func newMessageAggregator(queue *wsConversationQueue) *messageAggregator {
	return &messageAggregator{
		buffers: make(map[string]*aggregateBuffer),
		queue:   queue,
	}
}

func (ma *messageAggregator) add(in aggregateInput) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	buf, ok := ma.buffers[in.Conversation.ID]
	if ok {
		buf.prompts = append(buf.prompts, in.Prompt)
		buf.userMessageIDs = append(buf.userMessageIDs, in.UserMessageID)
		buf.systemContext = in.SystemContext

		if len(buf.prompts) >= maxAggregateMessages {
			buf.timer.Stop()
			buf.hardTimer.Stop()
			delete(ma.buffers, in.Conversation.ID)
			go ma.flushBuffer(buf)
			return
		}

		buf.timer.Reset(aggregateWindow)
		return
	}

	buf = &aggregateBuffer{
		userID:         in.UserID,
		conversation:   in.Conversation,
		bot:            in.Bot,
		prompts:        []string{in.Prompt},
		userMessageIDs: []string{in.UserMessageID},
		systemContext:  in.SystemContext,
	}

	buf.timer = time.AfterFunc(aggregateWindow, func() {
		ma.flush(in.Conversation.ID)
	})
	buf.hardTimer = time.AfterFunc(hardAggregateLimit, func() {
		ma.flush(in.Conversation.ID)
	})

	ma.buffers[in.Conversation.ID] = buf
}

func (ma *messageAggregator) flush(conversationID string) {
	ma.mu.Lock()
	buf, ok := ma.buffers[conversationID]
	if !ok {
		ma.mu.Unlock()
		return
	}
	delete(ma.buffers, conversationID)
	ma.mu.Unlock()

	ma.flushBuffer(buf)
}

func (ma *messageAggregator) closeAll() {
	ma.mu.Lock()
	buffers := make([]*aggregateBuffer, 0, len(ma.buffers))
	for conversationID, buf := range ma.buffers {
		delete(ma.buffers, conversationID)
		buffers = append(buffers, buf)
	}
	ma.mu.Unlock()

	for _, buf := range buffers {
		ma.flushBuffer(buf)
	}
}

func (ma *messageAggregator) flushBuffer(buf *aggregateBuffer) {
	buf.timer.Stop()
	buf.hardTimer.Stop()

	prompt := formatAggregatedPrompt(buf.prompts)
	if strings.TrimSpace(prompt) == "" {
		return
	}

	log.Printf("[ws-aggregate] flush user=%s conv=%s prompts=%d", buf.userID, buf.conversation.ID, len(buf.prompts))
	ma.queue.enqueue(queuedTurn{
		UserID:         buf.userID,
		Conversation:   buf.conversation,
		Bot:            buf.bot,
		AggregatedText: prompt,
		UserMessageIDs: append([]string(nil), buf.userMessageIDs...),
		SystemContext:  buf.systemContext,
	})
}

func formatAggregatedPrompt(prompts []string) string {
	if len(prompts) == 0 {
		return ""
	}
	if len(prompts) == 1 {
		return strings.TrimSpace(prompts[0])
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[用户连续发送了%d条消息]\n", len(prompts))
	for i, prompt := range prompts {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, strings.TrimSpace(prompt))
	}
	return strings.TrimSpace(sb.String())
}
