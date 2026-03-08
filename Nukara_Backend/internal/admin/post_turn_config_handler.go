package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type postTurnConfigPayload struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

func normalizePostTurnConfigPayload(payload postTurnConfigPayload) postTurnConfigPayload {
	payload.ProviderID = strings.TrimSpace(payload.ProviderID)
	payload.Model = strings.TrimSpace(payload.Model)
	return payload
}

func (s *Server) handlePostTurnConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		payload, err := s.loadPostTurnConfig()
		if err != nil {
			http.Error(w, "Failed to load post-turn config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	case http.MethodPut:
		var req postTurnConfigPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req = normalizePostTurnConfigPayload(req)
		if req.ProviderID != "" {
			exists, err := s.providerExists(req.ProviderID)
			if err != nil {
				http.Error(w, "Failed to validate provider: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if !exists {
				http.Error(w, "Provider not found", http.StatusBadRequest)
				return
			}
		}
		tx, err := s.db.Begin()
		if err != nil {
			http.Error(w, "Failed to update post-turn config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if err := writeSystemSettingValue(tx, "post_turn_provider_id", req.ProviderID); err != nil {
			http.Error(w, "Failed to save post-turn provider: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := writeSystemSettingValue(tx, "post_turn_model", req.Model); err != nil {
			http.Error(w, "Failed to save post-turn model: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit post-turn config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) loadPostTurnConfig() (postTurnConfigPayload, error) {
	providerID, err := s.readSystemSettingValue("post_turn_provider_id")
	if err != nil {
		if err == sql.ErrNoRows {
			return postTurnConfigPayload{}, nil
		}
		return postTurnConfigPayload{}, err
	}
	model, err := s.readSystemSettingValue("post_turn_model")
	if err != nil && err != sql.ErrNoRows {
		return postTurnConfigPayload{}, err
	}
	return postTurnConfigPayload{
		ProviderID: strings.TrimSpace(providerID),
		Model:      strings.TrimSpace(model),
	}, nil
}
