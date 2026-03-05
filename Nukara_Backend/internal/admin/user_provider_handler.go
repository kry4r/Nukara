package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type adminProviderOption struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Models   []string `json:"models"`
	IsActive bool     `json:"is_active"`
	Priority int      `json:"priority"`
}

type userProviderSettingItem struct {
	UserID              string     `json:"user_id"`
	Phone               string     `json:"phone"`
	Nickname            string     `json:"nickname"`
	ProviderID          string     `json:"provider_id"`
	Model               string     `json:"model"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
	EffectiveProviderID string     `json:"effective_provider_id"`
	EffectiveModel      string     `json:"effective_model"`
	IsOverride          bool       `json:"is_override"`
}

func (s *Server) handleUserProviderSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 50, 1, 200)
	offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 0, 100000)

	providers, err := s.listAdminProviderOptions()
	if err != nil {
		http.Error(w, "Failed to list providers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defaultProviderID, defaultModel, err := s.loadDefaultRoutingSettings()
	if err != nil {
		http.Error(w, "Failed to load default routing settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if defaultProviderID == "" && len(providers) > 0 {
		defaultProviderID = providers[0].ID
		for _, provider := range providers {
			if provider.IsActive {
				defaultProviderID = provider.ID
				break
			}
		}
	}
	if defaultModel == "" {
		for _, provider := range providers {
			if provider.ID != defaultProviderID {
				continue
			}
			if len(provider.Models) > 0 {
				defaultModel = strings.TrimSpace(provider.Models[0])
			}
			break
		}
	}

	items, total, err := s.listUserProviderSettingItems(query, limit, offset, defaultProviderID, defaultModel)
	if err != nil {
		http.Error(w, "Failed to list user provider settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":               items,
		"providers":           providers,
		"default_provider_id": defaultProviderID,
		"default_model":       defaultModel,
		"total":               total,
		"limit":               limit,
		"offset":              offset,
	})
}

func (s *Server) handleUserProviderSettingByUser(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/provider-settings/"), "/")
	if path == "" {
		http.Error(w, "User id is required", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(path)
	if userID == "" {
		http.Error(w, "User id is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.putUserProviderSetting(w, r, userID)
	case http.MethodDelete:
		s.deleteUserProviderSetting(w, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) putUserProviderSetting(w http.ResponseWriter, r *http.Request, userID string) {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.ProviderID = strings.TrimSpace(req.ProviderID)
	req.Model = strings.TrimSpace(req.Model)

	if req.ProviderID == "" {
		http.Error(w, "provider_id is required", http.StatusBadRequest)
		return
	}

	ok, err := s.userExists(userID)
	if err != nil {
		http.Error(w, "Failed to verify user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	ok, err = s.providerExists(req.ProviderID)
	if err != nil {
		http.Error(w, "Failed to verify provider: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Provider not found", http.StatusBadRequest)
		return
	}

	_, err = s.db.Exec(`
		INSERT INTO user_provider_settings (user_id, provider_id, model, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET provider_id = EXCLUDED.provider_id, model = EXCLUDED.model, updated_at = NOW()
	`, userID, req.ProviderID, req.Model)
	if err != nil {
		http.Error(w, "Failed to save user provider setting: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defaultProviderID, defaultModel, err := s.loadDefaultRoutingSettings()
	if err != nil {
		http.Error(w, "Failed to load default routing settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	item, err := s.getUserProviderSettingItem(userID, defaultProviderID, defaultModel)
	if err != nil {
		http.Error(w, "Failed to load updated user provider setting: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (s *Server) deleteUserProviderSetting(w http.ResponseWriter, userID string) {
	ok, err := s.userExists(userID)
	if err != nil {
		http.Error(w, "Failed to verify user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if _, err := s.db.Exec(`DELETE FROM user_provider_settings WHERE user_id = $1`, userID); err != nil {
		http.Error(w, "Failed to clear user provider setting: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "User provider setting cleared",
		"user_id": userID,
	})
}

func (s *Server) listUserProviderSettingItems(query string, limit, offset int, defaultProviderID, defaultModel string) ([]userProviderSettingItem, int, error) {
	rows, err := s.db.Query(`
		SELECT
			u.id::text,
			u.phone,
			u.nickname,
			COALESCE(ups.provider_id, ''),
			COALESCE(ups.model, ''),
			ups.updated_at
		FROM users u
		LEFT JOIN user_provider_settings ups ON ups.user_id = u.id
		WHERE (
			$1 = ''
			OR u.phone ILIKE '%' || $1 || '%'
			OR u.nickname ILIKE '%' || $1 || '%'
			OR u.id::text ILIKE '%' || $1 || '%'
		)
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]userProviderSettingItem, 0, limit)
	for rows.Next() {
		var item userProviderSettingItem
		var updatedAt sql.NullTime
		if scanErr := rows.Scan(
			&item.UserID,
			&item.Phone,
			&item.Nickname,
			&item.ProviderID,
			&item.Model,
			&updatedAt,
		); scanErr != nil {
			return nil, 0, scanErr
		}
		if updatedAt.Valid {
			t := updatedAt.Time.UTC()
			item.UpdatedAt = &t
		}
		item.ProviderID = strings.TrimSpace(item.ProviderID)
		item.Model = strings.TrimSpace(item.Model)
		item.IsOverride = item.ProviderID != ""
		item.EffectiveProviderID = item.ProviderID
		if item.EffectiveProviderID == "" {
			item.EffectiveProviderID = defaultProviderID
		}
		item.EffectiveModel = item.Model
		if item.EffectiveModel == "" {
			item.EffectiveModel = defaultModel
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM users u
		WHERE (
			$1 = ''
			OR u.phone ILIKE '%' || $1 || '%'
			OR u.nickname ILIKE '%' || $1 || '%'
			OR u.id::text ILIKE '%' || $1 || '%'
		)
	`, query).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Server) getUserProviderSettingItem(userID, defaultProviderID, defaultModel string) (userProviderSettingItem, error) {
	var item userProviderSettingItem
	var updatedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT
			u.id::text,
			u.phone,
			u.nickname,
			COALESCE(ups.provider_id, ''),
			COALESCE(ups.model, ''),
			ups.updated_at
		FROM users u
		LEFT JOIN user_provider_settings ups ON ups.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&item.UserID,
		&item.Phone,
		&item.Nickname,
		&item.ProviderID,
		&item.Model,
		&updatedAt,
	)
	if err != nil {
		return userProviderSettingItem{}, err
	}

	item.ProviderID = strings.TrimSpace(item.ProviderID)
	item.Model = strings.TrimSpace(item.Model)
	item.IsOverride = item.ProviderID != ""
	item.EffectiveProviderID = item.ProviderID
	if item.EffectiveProviderID == "" {
		item.EffectiveProviderID = defaultProviderID
	}
	item.EffectiveModel = item.Model
	if item.EffectiveModel == "" {
		item.EffectiveModel = defaultModel
	}
	if updatedAt.Valid {
		t := updatedAt.Time.UTC()
		item.UpdatedAt = &t
	}
	return item, nil
}

func (s *Server) listAdminProviderOptions() ([]adminProviderOption, error) {
	rows, err := s.db.Query(`
		SELECT id, name, models, is_active, priority
		FROM providers
		WHERE id <> 'custom'
		ORDER BY priority ASC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]adminProviderOption, 0, 8)
	for rows.Next() {
		var p adminProviderOption
		var modelsRaw []byte
		if scanErr := rows.Scan(&p.ID, &p.Name, &modelsRaw, &p.IsActive, &p.Priority); scanErr != nil {
			return nil, scanErr
		}
		_ = json.Unmarshal(modelsRaw, &p.Models)
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

func (s *Server) userExists(userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	return exists, err
}

func (s *Server) providerExists(providerID string) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(providerID), "custom") {
		return false, nil
	}
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM providers WHERE id = $1)`, providerID).Scan(&exists)
	return exists, err
}

func (s *Server) loadDefaultRoutingSettings() (string, string, error) {
	defaultProviderID, err := s.readSystemSettingValue("default_chat_provider_id")
	if err != nil {
		return "", "", err
	}
	defaultModel, err := s.readSystemSettingValue("default_chat_model")
	if err != nil {
		return "", "", err
	}
	return defaultProviderID, defaultModel, nil
}

func (s *Server) readSystemSettingValue(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`
		SELECT COALESCE(value->>'value', '')
		FROM system_settings
		WHERE key = $1
	`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
