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

type Neo4jClient struct {
	baseURL    string
	user       string
	pass       string
	httpClient *http.Client
}

type TopicEdge struct {
	Name   string
	Weight float64
}

func NewNeo4jClient(baseURL, user, pass string, httpClient *http.Client) *Neo4jClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Neo4jClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		user:       strings.TrimSpace(user),
		pass:       strings.TrimSpace(pass),
		httpClient: httpClient,
	}
}

func (c *Neo4jClient) ExpandTopics(ctx context.Context, seeds []string, limit int) ([]TopicEdge, error) {
	if c == nil || c.baseURL == "" || len(seeds) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	payload := map[string]any{
		"topics": seeds,
		"limit":  limit,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/query/topics", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("neo4j expand status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Topics []struct {
			Name   string  `json:"name"`
			Weight float64 `json:"weight"`
		} `json:"topics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]TopicEdge, 0, len(parsed.Topics))
	for _, topic := range parsed.Topics {
		name := strings.TrimSpace(topic.Name)
		if name == "" {
			continue
		}
		out = append(out, TopicEdge{Name: name, Weight: topic.Weight})
	}
	return out, nil
}

func (c *Neo4jClient) WriteMemoryTopicLinks(ctx context.Context, memoryID string, topics []string) error {
	if c == nil || c.baseURL == "" || strings.TrimSpace(memoryID) == "" {
		return nil
	}
	payload := map[string]any{
		"memory_id": strings.TrimSpace(memoryID),
		"topics":    topics,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/query/upsert-memory-topics", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("neo4j write status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
