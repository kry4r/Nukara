package admin

import (
	"encoding/json"
	"net/http"
	stdmail "net/mail"
	"strconv"
	"strings"

	internalmail "nukara/backend/internal/mail"
)

type emailAuthSettingsPayload struct {
	SMTPHost               string `json:"smtp_host"`
	SMTPPort               string `json:"smtp_port"`
	SMTPUsername           string `json:"smtp_username"`
	SMTPPassword           string `json:"smtp_password,omitempty"`
	SMTPPasswordConfigured bool   `json:"smtp_password_configured"`
	FromEmail              string `json:"from_email"`
	FromName               string `json:"from_name"`
	CodeTTLSeconds         int    `json:"code_ttl_seconds"`
}

type emailAuthTestPayload struct {
	ToEmail string `json:"to_email"`
}

func normalizeEmailAuthSettingsPayload(payload emailAuthSettingsPayload) emailAuthSettingsPayload {
	payload.SMTPHost = strings.TrimSpace(payload.SMTPHost)
	payload.SMTPPort = strings.TrimSpace(payload.SMTPPort)
	payload.SMTPUsername = strings.TrimSpace(payload.SMTPUsername)
	payload.SMTPPassword = strings.TrimSpace(payload.SMTPPassword)
	payload.FromEmail = strings.TrimSpace(payload.FromEmail)
	payload.FromName = strings.TrimSpace(payload.FromName)
	if payload.SMTPPort == "" {
		payload.SMTPPort = "465"
	}
	if payload.FromName == "" {
		payload.FromName = "Nukara"
	}
	if payload.CodeTTLSeconds <= 0 {
		payload.CodeTTLSeconds = 900
	}
	return payload
}

func maskEmailAuthSettingsPayload(payload emailAuthSettingsPayload) emailAuthSettingsPayload {
	payload.SMTPPasswordConfigured = strings.TrimSpace(payload.SMTPPassword) != ""
	payload.SMTPPassword = ""
	return payload
}

type adminSettingsReader struct {
	server *Server
}

func (r adminSettingsReader) GetSystemSetting(key string) (string, bool) {
	value, err := r.server.readSystemSettingValue(key)
	if err != nil {
		return "", false
	}
	return value, strings.TrimSpace(value) != ""
}

func (s *Server) handleEmailAuthSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getEmailAuthSettings(w, r)
	case http.MethodPut:
		s.updateEmailAuthSettings(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleEmailAuthSettingsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	var req emailAuthTestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	recipient := strings.TrimSpace(req.ToEmail)
	if recipient == "" {
		http.Error(w, "Test recipient email is required", http.StatusBadRequest)
		return
	}
	if _, err := stdmail.ParseAddress(recipient); err != nil {
		http.Error(w, "Invalid recipient email", http.StatusBadRequest)
		return
	}
	sender := internalmail.NewSMTPSender(adminSettingsReader{server: s})
	if err := sender.SendTestMail(r.Context(), recipient); err != nil {
		status := http.StatusBadRequest
		if !strings.Contains(strings.ToLower(err.Error()), "smtp not configured") && !strings.Contains(strings.ToLower(err.Error()), "invalid smtp") {
			status = http.StatusBadGateway
		}
		http.Error(w, "Failed to send test email: "+err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":  "Test email sent",
		"to_email": recipient,
	})
}

func (s *Server) getEmailAuthSettings(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	payload, err := s.loadEmailAuthSettings()
	if err != nil {
		http.Error(w, "Failed to load email auth settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) updateEmailAuthSettings(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	var req emailAuthSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	existingPassword, err := s.readSystemSettingValue("smtp_password")
	if err != nil {
		http.Error(w, "Failed to read existing email auth settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	payload := normalizeEmailAuthSettingsPayload(req)
	passwordToStore := payload.SMTPPassword
	if passwordToStore == "" {
		passwordToStore = strings.TrimSpace(existingPassword)
	}
	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Failed to update email auth settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	for key, value := range map[string]string{
		"smtp_host":              payload.SMTPHost,
		"smtp_port":              payload.SMTPPort,
		"smtp_username":          payload.SMTPUsername,
		"smtp_password":          passwordToStore,
		"smtp_from_email":        payload.FromEmail,
		"smtp_from_name":         payload.FromName,
		"email_code_ttl_seconds": strconv.Itoa(payload.CodeTTLSeconds),
	} {
		if err := writeSystemSettingValue(tx, key, value); err != nil {
			http.Error(w, "Failed to save email auth settings: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit email auth settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	payload.SMTPPassword = passwordToStore
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(maskEmailAuthSettingsPayload(payload))
}

func (s *Server) loadEmailAuthSettings() (emailAuthSettingsPayload, error) {
	host, err := s.readSystemSettingValue("smtp_host")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	port, err := s.readSystemSettingValue("smtp_port")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	username, err := s.readSystemSettingValue("smtp_username")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	password, err := s.readSystemSettingValue("smtp_password")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	fromEmail, err := s.readSystemSettingValue("smtp_from_email")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	fromName, err := s.readSystemSettingValue("smtp_from_name")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	ttlRaw, err := s.readSystemSettingValue("email_code_ttl_seconds")
	if err != nil {
		return emailAuthSettingsPayload{}, err
	}
	payload := normalizeEmailAuthSettingsPayload(emailAuthSettingsPayload{
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPUsername: username,
		SMTPPassword: password,
		FromEmail:    fromEmail,
		FromName:     fromName,
	})
	if ttlRaw != "" {
		if ttl, err := strconv.Atoi(strings.TrimSpace(ttlRaw)); err == nil && ttl > 0 {
			payload.CodeTTLSeconds = ttl
		}
	}
	return maskEmailAuthSettingsPayload(payload), nil
}
