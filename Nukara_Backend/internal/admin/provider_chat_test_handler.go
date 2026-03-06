package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"nukara/backend/internal/agentx/llm"
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
		"status":      "ok",
		"provider_id": id,
		"latency_ms":  latency,
		"reply":       reply,
	})
}

func (s *Server) runProviderChatTest(id, message, model string) (string, error) {
	if s.db == nil {
		return "", errors.New("database is not configured")
	}
	provider, err := s.getProviderByID(id)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(provider.BaseURL) == "" {
		return "", errors.New("provider base_url is empty")
	}

	selectedModel := strings.TrimSpace(model)
	if selectedModel == "" {
		selectedModel = firstModel(provider.Models)
	}
	if selectedModel == "" {
		defaultModel, _ := s.readSystemSettingValue("default_chat_model")
		selectedModel = strings.TrimSpace(defaultModel)
	}

	timeout := s.chatTestTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := llm.NewOpenAICompatClient(provider.BaseURL, provider.APIKey, selectedModel, normalizeProviderAPIMode(provider.APIMode), &http.Client{Timeout: timeout})
	deltaCh, errCh, err := client.StreamChat(ctx, llm.ChatRequest{
		ConversationID: "admin-provider-chat-test",
		RobotID:        "admin-provider-chat-test",
		Prompt:         strings.TrimSpace(message),
		Model:          selectedModel,
		SystemContext:  map[string]any{"source": "admin-provider-chat-test"},
	})
	if err != nil {
		return "", err
	}

	var reply strings.Builder
	for deltaCh != nil || errCh != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case delta, ok := <-deltaCh:
			if !ok {
				deltaCh = nil
				continue
			}
			reply.WriteString(delta)
		case streamErr, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if streamErr != nil {
				return "", streamErr
			}
		}
	}

	text := strings.TrimSpace(reply.String())
	if text == "" {
		return "", errors.New("provider returned empty response")
	}
	return text, nil
}
