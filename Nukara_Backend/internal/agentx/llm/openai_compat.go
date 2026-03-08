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

const (
	APIModeChatCompletions       = "chat_completions"
	APIModeResponses             = "responses"
	APIModeAuto                  = "auto"
	openAICompatMaxRetryAttempts = 3
	openAICompatRetryMaxDelay    = 2 * time.Second
)

var openAICompatRetryBaseDelay = 200 * time.Millisecond

type OpenAICompatClient struct {
	baseURL      string
	apiKey       string
	defaultModel string
	apiMode      string
	httpClient   *http.Client
}

func NewOpenAICompatClient(baseURL, apiKey, defaultModel, apiMode string, httpClient *http.Client) *OpenAICompatClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAICompatClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       strings.TrimSpace(apiKey),
		defaultModel: strings.TrimSpace(defaultModel),
		apiMode:      normalizeOpenAICompatMode(apiMode),
		httpClient:   httpClient,
	}
}

func normalizeOpenAICompatMode(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case APIModeResponses:
		return APIModeResponses
	case APIModeAuto:
		return APIModeAuto
	default:
		return APIModeChatCompletions
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

	modes := []string{c.apiMode}
	if c.apiMode == APIModeAuto {
		modes = []string{APIModeResponses, APIModeChatCompletions}
	}

	var lastErr error
	for _, mode := range modes {
		deltaCh, errCh, err := c.streamWithMode(ctx, req, model, mode)
		if err == nil {
			return deltaCh, errCh, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("openai-compatible client failed without a concrete error")
	}
	return nil, nil, lastErr
}

func (c *OpenAICompatClient) streamWithMode(ctx context.Context, req ChatRequest, model, mode string) (<-chan string, <-chan error, error) {
	payload, endpoint, err := buildOpenAICompatRequest(req, model, mode)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %s request: %w", mode, err)
	}

	var resp *http.Response
	for attempt := 0; attempt < openAICompatMaxRetryAttempts; attempt++ {
		httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(raw))
		if reqErr != nil {
			return nil, nil, fmt.Errorf("create %s request: %w", mode, reqErr)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err = c.httpClient.Do(httpReq)
		if err != nil {
			return nil, nil, fmt.Errorf("request %s: %w", mode, err)
		}
		if resp.StatusCode == http.StatusOK {
			break
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		bodyText := strings.TrimSpace(string(body))
		if !shouldRetryOpenAICompatStatus(resp.StatusCode) || attempt == openAICompatMaxRetryAttempts-1 {
			return nil, nil, fmt.Errorf("%s status=%d body=%s", mode, resp.StatusCode, bodyText)
		}
		if err := waitOpenAICompatRetry(ctx, attempt, resp.Header.Get("Retry-After")); err != nil {
			return nil, nil, err
		}
	}
	if resp == nil {
		return nil, nil, fmt.Errorf("request %s: empty response", mode)
	}

	deltaCh := make(chan string, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(deltaCh)
		defer close(errCh)
		defer resp.Body.Close()

		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.Contains(contentType, "text/event-stream") {
			if err := decodeSSEStream(ctx, resp.Body, deltaCh, mode); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
			return
		}

		if err := decodeJSONResponse(resp.Body, deltaCh, mode); err != nil {
			errCh <- err
		}
	}()

	return deltaCh, errCh, nil
}

func shouldRetryOpenAICompatStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests
}

