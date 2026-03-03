package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"nukara/backend/internal/store"
)

type nanobotConfig struct {
	Agents struct {
		Defaults struct {
			Model string `json:"model"`
		} `json:"defaults"`
	} `json:"agents"`
	Providers map[string]nanobotProvider `json:"providers"`
	raw       map[string]any
}

type nanobotProvider struct {
	APIKey  string `json:"api_key"`
	APIBase string `json:"api_base"`
}

type adminProviderState struct {
	ActiveProviderID string            `json:"active_provider_id"`
	ProviderModels   map[string]string `json:"provider_models"`
}

var errProviderNotFound = errors.New("provider not found")

func defaultNanobotConfig() nanobotConfig {
	cfg := nanobotConfig{
		Providers: map[string]nanobotProvider{
			"custom": {},
		},
		raw: map[string]any{
			"agents": map[string]any{
				"defaults": map[string]any{},
			},
			"providers": map[string]any{
				"custom": map[string]any{},
			},
		},
	}
	return cfg
}

func defaultProviderState() adminProviderState {
	return adminProviderState{
		ProviderModels: map[string]string{},
	}
}

func (s *Server) loadNanobotConfig() (nanobotConfig, error) {
	raw, err := os.ReadFile(s.nanobotConfigPath)
	if err != nil {
		return nanobotConfig{}, err
	}

	cfg := defaultNanobotConfig()
	if len(strings.TrimSpace(string(raw))) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nanobotConfig{}, err
	}
	if err := json.Unmarshal(raw, &cfg.raw); err != nil {
		return nanobotConfig{}, err
	}
	if cfg.raw == nil {
		cfg.raw = map[string]any{}
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]nanobotProvider{}
	}
	if _, ok := cfg.Providers["custom"]; !ok {
		cfg.Providers["custom"] = nanobotProvider{}
	}
	return cfg, nil
}

func (s *Server) saveNanobotConfig(cfg nanobotConfig) error {
	if cfg.Providers == nil {
		cfg.Providers = map[string]nanobotProvider{}
	}
	if _, ok := cfg.Providers["custom"]; !ok {
		cfg.Providers["custom"] = nanobotProvider{}
	}
	root := cloneAnyMap(cfg.raw)
	if root == nil {
		root = map[string]any{}
	}
	providers := map[string]any{}
	for id, provider := range cfg.Providers {
		providers[id] = map[string]any{
			"api_key":  provider.APIKey,
			"api_base": provider.APIBase,
		}
	}
	root["providers"] = providers

	agents := ensureAnyMap(root["agents"])
	defaults := ensureAnyMap(agents["defaults"])
	model := strings.TrimSpace(cfg.Agents.Defaults.Model)
	if model != "" {
		defaults["model"] = model
	}
	agents["defaults"] = defaults
	root["agents"] = agents
	cfg.raw = root

	return saveJSONAtomically(s.nanobotConfigPath, "nanobot-config-", root)
}

func (s *Server) loadProviderState() (adminProviderState, error) {
	raw, err := os.ReadFile(s.nanobotStatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultProviderState(), nil
		}
		return adminProviderState{}, err
	}

	state := defaultProviderState()
	if len(strings.TrimSpace(string(raw))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return adminProviderState{}, err
	}
	if state.ProviderModels == nil {
		state.ProviderModels = map[string]string{}
	}
	return state, nil
}

func (s *Server) saveProviderState(state adminProviderState) error {
	if state.ProviderModels == nil {
		state.ProviderModels = map[string]string{}
	}
	return saveJSONAtomically(s.nanobotStatePath, "nanobot-provider-state-", state)
}

func saveJSONAtomically(path, tempPrefix string, value any) error {
	updated, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, tempPrefix+"*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(updated); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func sanitizeProviderID(raw string) string {
	input := strings.ToLower(strings.TrimSpace(raw))
	input = strings.ReplaceAll(input, " ", "-")
	var b strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		if r == '/' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-_.")
}

func firstModel(models []string) string {
	for _, model := range models {
		v := strings.TrimSpace(model)
		if v != "" {
			return v
		}
	}
	return ""
}

func sortedProviderIDs(providers map[string]nanobotProvider) []string {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	customPos := -1
	for i, id := range ids {
		if id == "custom" {
			customPos = i
			break
		}
	}
	if customPos > 0 {
		ids = append([]string{"custom"}, append(ids[:customPos], ids[customPos+1:]...)...)
	}
	return ids
}

func detectActiveProviderID(cfg nanobotConfig, state adminProviderState) string {
	if state.ActiveProviderID != "" {
		if _, ok := cfg.Providers[state.ActiveProviderID]; ok {
			return state.ActiveProviderID
		}
	}
	if len(cfg.Providers) == 0 {
		return "custom"
	}
	custom, ok := cfg.Providers["custom"]
	if !ok {
		ids := sortedProviderIDs(cfg.Providers)
		if len(ids) == 0 {
			return "custom"
		}
		return ids[0]
	}
	for id, provider := range cfg.Providers {
		if id == "custom" {
			continue
		}
		if provider.APIKey == custom.APIKey && provider.APIBase == custom.APIBase &&
			(provider.APIKey != "" || provider.APIBase != "") {
			return id
		}
	}
	return "custom"
}

func providerFromConfig(id string, cfg nanobotConfig, state adminProviderState, activeID string, priority int) (store.Provider, error) {
	provider, ok := cfg.Providers[id]
	if !ok {
		return store.Provider{}, fmt.Errorf("%w: %s", errProviderNotFound, id)
	}
	model := strings.TrimSpace(state.ProviderModels[id])
	if model == "" && id == activeID {
		model = strings.TrimSpace(cfg.Agents.Defaults.Model)
	}
	models := []string{}
	if model != "" {
		models = []string{model}
	}
	return store.Provider{
		ID:       id,
		Name:     id,
		APIKey:   provider.APIKey,
		BaseURL:  provider.APIBase,
		Models:   models,
		IsActive: id == activeID,
		Priority: priority,
	}, nil
}

func providersFromConfig(cfg nanobotConfig, state adminProviderState) []store.Provider {
	activeID := detectActiveProviderID(cfg, state)
	ids := sortedProviderIDs(cfg.Providers)
	result := make([]store.Provider, 0, len(ids))
	for i, id := range ids {
		provider, err := providerFromConfig(id, cfg, state, activeID, i+1)
		if err == nil {
			result = append(result, provider)
		}
	}
	return result
}

func applyProviderToConfig(cfg *nanobotConfig, id string, model string) error {
	provider, ok := cfg.Providers[id]
	if !ok {
		return fmt.Errorf("%w: %s", errProviderNotFound, id)
	}
	cfg.Providers["custom"] = provider
	if strings.TrimSpace(model) != "" {
		cfg.Agents.Defaults.Model = strings.TrimSpace(model)
	}
	return nil
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func ensureAnyMap(value any) map[string]any {
	if mapValue, ok := value.(map[string]any); ok {
		return mapValue
	}
	return map[string]any{}
}
