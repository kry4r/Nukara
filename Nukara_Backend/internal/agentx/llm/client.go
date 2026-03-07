package llm

import (
	"context"
	"errors"
	"strings"

	"nukara/backend/internal/agentx/postprocess"
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

		for _, chunk := range postprocess.SplitSegments(reply, 80) {
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

