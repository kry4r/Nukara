package llm

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	ConversationID string
	RobotID        string
	Prompt         string
	SystemContext  map[string]any
	SystemPrompt   string
	History        []ChatMessage
	Model          string
}

type StreamClient interface {
	StreamChat(ctx context.Context, req ChatRequest) (<-chan string, <-chan error, error)
}

type syncChatClient interface {
	Chat(ctx context.Context, convID, robotID, text string, systemContext map[string]any) (string, error)
}

type LegacyAgentClient struct {
	client syncChatClient
}

func NewLegacyAgentClient(client syncChatClient) *LegacyAgentClient {
	return &LegacyAgentClient{client: client}
}

func (c *LegacyAgentClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan string, <-chan error, error) {
	if c == nil || c.client == nil {
		return nil, nil, errors.New("legacy chat client is nil")
	}

	deltaCh := make(chan string, 8)
	errCh := make(chan error, 1)

	go func() {
		defer close(deltaCh)
		defer close(errCh)

		reply, err := c.client.Chat(ctx, req.ConversationID, req.RobotID, req.Prompt, req.SystemContext)
		if err != nil {
			errCh <- err
			return
		}
		reply = strings.TrimSpace(reply)
		if reply == "" {
			return
		}

		for _, chunk := range splitByRuneWindow(reply, 8) {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case deltaCh <- chunk:
			}
		}
	}()

	return deltaCh, errCh, nil
}

func splitByRuneWindow(s string, window int) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if window <= 0 {
		return []string{s}
	}

	var out []string
	var b strings.Builder
	runes := 0
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			s = s[size:]
			continue
		}
		b.WriteRune(r)
		runes++
		s = s[size:]
		if runes >= window {
			out = append(out, b.String())
			b.Reset()
			runes = 0
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
