package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type selfCognitionSummaryConfigPayload struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

func normalizeSelfCognitionSummaryConfigPayload(payload selfCognitionSummaryConfigPayload) selfCognitionSummaryConfigPayload {
	payload.ProviderID = strings.TrimSpace(payload.ProviderID)
	payload.Model = strings.TrimSpace(payload.Model)
	return payload
}

func (s *Server) handleSelfCognitionSummaryConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		payload, err := s.loadSelfCognitionSummaryConfig()
		if err != nil {
			http.Error(w, "Failed to load self-cognition summary config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	case http.MethodPut:
		var req selfCognitionSummaryConfigPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req = normalizeSelfCognitionSummaryConfigPayload(req)
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
			http.Error(w, "Failed to update self-cognition summary config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if err := writeSystemSettingValue(tx, "self_cognition_summary_provider_id", req.ProviderID); err != nil {
			http.Error(w, "Failed to save self-cognition summary provider: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := writeSystemSettingValue(tx, "self_cognition_summary_model", req.Model); err != nil {
			http.Error(w, "Failed to save self-cognition summary model: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit self-cognition summary config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) loadSelfCognitionSummaryConfig() (selfCognitionSummaryConfigPayload, error) {
	providerID, err := s.readSystemSettingValue("self_cognition_summary_provider_id")
	if err != nil {
		if err == sql.ErrNoRows {
			return selfCognitionSummaryConfigPayload{}, nil
		}
		return selfCognitionSummaryConfigPayload{}, err
	}
	model, err := s.readSystemSettingValue("self_cognition_summary_model")
	if err != nil && err != sql.ErrNoRows {
		return selfCognitionSummaryConfigPayload{}, err
	}
	return selfCognitionSummaryConfigPayload{
		ProviderID: strings.TrimSpace(providerID),
		Model:      strings.TrimSpace(model),
	}, nil
}
