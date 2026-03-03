package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) chatTestProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
		Model   string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "你好，请回复“provider 已连接”。"
	}

	start := time.Now()
	reply, err := s.runProviderChatTest(id, req.Message, req.Model)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		if errors.Is(err, errProviderNotFound) {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        "error",
			"latency_ms":    latency,
			"error_message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"provider_id": id,
		"latency_ms":  latency,
		"reply":       reply,
	})
}

func (s *Server) runProviderChatTest(id, message, model string) (string, error) {
	s.configMu.Lock()
	cfg, err := s.loadNanobotConfig()
	if err != nil {
		s.configMu.Unlock()
		return "", err
	}
	state, err := s.loadProviderState()
	if err != nil {
		s.configMu.Unlock()
		return "", err
	}
	if _, exists := cfg.Providers[id]; !exists {
		s.configMu.Unlock()
		return "", fmt.Errorf("%w: %s", errProviderNotFound, id)
	}
	if strings.TrimSpace(model) == "" {
		model = strings.TrimSpace(state.ProviderModels[id])
	}
	if err := applyProviderToConfig(&cfg, id, model); err != nil {
		s.configMu.Unlock()
		return "", err
	}
	if strings.TrimSpace(model) != "" {
		state.ProviderModels[id] = strings.TrimSpace(model)
	}
	state.ActiveProviderID = id
	if err := s.saveProviderConfigAndState(cfg, state); err != nil {
		s.configMu.Unlock()
		return "", err
	}
	s.configMu.Unlock()

	return s.chatWithNanobot(message)
}

func (s *Server) chatWithNanobot(message string) (string, error) {
	payload := map[string]any{
		"conversation_id": fmt.Sprintf("admin-provider-test-%d", time.Now().UnixNano()),
		"robot_id":        "default",
		"content": map[string]any{
			"type": "text",
			"text": message,
		},
		"system_context": map[string]any{
			"bot_name": "ProviderTestBot",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.nanobotHTTPURL, "/")+"/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.nanobotToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.nanobotToken)
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nanobot returned %d: %s", resp.StatusCode, string(raw))
	}

	var payloadMap map[string]any
	if err := json.Unmarshal(raw, &payloadMap); err != nil {
		return "", fmt.Errorf("invalid nanobot response: %w", err)
	}

	reply := extractNanobotReply(payloadMap)
	if reply == "" {
		reply = string(raw)
	}
	return reply, nil
}

func extractNanobotReply(payload map[string]any) string {
	if text, ok := payload["text"].(string); ok && strings.TrimSpace(text) != "" {
		return text
	}
	if reply, ok := payload["reply"].(string); ok && strings.TrimSpace(reply) != "" {
		return reply
	}
	if message, ok := payload["message"].(string); ok && strings.TrimSpace(message) != "" {
		return message
	}
	content, ok := payload["content"].(map[string]any)
	if !ok {
		return ""
	}
	if text, ok := content["text"].(string); ok {
		return text
	}
	return ""
}
