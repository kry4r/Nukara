package agentx

import (
	"context"
	"errors"
	"log"
	"strings"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/agentx/postprocess"
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
			return llm.NewOpenAICompatClient(route.BaseURL, route.APIKey, route.Model, route.APIMode, nil)
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
	providerBaseURL := ""
	request := llm.ChatRequest{
		ConversationID: req.ConversationID,
		RobotID:        req.BotID,
		Prompt:         req.AggregatedText,
		SystemContext:  req.SystemContext,
		SystemPrompt:   req.SystemPrompt,
		History:        append([]llm.ChatMessage(nil), req.History...),
	}
	if r.routeResolver != nil && r.clientFactory != nil {
		if route, routeErr := r.routeResolver.ResolveChatRoute(req.UserID, req.BotID); routeErr == nil && strings.TrimSpace(route.BaseURL) != "" {
			client = r.clientFactory(route)
			request.Model = route.Model
			providerMode = "route:" + strings.TrimSpace(route.ProviderID)
			providerBaseURL = strings.TrimSpace(route.BaseURL)
		}
	}
	log.Printf("[agentx-runtime] conv=%s mode=%s base=%s model=%s prompt_chars=%d", req.ConversationID, providerMode, providerBaseURL, strings.TrimSpace(request.Model), len([]rune(strings.TrimSpace(req.AggregatedText))))
	if client == nil {
		return nil, nil, errors.New("chat provider client is not configured")
	}

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
				if err != nil {
					log.Printf("[agentx-runtime] stream error conv=%s mode=%s model=%s err=%v", req.ConversationID, providerMode, strings.TrimSpace(request.Model), err)
				}
				errCh = nil
			}
		}

		fullRaw := strings.TrimSpace(rawText.String())
		if fullRaw == "" {
			log.Printf("[agentx-runtime] empty provider response conv=%s mode=%s model=%s", req.ConversationID, providerMode, strings.TrimSpace(request.Model))
			finalCh <- fallbackTurn()
			return
		}

		clean, statusEmoji, statusText := agent.ExtractStatus(agent.SanitizeLLMReply(fullRaw), "")
		clean, emotion := agent.ExtractEmotion(clean)
		clean = strings.TrimSpace(postprocess.StripSegmentProtocol(clean))
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
