package neo4jadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

const defaultTopicLimit = 5

type Topic struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type Store interface {
	Ping(ctx context.Context) error
	ExpandTopics(ctx context.Context, topics []string, limit int) ([]Topic, error)
	UpsertMemoryTopics(ctx context.Context, memoryID string, topics []string) error
}

type Server struct {
	store    Store
	username string
	password string
	mux      *http.ServeMux
}

type expandTopicsRequest struct {
	Topics []string `json:"topics"`
	Limit  int      `json:"limit"`
}

type upsertTopicsRequest struct {
	MemoryID string   `json:"memory_id"`
	Topics   []string `json:"topics"`
}

func NewServer(store Store, username, password string) *Server {
	s := &Server{
		store:    store,
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/query/topics", s.handleExpandTopics)
	s.mux.HandleFunc("/query/upsert-memory-topics", s.handleUpsertMemoryTopics)
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.username == "" || s.password == "" {
		return true
	}
	username, password, ok := r.BasicAuth()
	if ok && username == s.username && password == s.password {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="nukara-neo4j-adapter"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "error": "store is not configured"})
		return
	}
	if err := s.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleExpandTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAuth(w, r) {
		return
	}
	if s.store == nil {
		http.Error(w, "Store is not configured", http.StatusServiceUnavailable)
		return
	}

	var req expandTopicsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 {
		req.Limit = defaultTopicLimit
	}
	requestTopics := normalizeTopics(req.Topics)
	if len(requestTopics) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"topics": []Topic{}})
		return
	}

	topics, err := s.store.ExpandTopics(r.Context(), requestTopics, req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": topics})
}

func (s *Server) handleUpsertMemoryTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAuth(w, r) {
		return
	}
	if s.store == nil {
		http.Error(w, "Store is not configured", http.StatusServiceUnavailable)
		return
	}

	var req upsertTopicsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.MemoryID = strings.TrimSpace(req.MemoryID)
	if req.MemoryID == "" {
		http.Error(w, "memory_id is required", http.StatusBadRequest)
		return
	}
	if err := s.store.UpsertMemoryTopics(r.Context(), req.MemoryID, normalizeTopics(req.Topics)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeTopics(topics []string) []string {
	seen := make(map[string]struct{}, len(topics))
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		key := strings.ToLower(topic)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, topic)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
