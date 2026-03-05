package memory

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

type QdrantClient struct {
	baseURL    string
	apiKey     string
	collection string
	httpClient *http.Client
}

type QdrantSearchResult struct {
	ID      string
	Content string
	Topics  []string
}

func NewQdrantClient(baseURL, apiKey, collection string, httpClient *http.Client) *QdrantClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &QdrantClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     strings.TrimSpace(apiKey),
		collection: strings.TrimSpace(collection),
		httpClient: httpClient,
	}
}

func (c *QdrantClient) Search(ctx context.Context, userID, botID, query string, limit int) ([]QdrantSearchResult, error) {
	if c == nil || c.baseURL == "" || c.collection == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	payload := map[string]any{
		"vector": []float64{0.1, 0.2, 0.3},
		"limit":  limit,
		"filter": map[string]any{
			"must": []map[string]any{
				{"key": "user_id", "match": map[string]any{"value": userID}},
				{"key": "bot_id", "match": map[string]any{"value": botID}},
				{"key": "status", "match": map[string]any{"value": "active"}},
			},
		},
		"params": map[string]any{
			"query": query,
		},
	}
	raw, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, c.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qdrant search status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Result []struct {
			ID      any `json:"id"`
			Payload struct {
				Content string   `json:"content"`
				Topics  []string `json:"topics"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]QdrantSearchResult, 0, len(parsed.Result))
	for _, item := range parsed.Result {
		out = append(out, QdrantSearchResult{
			ID:      fmt.Sprintf("%v", item.ID),
			Content: strings.TrimSpace(item.Payload.Content),
			Topics:  append([]string(nil), item.Payload.Topics...),
		})
	}
	return out, nil
}

func (c *QdrantClient) Upsert(ctx context.Context, points []map[string]any) error {
	if c == nil || c.baseURL == "" || c.collection == "" || len(points) == 0 {
		return nil
	}
	payload := map[string]any{"points": points}
	raw, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", c.baseURL, c.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant upsert status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
