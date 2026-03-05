package api

import (
	"sync"

	"nukara/backend/internal/store"
)

type queuedTurn struct {
	UserID         string
	Conversation   store.Conversation
	Bot            store.Bot
	AggregatedText string
	UserMessageIDs []string
	SystemContext  map[string]any
}

type wsConversationQueue struct {
	mu      sync.Mutex
	runners map[string]*convRunner
	server  *Server
}

type convRunner struct {
	queue chan queuedTurn
}

func newWSConversationQueue(server *Server) *wsConversationQueue {
	return &wsConversationQueue{
		runners: make(map[string]*convRunner),
		server:  server,
	}
}

func (q *wsConversationQueue) enqueue(turn queuedTurn) {
	key := turn.UserID + ":" + turn.Conversation.ID

	q.mu.Lock()
	runner, ok := q.runners[key]
	if !ok {
		runner = &convRunner{
			queue: make(chan queuedTurn, 32),
		}
		q.runners[key] = runner
		go q.run(key, runner)
	}
	q.mu.Unlock()

	runner.queue <- turn
}

func (q *wsConversationQueue) run(key string, runner *convRunner) {
	for turn := range runner.queue {
		q.server.processQueuedTurn(turn)
	}
}
