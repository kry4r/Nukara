package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listProviders(w, r)
	case http.MethodPost:
		s.createProvider(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProviderByID(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/providers/"), "/")
	if path == "" {
		http.Error(w, "Provider id is required", http.StatusBadRequest)
		return
	}
	parts := strings.Split(path, "/")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		http.Error(w, "Provider id is required", http.StatusBadRequest)
		return
	}

	if len(parts) > 1 {
		switch parts[1] {
		case "test":
			s.testProvider(w, r, id)
			return
		case "switch":
			s.switchProvider(w, r, id)
			return
		case "chat-test":
			s.chatTestProvider(w, r, id)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		s.getProvider(w, r, id)
	case http.MethodPut:
		s.updateProvider(w, r, id)
	case http.MethodDelete:
		s.deleteProvider(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	providers, err := s.listProvidersFromDB()
	if err != nil {
		http.Error(w, "Failed to list providers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(providers)
}

func (s *Server) createProvider(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	var req store.Provider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "Provider name is required", http.StatusBadRequest)
		return
	}
	id := sanitizeProviderID(name)
	if id == "" || id == "custom" {
		http.Error(w, "Provider name is invalid", http.StatusBadRequest)
		return
	}

	models := normalizeModels(req.Models)
	priority := req.Priority
	if priority <= 0 {
		nextPriority, err := s.nextProviderPriority()
		if err != nil {
			http.Error(w, "Failed to allocate provider priority: "+err.Error(), http.StatusInternalServerError)
			return
		}
		priority = nextPriority
	}

	createdAt := time.Time{}
	updatedAt := time.Time{}
	modelsRaw, err := json.Marshal(models)
	if err != nil {
		http.Error(w, "Invalid provider models", http.StatusBadRequest)
		return
	}

	err = s.db.QueryRow(`
		INSERT INTO providers(id, name, api_key, base_url, models, is_active, priority, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at
	`, id, name, strings.TrimSpace(req.APIKey), strings.TrimSpace(req.BaseURL), string(modelsRaw), req.IsActive, priority).
		Scan(&createdAt, &updatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			http.Error(w, "Provider already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	model := firstModel(models)
	if req.IsActive {
		if err := s.syncActiveProviderToDB(id, model); err != nil {
			http.Error(w, "Failed to sync active provider: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err := s.ensureDefaultProviderReady(); err != nil {
		http.Error(w, "Failed to initialize default provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	created, err := s.getProviderByID(id)
	if err != nil {
		http.Error(w, "Failed to load created provider: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if createdAt.IsZero() {
		createdAt = created.CreatedAt
	}
	if updatedAt.IsZero() {
		updatedAt = created.UpdatedAt
	}
	created.CreatedAt = createdAt
	created.UpdatedAt = updatedAt

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) getProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	provider, err := s.getProviderByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(provider)
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	current, err := s.getProviderByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req store.Provider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = current.Name
	}
	if name == "" {
		name = current.ID
	}
	models := current.Models
	if req.Models != nil {
		models = normalizeModels(req.Models)
	}
	priority := current.Priority
	if req.Priority > 0 {
		priority = req.Priority
	}
	isActive := current.IsActive || req.IsActive
	modelsRaw, err := json.Marshal(models)
	if err != nil {
		http.Error(w, "Invalid provider models", http.StatusBadRequest)
		return
	}

	_, err = s.db.Exec(`
		UPDATE providers
		SET name = $1,
		    api_key = $2,
		    base_url = $3,
		    models = $4::jsonb,
		    is_active = $5,
		    priority = $6,
		    updated_at = NOW()
		WHERE id = $7
	`, name, strings.TrimSpace(req.APIKey), strings.TrimSpace(req.BaseURL), string(modelsRaw), isActive, priority, id)
	if err != nil {
		http.Error(w, "Failed to update provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if req.IsActive {
		if err := s.syncActiveProviderToDB(id, firstModel(models)); err != nil {
			http.Error(w, "Failed to sync active provider: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	updated, err := s.getProviderByID(id)
	if err != nil {
		http.Error(w, "Failed to load updated provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	if strings.EqualFold(strings.TrimSpace(id), "custom") {
		http.Error(w, "Provider custom cannot be deleted", http.StatusBadRequest)
		return
	}

	var existed bool
	var wasActive bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM providers WHERE id = $1), COALESCE((SELECT is_active FROM providers WHERE id = $1), FALSE)`, strings.TrimSpace(id)).
		Scan(&existed, &wasActive)
	if err != nil {
		http.Error(w, "Failed to query provider: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !existed {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	if _, err := s.db.Exec(`DELETE FROM providers WHERE id = $1`, strings.TrimSpace(id)); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			http.Error(w, "Provider is in use by users/bots; clear assignments before delete", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to delete provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defaultID, err := s.readSystemSettingValue("default_chat_provider_id")
	if err != nil {
		http.Error(w, "Failed to check default provider: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if wasActive || strings.EqualFold(strings.TrimSpace(defaultID), strings.TrimSpace(id)) {
		if err := s.ensureDefaultProviderReady(); err != nil {
			http.Error(w, "Failed to re-elect default provider: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "Provider deleted",
		"id":      id,
	})
}

func (s *Server) switchProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Model  string   `json:"model"`
		Models []string `json:"models"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	provider, err := s.getProviderByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = firstModel(req.Models)
	}
	if model == "" {
		model = firstModel(provider.Models)
	}

	if err := s.syncActiveProviderToDB(id, model); err != nil {
		http.Error(w, "Failed to switch provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":            "Provider switched",
		"active_provider_id": id,
		"model":              model,
	})
}

func (s *Server) testProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "ping"
	}
	start := time.Now()
	reply, err := s.runProviderChatTest(id, req.Message, "")
	latency := time.Since(start).Milliseconds()

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        "error",
			"latency_ms":    latency,
			"error_message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"latency_ms": latency,
		"reply":      reply,
	})
}

func (s *Server) nextProviderPriority() (int, error) {
	var next int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(priority), 0) + 1 FROM providers`).Scan(&next)
	if err != nil {
		return 0, err
	}
	if next <= 0 {
		next = 1
	}
	return next, nil
}

func (s *Server) listProvidersFromDB() ([]store.Provider, error) {
	rows, err := s.db.Query(`
		SELECT id, name, api_key, base_url, models, is_active, priority, created_at, updated_at
		FROM providers
		WHERE id <> 'custom'
		ORDER BY priority ASC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]store.Provider, 0, 8)
	for rows.Next() {
		var p store.Provider
		var modelsRaw []byte
		if scanErr := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &modelsRaw, &p.IsActive, &p.Priority, &p.CreatedAt, &p.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		p.Models = unmarshalModels(modelsRaw)
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" {
			p.Name = p.ID
		}
		providers = append(providers, p)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

func (s *Server) getProviderByID(id string) (store.Provider, error) {
	var p store.Provider
	var modelsRaw []byte
	err := s.db.QueryRow(`
		SELECT id, name, api_key, base_url, models, is_active, priority, created_at, updated_at
		FROM providers
		WHERE id = $1 AND id <> 'custom'
	`, strings.TrimSpace(id)).
		Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &modelsRaw, &p.IsActive, &p.Priority, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return store.Provider{}, err
	}
	p.Models = unmarshalModels(modelsRaw)
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = p.ID
	}
	return p, nil
}

func (s *Server) ensureDefaultProviderReady() error {
	defaultProviderID, err := s.readSystemSettingValue("default_chat_provider_id")
	if err != nil {
		return err
	}
	if defaultProviderID != "" {
		var exists bool
		if err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM providers WHERE id = $1)`, strings.TrimSpace(defaultProviderID)).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
	}

	providers, err := s.listProvidersFromDB()
	if err != nil {
		return err
	}
	if len(providers) == 0 {
		return nil
	}
	chosen := providers[0]
	return s.syncActiveProviderToDB(chosen.ID, firstModel(chosen.Models))
}

func (s *Server) syncActiveProviderToDB(activeProviderID, model string) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`UPDATE providers SET is_active = FALSE`); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE providers SET is_active = TRUE, updated_at = NOW() WHERE id = $1`, strings.TrimSpace(activeProviderID)); err != nil {
		return err
	}
	if _, err = tx.Exec(`
		INSERT INTO system_settings(key, value, updated_at)
		VALUES ('default_chat_provider_id', jsonb_build_object('value', $1), NOW())
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, strings.TrimSpace(activeProviderID)); err != nil {
		return err
	}
	if strings.TrimSpace(model) != "" {
		if _, err = tx.Exec(`
			INSERT INTO system_settings(key, value, updated_at)
			VALUES ('default_chat_model', jsonb_build_object('value', $1), NOW())
			ON CONFLICT (key)
			DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
		`, strings.TrimSpace(model)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
