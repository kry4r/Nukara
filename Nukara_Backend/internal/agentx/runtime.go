package agentx

import (
	"context"
	"log"
	"strings"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/agentx/provider"
)

type RuntimeDeps struct {
	ProviderClient llm.StreamClient
	RouteResolver  interface {
		ResolveChatRoute(userID, botID string) (provider.Route, error)
	}
	ClientFactory func(route provider.Route) llm.StreamClient
}

type Runtime struct {
	providerClient llm.StreamClient
	routeResolver  interface {
		ResolveChatRoute(userID, botID string) (provider.Route, error)
	}
	clientFactory func(route provider.Route) llm.StreamClient
}

func NewRuntime(deps RuntimeDeps) *Runtime {
	factory := deps.ClientFactory
	if factory == nil {
		factory = func(route provider.Route) llm.StreamClient {
			return llm.NewOpenAICompatClient(route.BaseURL, route.APIKey, route.Model, nil)
		}
	}
	return &Runtime{
		providerClient: deps.ProviderClient,
		routeResolver:  deps.RouteResolver,
		clientFactory:  factory,
	}
}

func (r *Runtime) StreamTurn(ctx context.Context, req TurnRequest) (<-chan StreamDelta, <-chan FinalTurn, error) {
	client := r.providerClient
	providerMode := "default"
	request := llm.ChatRequest{
		ConversationID: req.ConversationID,
		RobotID:        req.BotID,
		Prompt:         req.AggregatedText,
		SystemContext:  req.SystemContext,
	}
	if r.routeResolver != nil && r.clientFactory != nil {
		if route, routeErr := r.routeResolver.ResolveChatRoute(req.UserID, req.BotID); routeErr == nil && strings.TrimSpace(route.BaseURL) != "" {
			client = r.clientFactory(route)
			request.Model = route.Model
			providerMode = "route:" + strings.TrimSpace(route.ProviderID)
		}
	}
	log.Printf("[agentx-runtime] conv=%s mode=%s model=%s prompt_chars=%d", req.ConversationID, providerMode, strings.TrimSpace(request.Model), len([]rune(strings.TrimSpace(req.AggregatedText))))
	rawDeltaCh, errCh, err := client.StreamChat(ctx, request)
	if err != nil {
		return nil, nil, err
	}

	deltaCh := make(chan StreamDelta, 16)
	finalCh := make(chan FinalTurn, 1)

	go func() {
		defer close(deltaCh)
		defer close(finalCh)

		var rawText strings.Builder
		for rawDeltaCh != nil || errCh != nil {
			select {
			case <-ctx.Done():
				finalCh <- fallbackTurn()
				return
			case delta, ok := <-rawDeltaCh:
				if !ok {
					rawDeltaCh = nil
					continue
				}
				if strings.TrimSpace(delta) == "" {
					continue
				}
				rawText.WriteString(delta)
				visible := stripVisibleTags(delta)
				if strings.TrimSpace(visible) == "" {
					continue
				}
				deltaCh <- StreamDelta{Delta: visible}
			case err, ok := <-errCh:
				if !ok {
					errCh = nil
					continue
				}
				_ = err // keep stream stable; fallback below if no content
				errCh = nil
			}
		}

		fullRaw := strings.TrimSpace(rawText.String())
		if fullRaw == "" {
			finalCh <- fallbackTurn()
			return
		}

		clean, statusEmoji, statusText := agent.ExtractStatus(agent.SanitizeLLMReply(fullRaw), "")
		clean, emotion := agent.ExtractEmotion(clean)
		clean = strings.TrimSpace(clean)
		if clean == "" {
			finalCh <- fallbackTurn()
			return
		}
		if statusEmoji == "" || statusEmoji == "☕️" {
			statusEmoji = agent.EmotionDefaultEmoji(emotion)
		}

		finalCh <- FinalTurn{
			Segments: []FinalSegment{
				{
					Text:        clean,
					EmotionTag:  emotion,
					StatusEmoji: statusEmoji,
					StatusText:  statusText,
				},
			},
		}
	}()

	return deltaCh, finalCh, nil
}

func stripVisibleTags(s string) string {
	s = strings.ReplaceAll(s, "[status:", "")
	s = strings.ReplaceAll(s, "[emotion:", "")
	return s
}

func fallbackTurn() FinalTurn {
	return FinalTurn{
		Segments: []FinalSegment{
			{
				Text:        "我在呢，刚刚有点走神了，我们继续聊。",
				EmotionTag:  "gentle",
				StatusEmoji: "💭",
				StatusText:  "在想你",
			},
		},
	}
}
