package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestOpenAICompatClientUsesResponsesEndpoint(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q, want /v1/responses", req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		payload := string(body)
		if !strings.Contains(payload, `"model":"gpt-4.1-mini"`) {
			t.Fatalf("request body missing model: %s", payload)
		}
		if !strings.Contains(payload, `[System]`) || !strings.Contains(payload, `你是温柔的朋友`) {
			t.Fatalf("request body missing system prompt: %s", payload)
		}
		if !strings.Contains(payload, `[Assistant]`) || !strings.Contains(payload, `我会记得你喜欢咖啡`) {
			t.Fatalf("request body missing assistant history: %s", payload)
		}
		if !strings.Contains(payload, `[User]`) || !strings.Contains(payload, `你好`) {
			t.Fatalf("request body missing user input: %s", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"output": [
				  {"type":"message","content":[{"type":"output_text","text":"你好，我在。"}]}
				]
			}`)),
		}, nil
	})}

	compat := NewOpenAICompatClient("https://api.openai.com/v1", "sk-test", "gpt-4.1-mini", "responses", client)
	deltaCh, errCh, err := compat.StreamChat(context.Background(), ChatRequest{
		Prompt:       "你好",
		SystemPrompt: "你是温柔的朋友。",
		History: []ChatMessage{
			{Role: "assistant", Content: "我会记得你喜欢咖啡。"},
		},
	})
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var got strings.Builder
	for deltaCh != nil || errCh != nil {
		select {
		case delta, ok := <-deltaCh:
			if !ok {
				deltaCh = nil
				continue
			}
			got.WriteString(delta)
		case streamErr, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if streamErr != nil {
				t.Fatalf("stream error: %v", streamErr)
			}
		}
	}

	if got.String() != "你好，我在。" {
		t.Fatalf("reply = %q, want %q", got.String(), "你好，我在。")
	}
}

func TestDecodeSSEStreamDedupesResponsesCumulativeText(t *testing.T) {
	raw := strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"今晚我会\"}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"今晚我会记得你喜欢海边散步。\"}\n\n" +
		"data: [DONE]\n\n")
	deltaCh := make(chan string, 4)
	if err := decodeSSEStream(context.Background(), raw, deltaCh, APIModeResponses); err != nil {
		t.Fatalf("decodeSSEStream failed: %v", err)
	}
	close(deltaCh)

	var got strings.Builder
	for delta := range deltaCh {
		got.WriteString(delta)
	}
	if got.String() != "今晚我会记得你喜欢海边散步。" {
		t.Fatalf("deduped delta = %q", got.String())
	}
}

func TestOpenAICompatClientUsesChatCompletionsEndpoint(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		payload := string(body)
		if !strings.Contains(payload, `"role":"system"`) || !strings.Contains(payload, `你是GrokBot`) {
			t.Fatalf("request body missing system prompt: %s", payload)
		}
		if !strings.Contains(payload, `"role":"assistant"`) || !strings.Contains(payload, `我最喜欢热拿铁`) {
			t.Fatalf("request body missing assistant history: %s", payload)
		}
		if !strings.Contains(payload, `"role":"user"`) || !strings.Contains(payload, `hello`) {
			t.Fatalf("request body missing user prompt: %s", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"completion-ok"}}]}`)),
		}, nil
	})}

	compat := NewOpenAICompatClient("https://api.openai.com/v1", "sk-test", "gpt-4o-mini", "chat_completions", client)
	deltaCh, errCh, err := compat.StreamChat(context.Background(), ChatRequest{
		Prompt:       "hello",
		SystemPrompt: "你是GrokBot，要像朋友一样聊天。",
		History: []ChatMessage{
			{Role: "assistant", Content: "我最喜欢热拿铁。"},
		},
	})
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var got strings.Builder
	for deltaCh != nil || errCh != nil {
		select {
		case delta, ok := <-deltaCh:
			if !ok {
				deltaCh = nil
				continue
			}
			got.WriteString(delta)
		case streamErr, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if streamErr != nil {
				t.Fatalf("stream error: %v", streamErr)
			}
		}
	}

	if got.String() != "completion-ok" {
		t.Fatalf("reply = %q, want %q", got.String(), "completion-ok")
	}
}

func TestOpenAICompatClientRetriesTooManyRequests(t *testing.T) {
	previousDelay := openAICompatRetryBaseDelay
	openAICompatRetryBaseDelay = time.Millisecond
	defer func() { openAICompatRetryBaseDelay = previousDelay }()

	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"Concurrency limit exceeded"}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"retry-ok"}}]}`)),
		}, nil
	})}

	compat := NewOpenAICompatClient("https://api.openai.com/v1", "sk-test", "gpt-4o-mini", "chat_completions", client)
	deltaCh, errCh, err := compat.StreamChat(context.Background(), ChatRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var got strings.Builder
	for deltaCh != nil || errCh != nil {
		select {
		case delta, ok := <-deltaCh:
			if !ok {
				deltaCh = nil
				continue
			}
			got.WriteString(delta)
		case streamErr, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if streamErr != nil {
				t.Fatalf("stream error: %v", streamErr)
			}
		}
	}

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if got.String() != "retry-ok" {
		t.Fatalf("reply = %q, want %q", got.String(), "retry-ok")
	}
}

func TestOpenAICompatClientReturnsLastTooManyRequestsError(t *testing.T) {
	previousDelay := openAICompatRetryBaseDelay
	openAICompatRetryBaseDelay = time.Millisecond
	defer func() { openAICompatRetryBaseDelay = previousDelay }()

	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"Concurrency limit exceeded"}`)),
		}, nil
	})}

	compat := NewOpenAICompatClient("https://api.openai.com/v1", "sk-test", "gpt-4o-mini", "chat_completions", client)
	_, _, err := compat.StreamChat(context.Background(), ChatRequest{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != openAICompatMaxRetryAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, openAICompatMaxRetryAttempts)
	}
	want := fmt.Sprintf("chat_completions status=%d body=%s", http.StatusTooManyRequests, `{"error":"Concurrency limit exceeded"}`)
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestOpenAICompatEmbedderUsesEmbeddingsEndpoint(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %q, want /v1/embeddings", req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(req.Body)
		payload := string(body)
		if !strings.Contains(payload, `"model":"text-embedding-3-small"`) || !strings.Contains(payload, `喜欢海边`) || !strings.Contains(payload, `"encoding_format":"float"`) {
			t.Fatalf("embedding request body mismatch: %s", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}

	embedder := NewOpenAICompatEmbedder("https://api.openai.com/v1", "sk-test", "text-embedding-3-small", client)
	vector, err := embedder.Embed(context.Background(), "喜欢海边")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(vector) != 3 || vector[0] != 0.1 || vector[2] != 0.3 {
		t.Fatalf("embedding vector = %v", vector)
	}
}
