package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatClient struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

func NewOpenAICompatClient(baseURL, apiKey, defaultModel string, httpClient *http.Client) *OpenAICompatClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAICompatClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       strings.TrimSpace(apiKey),
		defaultModel: strings.TrimSpace(defaultModel),
		httpClient:   httpClient,
	}
}

func (c *OpenAICompatClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan string, <-chan error, error) {
	if c == nil || c.httpClient == nil || strings.TrimSpace(c.baseURL) == "" {
		return nil, nil, errors.New("openai-compatible client not configured")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.defaultModel
	}
	if model == "" {
		model = "MiniMax-M2.5"
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
		"stream": true,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("request chat completions: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("chat completions status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	deltaCh := make(chan string, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(deltaCh)
		defer close(errCh)
		defer resp.Body.Close()

		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.Contains(contentType, "text/event-stream") {
			if err := decodeSSEStream(ctx, resp.Body, deltaCh); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
			return
		}

		if err := decodeJSONResponse(resp.Body, deltaCh); err != nil {
			errCh <- err
		}
	}()

	return deltaCh, errCh, nil
}

type streamEnvelope struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func decodeSSEStream(ctx context.Context, body io.Reader, deltaCh chan<- string) error {
	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			return nil
		}
		var env streamEnvelope
		if err := json.Unmarshal([]byte(data), &env); err != nil {
			return fmt.Errorf("decode sse chunk: %w", err)
		}
		if len(env.Choices) == 0 {
			continue
		}
		delta := env.Choices[0].Delta.Content
		if strings.TrimSpace(delta) == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case deltaCh <- delta:
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan sse stream: %w", err)
	}
	return nil
}

func decodeJSONResponse(body io.Reader, deltaCh chan<- string) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var env streamEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(env.Choices) == 0 {
		return nil
	}
	content := strings.TrimSpace(env.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(env.Choices[0].Delta.Content)
	}
	if content == "" {
		return nil
	}
	deltaCh <- content
	return nil
}
