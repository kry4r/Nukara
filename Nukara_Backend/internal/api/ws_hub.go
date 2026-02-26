package api

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"nukara/backend/internal/store"
)

type wsSession struct {
	userID string
	conn   *wsConn
}

type wsHub struct {
	mu    sync.RWMutex
	store store.DataStore

	byUser map[string]map[*wsSession]struct{}
	total  int

	redis     *redis.Client
	redisChan string
	nodeID    string
}

type wsRedisEnvelope struct {
	Source  string         `json:"source"`
	UserID  string         `json:"user_id"`
	Payload map[string]any `json:"payload"`
}

func newWSHub(st store.DataStore, redisAddr string) *wsHub {
	hub := &wsHub{
		store:     st,
		byUser:    map[string]map[*wsSession]struct{}{},
		redisChan: "nukara:ws:broadcast",
		nodeID:    store.NewID(),
	}

	if strings.TrimSpace(redisAddr) != "" {
		client := redis.NewClient(&redis.Options{Addr: strings.TrimSpace(redisAddr)})
		if err := client.Ping(context.Background()).Err(); err != nil {
			log.Printf("ws redis disabled (ping failed): %v", err)
			_ = client.Close()
		} else {
			hub.redis = client
			hub.startSubscriber()
		}
	}

	return hub
}

func (h *wsHub) register(userID string, conn *wsConn) *wsSession {
	session := &wsSession{userID: userID, conn: conn}

	h.mu.Lock()
	userSessions := h.byUser[userID]
	if userSessions == nil {
		userSessions = map[*wsSession]struct{}{}
		h.byUser[userID] = userSessions
	}
	userSessions[session] = struct{}{}
	h.total++
	total := h.total
	h.mu.Unlock()

	h.store.SetWSConnections(total)
	return session
}

func (h *wsHub) unregister(session *wsSession) {
	if session == nil {
		return
	}

	h.mu.Lock()
	userSessions := h.byUser[session.userID]
	if _, exists := userSessions[session]; exists {
		delete(userSessions, session)
		if len(userSessions) == 0 {
			delete(h.byUser, session.userID)
		}
		if h.total > 0 {
			h.total--
		}
	}
	total := h.total
	h.mu.Unlock()

	h.store.SetWSConnections(total)
}

func (h *wsHub) publishToUser(userID string, payload map[string]any) int {
	delivered := h.publishLocalToUser(userID, payload)
	h.publishRemote(userID, payload)
	return delivered
}

func (h *wsHub) publishLocalToUser(userID string, payload map[string]any) int {
	sessions := h.snapshotUserSessions(userID)
	if len(sessions) == 0 {
		return 0
	}

	delivered := 0
	for _, session := range sessions {
		if err := session.conn.WriteJSON(payload); err != nil {
			_ = session.conn.Close()
			h.unregister(session)
			continue
		}
		delivered++
	}
	return delivered
}

func (h *wsHub) publishRemote(userID string, payload map[string]any) {
	if h.redis == nil {
		return
	}
	envelope := wsRedisEnvelope{Source: h.nodeID, UserID: userID, Payload: payload}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	if err := h.redis.Publish(context.Background(), h.redisChan, raw).Err(); err != nil {
		log.Printf("ws redis publish failed: %v", err)
	}
}

func (h *wsHub) snapshotUserSessions(userID string) []*wsSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sessionsMap := h.byUser[userID]
	if len(sessionsMap) == 0 {
		return nil
	}

	sessions := make([]*wsSession, 0, len(sessionsMap))
	for session := range sessionsMap {
		sessions = append(sessions, session)
	}
	return sessions
}

func (h *wsHub) startSubscriber() {
	if h.redis == nil {
		return
	}
	go func() {
		for {
			ctx := context.Background()
			pubsub := h.redis.Subscribe(ctx, h.redisChan)
			if _, err := pubsub.ReceiveTimeout(ctx, 3*time.Second); err != nil {
				_ = pubsub.Close()
				time.Sleep(1 * time.Second)
				continue
			}

			ch := pubsub.Channel()
			for message := range ch {
				var envelope wsRedisEnvelope
				if err := json.Unmarshal([]byte(message.Payload), &envelope); err != nil {
					continue
				}
				if envelope.Source == h.nodeID || strings.TrimSpace(envelope.UserID) == "" {
					continue
				}
				h.publishLocalToUser(envelope.UserID, envelope.Payload)
			}
			_ = pubsub.Close()
			time.Sleep(500 * time.Millisecond)
		}
	}()
}
