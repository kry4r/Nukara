package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"nukara/backend/internal/store"
)

type Server struct {
	store store.DataStore
}

func NewServer(st store.DataStore) *Server {
	return &Server{
		store: st,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		})
		return
	}

	postgresConnected := false
	redisConnected := false

	// Check PostgreSQL and Redis connections
	if pgStore, ok := s.store.(*store.PostgresStore); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if db := pgStore.DB(); db != nil {
			if err := db.PingContext(ctx); err == nil {
				postgresConnected = true
			}
		}
		redisConnected = pgStore.HasRedis()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "ok",
		"postgres_connected": postgresConnected,
		"redis_connected":    redisConnected,
	})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
