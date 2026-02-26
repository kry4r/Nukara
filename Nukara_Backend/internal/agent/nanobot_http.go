package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// nanobotHTTPClient is an HTTP client for the nanobot extend-chat /chat endpoint.
type nanobotHTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newNanobotHTTPClient(baseURL, token string) *nanobotHTTPClient {
	return &nanobotHTTPClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

type chatRequest struct {
	ConversationID string         `json:"conversation_id"`
	RobotID        string         `json:"robot_id"`
	Content        *EventContent  `json:"content"`
	SystemContext   map[string]any `json:"system_context,omitempty"`
}

type chatResponse struct {
	ConversationID string        `json:"conversation_id"`
	Content        *EventContent `json:"content"`
}

// Chat sends a synchronous chat request and returns the full reply text.
func (c *nanobotHTTPClient) Chat(ctx context.Context, convID, robotID, text string, systemContext map[string]any) (string, error) {
	body := chatRequest{
		ConversationID: convID,
		RobotID:        robotID,
		Content:        &EventContent{Type: "text", Text: text},
		SystemContext:   systemContext,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nanobot returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Content != nil {
		return result.Content.Text, nil
	}
	return "", nil
}