func waitOpenAICompatRetry(ctx context.Context, attempt int, retryAfter string) error {
	delay := parseRetryAfterDelay(retryAfter)
	if delay <= 0 {
		delay = openAICompatRetryBaseDelay << attempt
	}
	if delay > openAICompatRetryMaxDelay {
		delay = openAICompatRetryMaxDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseRetryAfterDelay(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil && seconds > 0 {
		return seconds
	}
	if retryAt, err := http.ParseTime(raw); err == nil {
		delay := time.Until(retryAt)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

func buildOpenAICompatRequest(req ChatRequest, model, mode string) (map[string]any, string, error) {
	messages := buildOpenAICompatMessages(req)
	switch normalizeOpenAICompatMode(mode) {
	case APIModeResponses:
		return map[string]any{
			"model":  model,
			"input":  flattenResponsesInput(messages),
			"stream": true,
		}, "/responses", nil
	case APIModeChatCompletions:
		payloadMessages := make([]map[string]string, 0, len(messages))
		for _, message := range messages {
			payloadMessages = append(payloadMessages, map[string]string{
				"role":    normalizeMessageRole(message.Role),
				"content": message.Content,
			})
		}
		return map[string]any{
			"model":    model,
			"messages": payloadMessages,
			"stream":   true,
		}, "/chat/completions", nil
	default:
		return nil, "", fmt.Errorf("unsupported openai api mode: %s", mode)
	}
}

func buildOpenAICompatMessages(req ChatRequest) []ChatMessage {
	messages := make([]ChatMessage, 0, len(req.History)+2)
	if systemPrompt := strings.TrimSpace(req.SystemPrompt); systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	for _, message := range req.History {
		role := normalizeMessageRole(message.Role)
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, ChatMessage{Role: role, Content: content})
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: prompt})
	}
	return messages
}

func normalizeMessageRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "system":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return "user"
	}
}

func flattenResponsesInput(messages []ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		role := normalizeMessageRole(message.Role)
		switch role {
		case "system":
			builder.WriteString("[System]\n")
		case "assistant":
			builder.WriteString("[Assistant]\n")
		default:
			builder.WriteString("[User]\n")
		}
		builder.WriteString(content)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

type chatCompletionEnvelope struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type responsesEnvelope struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func decodeSSEStream(ctx context.Context, body io.Reader, deltaCh chan<- string, mode string) error {
	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	responsesAccumulated := ""

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
		var delta string
		switch normalizeOpenAICompatMode(mode) {
		case APIModeResponses:
			delta = extractResponsesEventText([]byte(data))
			if strings.HasPrefix(delta, responsesAccumulated) {
				delta = strings.TrimPrefix(delta, responsesAccumulated)
			}
			if strings.TrimSpace(delta) != "" {
				responsesAccumulated += delta
			}
		default:
			delta = extractChatCompletionEventText([]byte(data))
		}
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
		return fmt.Errorf("scan %s stream: %w", mode, err)
	}
	return nil
}

func decodeJSONResponse(body io.Reader, deltaCh chan<- string, mode string) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read %s response: %w", mode, err)
	}

	var content string
	switch normalizeOpenAICompatMode(mode) {
	case APIModeResponses:
		content = extractResponsesText(raw)
	default:
		content = extractChatCompletionText(raw)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	deltaCh <- content
	return nil
}

func extractChatCompletionEventText(raw []byte) string {
	var env chatCompletionEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ""
	}
	if len(env.Choices) == 0 {
		return ""
	}
	content := strings.TrimSpace(env.Choices[0].Delta.Content)
	if content == "" {
		content = strings.TrimSpace(env.Choices[0].Message.Content)
	}
	return content
}

func extractChatCompletionText(raw []byte) string {
	var env chatCompletionEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ""
	}
	if len(env.Choices) == 0 {
		return ""
	}
	content := strings.TrimSpace(env.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(env.Choices[0].Delta.Content)
	}
	return content
}

func extractResponsesEventText(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	typeName, _ := payload["type"].(string)
	typeName = strings.TrimSpace(typeName)
	if delta, _ := payload["delta"].(string); strings.Contains(typeName, "output_text") && strings.TrimSpace(delta) != "" {
		return delta
	}
	if text, _ := payload["text"].(string); strings.Contains(typeName, "output_text") && strings.TrimSpace(text) != "" {
		return text
	}
	if outputText, _ := payload["output_text"].(string); strings.TrimSpace(outputText) != "" {
		return outputText
	}
	if nested := extractResponsesText(raw); strings.TrimSpace(nested) != "" {
		return nested
	}
	return ""
}

func extractResponsesText(raw []byte) string {
	var env responsesEnvelope
	if err := json.Unmarshal(raw, &env); err == nil {
		if strings.TrimSpace(env.OutputText) != "" {
			return strings.TrimSpace(env.OutputText)
		}
		for _, item := range env.Output {
			for _, content := range item.Content {
				if strings.EqualFold(strings.TrimSpace(content.Type), "output_text") && strings.TrimSpace(content.Text) != "" {
					return strings.TrimSpace(content.Text)
				}
			}
		}
	}
	return ""
}
