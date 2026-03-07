package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatEmbedder struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

func NewOpenAICompatEmbedder(baseURL, apiKey, defaultModel string, httpClient *http.Client) *OpenAICompatEmbedder {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &OpenAICompatEmbedder{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       strings.TrimSpace(apiKey),
		defaultModel: strings.TrimSpace(defaultModel),
		httpClient:   httpClient,
	}
}

func (e *OpenAICompatEmbedder) Embed(ctx context.Context, input string) ([]float64, error) {
	if e == nil || e.httpClient == nil || strings.TrimSpace(e.baseURL) == "" {
		return nil, nil
	}
	model := strings.TrimSpace(e.defaultModel)
	if model == "" || strings.TrimSpace(input) == "" {
		return nil, nil
	}
	payload := map[string]any{
		"model":           model,
		"input":           input,
		"encoding_format": "float",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embeddings status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 {
		return nil, nil
	}
	return append([]float64(nil), parsed.Data[0].Embedding...), nil
}
