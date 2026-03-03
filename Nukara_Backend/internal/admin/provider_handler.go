package admin

import (
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
	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfg, err := s.loadNanobotConfig()
	if err != nil {
		http.Error(w, "Failed to load nanobot config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.loadProviderState()
	if err != nil {
		http.Error(w, "Failed to load provider state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(providersFromConfig(cfg, state))
}

func (s *Server) createProvider(w http.ResponseWriter, r *http.Request) {
	var p store.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	id := sanitizeProviderID(p.Name)
	if id == "" {
		http.Error(w, "Provider name is invalid", http.StatusBadRequest)
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfg, err := s.loadNanobotConfig()
	if err != nil {
		http.Error(w, "Failed to load nanobot config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.loadProviderState()
	if err != nil {
		http.Error(w, "Failed to load provider state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, exists := cfg.Providers[id]; exists {
		http.Error(w, "Provider already exists", http.StatusConflict)
		return
	}

	cfg.Providers[id] = nanobotProvider{
		APIKey:  p.APIKey,
		APIBase: p.BaseURL,
	}

	model := firstModel(p.Models)
	if p.IsActive || len(cfg.Providers) == 1 {
		if err := applyProviderToConfig(&cfg, id, model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.ActiveProviderID = id
	} else if strings.TrimSpace(cfg.Agents.Defaults.Model) == "" && model != "" {
		cfg.Agents.Defaults.Model = model
	}
	if strings.TrimSpace(model) != "" {
		state.ProviderModels[id] = model
	}

	if err := s.saveProviderConfigAndState(cfg, state); err != nil {
		http.Error(w, "Failed to persist provider data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	created, err := providerFromConfig(id, cfg, state, detectActiveProviderID(cfg, state), providerPriority(cfg, id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) getProvider(w http.ResponseWriter, r *http.Request, id string) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfg, err := s.loadNanobotConfig()
	if err != nil {
		http.Error(w, "Failed to load nanobot config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.loadProviderState()
	if err != nil {
		http.Error(w, "Failed to load provider state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	provider, err := providerFromConfig(id, cfg, state, detectActiveProviderID(cfg, state), providerPriority(cfg, id))
	if err != nil {
		if errors.Is(err, errProviderNotFound) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(provider)
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request, id string) {
	var p store.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfg, err := s.loadNanobotConfig()
	if err != nil {
		http.Error(w, "Failed to load nanobot config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.loadProviderState()
	if err != nil {
		http.Error(w, "Failed to load provider state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	existing, exists := cfg.Providers[id]
	if !exists {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	newID := id
	if strings.TrimSpace(p.Name) != "" {
		newID = sanitizeProviderID(p.Name)
	}
	if newID == "" {
		http.Error(w, "Provider name is invalid", http.StatusBadRequest)
		return
	}
	if newID != id {
		if _, exists := cfg.Providers[newID]; exists {
			http.Error(w, "Provider already exists", http.StatusConflict)
			return
		}
	}

	entry := existing
	entry.APIKey = p.APIKey
	entry.APIBase = p.BaseURL

	activeID := detectActiveProviderID(cfg, state)

	if newID != id {
		delete(cfg.Providers, id)
		if model, ok := state.ProviderModels[id]; ok {
			state.ProviderModels[newID] = model
		}
		delete(state.ProviderModels, id)
	}
	cfg.Providers[newID] = entry

	model := firstModel(p.Models)
	if strings.TrimSpace(model) != "" {
		state.ProviderModels[newID] = model
	}
	if p.IsActive || activeID == id || activeID == newID {
		if err := applyProviderToConfig(&cfg, newID, model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.ActiveProviderID = newID
	} else if strings.TrimSpace(model) != "" && detectActiveProviderID(cfg, state) == newID {
		cfg.Agents.Defaults.Model = model
	}

	if err := s.saveProviderConfigAndState(cfg, state); err != nil {
		http.Error(w, "Failed to persist provider data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	updated, err := providerFromConfig(newID, cfg, state, detectActiveProviderID(cfg, state), providerPriority(cfg, newID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	if id == "custom" {
		http.Error(w, "Provider custom cannot be deleted", http.StatusBadRequest)
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfg, err := s.loadNanobotConfig()
	if err != nil {
		http.Error(w, "Failed to load nanobot config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.loadProviderState()
	if err != nil {
		http.Error(w, "Failed to load provider state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, exists := cfg.Providers[id]; !exists {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}
	delete(cfg.Providers, id)
	delete(state.ProviderModels, id)
	if state.ActiveProviderID == id {
		state.ActiveProviderID = "custom"
	}

	if err := s.saveProviderConfigAndState(cfg, state); err != nil {
		http.Error(w, "Failed to persist provider data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"Provider deleted"}`))
}

func (s *Server) switchProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Model  string   `json:"model"`
		Models []string `json:"models"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = firstModel(req.Models)
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfg, err := s.loadNanobotConfig()
	if err != nil {
		http.Error(w, "Failed to load nanobot config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.loadProviderState()
	if err != nil {
		http.Error(w, "Failed to load provider state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, exists := cfg.Providers[id]; !exists {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}
	if model == "" {
		model = firstModel([]string{state.ProviderModels[id]})
	}

	if err := applyProviderToConfig(&cfg, id, model); err != nil {
		if errors.Is(err, errProviderNotFound) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(model) != "" {
		state.ProviderModels[id] = strings.TrimSpace(model)
	}
	state.ActiveProviderID = id
	if err := s.saveProviderConfigAndState(cfg, state); err != nil {
		http.Error(w, "Failed to persist provider data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	responseModel := strings.TrimSpace(cfg.Agents.Defaults.Model)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":            "Provider switched",
		"active_provider_id": id,
		"model":              responseModel,
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
		if errors.Is(err, errProviderNotFound) {
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

func providerPriority(cfg nanobotConfig, id string) int {
	ids := sortedProviderIDs(cfg.Providers)
	for i, candidate := range ids {
		if candidate == id {
			return i + 1
		}
	}
	return 0
}

func (s *Server) saveProviderConfigAndState(cfg nanobotConfig, state adminProviderState) error {
	if err := s.saveNanobotConfig(cfg); err != nil {
		return err
	}
	if err := s.saveProviderState(state); err != nil {
		return err
	}
	return nil
}
