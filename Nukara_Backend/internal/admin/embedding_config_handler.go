package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type embeddingConfigPayload struct {
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	ProviderID string `json:"provider_id"`
}

func (s *Server) handleEmbeddingConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getEmbeddingConfig(w, r)
	case http.MethodPut:
		s.updateEmbeddingConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getEmbeddingConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	payload, err := s.loadEmbeddingConfig()
	if err != nil {
		http.Error(w, "Failed to load embedding config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) updateEmbeddingConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	var req embeddingConfigPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Model = strings.TrimSpace(req.Model)
	req.ProviderID = strings.TrimSpace(req.ProviderID)

	if req.ProviderID != "" {
		exists, err := s.providerExists(req.ProviderID)
		if err != nil {
			http.Error(w, "Failed to validate fallback provider: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "Fallback provider does not exist", http.StatusBadRequest)
			return
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Failed to update embedding config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if err := writeSystemSettingValue(tx, "embedding_base_url", req.BaseURL); err != nil {
		http.Error(w, "Failed to save embedding base URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := writeSystemSettingValue(tx, "embedding_api_key", req.APIKey); err != nil {
		http.Error(w, "Failed to save embedding API key: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := writeSystemSettingValue(tx, "embedding_model", req.Model); err != nil {
		http.Error(w, "Failed to save embedding model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := writeSystemSettingValue(tx, "embedding_provider_id", req.ProviderID); err != nil {
		http.Error(w, "Failed to save embedding fallback provider: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit embedding config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(req)
}

func (s *Server) loadEmbeddingConfig() (embeddingConfigPayload, error) {
	baseURL, err := s.readSystemSettingValue("embedding_base_url")
	if err != nil {
		return embeddingConfigPayload{}, err
	}
	apiKey, err := s.readSystemSettingValue("embedding_api_key")
	if err != nil {
		return embeddingConfigPayload{}, err
	}
	model, err := s.readSystemSettingValue("embedding_model")
	if err != nil {
		return embeddingConfigPayload{}, err
	}
	providerID, err := s.readSystemSettingValue("embedding_provider_id")
	if err != nil {
		return embeddingConfigPayload{}, err
	}
	return embeddingConfigPayload{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		ProviderID: providerID,
	}, nil
}

type systemSettingsWriter interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func writeSystemSettingValue(writer systemSettingsWriter, key, value string) error {
	_, err := writer.Exec(`
		INSERT INTO system_settings(key, value, updated_at)
		VALUES ($1, jsonb_build_object('value', $2::text), NOW())
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, strings.TrimSpace(key), strings.TrimSpace(value))
	return err
}
