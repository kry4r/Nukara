package agentx

import (
	"context"
	"sync"
	"testing"

	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/agentx/provider"
	"nukara/backend/internal/store"
)

type captureStreamClient struct {
	mu      sync.Mutex
	calls   int
	lastReq llm.ChatRequest
	chunks  []string
}

func (c *captureStreamClient) StreamChat(ctx context.Context, req llm.ChatRequest) (<-chan string, <-chan error, error) {
	c.mu.Lock()
	c.calls++
	c.lastReq = req
	chunks := append([]string(nil), c.chunks...)
	c.mu.Unlock()

	deltaCh := make(chan string, len(chunks))
	errCh := make(chan error, 1)
	for _, chunk := range chunks {
		deltaCh <- chunk
	}
	close(deltaCh)
	close(errCh)
	return deltaCh, errCh, nil
}

func (c *captureStreamClient) snapshot() (int, llm.ChatRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.lastReq
}

type staticResolver struct {
	route provider.Route
	err   error
}

func (r staticResolver) ResolveChatRoute(userID, botID string) (provider.Route, error) {
	return r.route, r.err
}

type staticSubtaskResolver struct {
	chatRoute                 provider.Route
	subtaskRoute              provider.Route
	selfCognitionSummaryRoute provider.Route
	err                       error
}

func (r staticSubtaskResolver) ResolveChatRoute(userID, botID string) (provider.Route, error) {
	return r.chatRoute, r.err
}

func (r staticSubtaskResolver) ResolveSubtaskRoute(userID, botID string) (provider.Route, error) {
	return r.subtaskRoute, r.err
}

func (r staticSubtaskResolver) ResolveSelfCognitionSummaryRoute(userID, botID string) (provider.Route, error) {
	return r.selfCognitionSummaryRoute, r.err
}

func readFinalText(t *testing.T, finalCh <-chan FinalTurn) string {
	t.Helper()
	final, ok := <-finalCh
	if !ok {
		t.Fatalf("final channel closed without final turn")
	}
	if len(final.Segments) == 0 {
		t.Fatalf("final turn has no segments")
	}
	return final.Segments[0].Text
}

type resolverSetting struct {
	providerID string
	model      string
}

type resolverStoreStub struct {
	providers      []store.Provider
	systemSettings map[string]string
	userSettings   map[string]resolverSetting
	botSettings    map[string]resolverSetting
}

func (s resolverStoreStub) ListProviders() ([]store.Provider, error) {
	return append([]store.Provider(nil), s.providers...), nil
}

func (s resolverStoreStub) GetUserProviderSetting(userID string) (providerID, model string, ok bool) {
	v, ok := s.userSettings[userID]
	if !ok {
		return "", "", false
	}
	return v.providerID, v.model, true
}

func (s resolverStoreStub) GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool) {
	v, ok := s.botSettings[userID+":"+botID]
	if !ok {
		return "", "", false
	}
	return v.providerID, v.model, true
}

func (s resolverStoreStub) GetSystemSetting(key string) (value string, ok bool) {
	value, ok = s.systemSettings[key]
	return value, ok
}

func TestRuntimeUsesUserSelectedProviderRoute(t *testing.T) {
	defaultClient := &captureStreamClient{chunks: []string{"fallback"}}
	routeClient := &captureStreamClient{chunks: []string{"route-provider-hit"}}
	resolverStore := resolverStoreStub{
		providers: []store.Provider{
			{ID: "provider_default", BaseURL: "http://default.local", APIKey: "default-key", Models: []string{"default-model"}, APIMode: "chat_completions", IsActive: true, Priority: 1},
			{ID: "provider_user", BaseURL: "http://user.local", APIKey: "user-key", Models: []string{"user-model"}, APIMode: "responses", Priority: 2},
		},
		systemSettings: map[string]string{
			"default_chat_provider_id": "provider_default",
			"default_chat_model":       "default-model",
		},
		userSettings: map[string]resolverSetting{
			"user-1": {providerID: "provider_user", model: "user-model"},
		},
		botSettings: map[string]resolverSetting{},
	}

	var routeSeen provider.Route
	rt := NewRuntime(RuntimeDeps{
		ProviderClient: defaultClient,
		RouteResolver:  provider.NewRouter(resolverStore),
		ClientFactory: func(route provider.Route) llm.StreamClient {
			routeSeen = route
			return routeClient
		},
	})

	_, finalCh, err := rt.StreamTurn(context.Background(), TurnRequest{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		AggregatedText: "hello",
	})
	if err != nil {
		t.Fatalf("StreamTurn failed: %v", err)
	}

	if got := readFinalText(t, finalCh); got != "route-provider-hit" {
		t.Fatalf("final text = %q, want %q", got, "route-provider-hit")
	}
	if routeSeen.ProviderID != "provider_user" {
		t.Fatalf("route provider = %q, want provider_user", routeSeen.ProviderID)
	}
	if routeSeen.APIMode != "responses" {
		t.Fatalf("route api mode = %q, want responses", routeSeen.APIMode)
	}

	defaultCalls, _ := defaultClient.snapshot()
	if defaultCalls != 0 {
		t.Fatalf("default client should not be called, got %d calls", defaultCalls)
	}

	routeCalls, routeReq := routeClient.snapshot()
	if routeCalls != 1 {
		t.Fatalf("route client calls = %d, want 1", routeCalls)
	}
	if routeReq.Model != "user-model" {
		t.Fatalf("route model = %q, want user-model", routeReq.Model)
	}
}

func TestRuntimeFallsBackWhenResolvedRouteMissingBaseURL(t *testing.T) {
	defaultClient := &captureStreamClient{chunks: []string{"default-hit"}}
	routeClient := &captureStreamClient{chunks: []string{"should-not-hit"}}

	rt := NewRuntime(RuntimeDeps{
		ProviderClient: defaultClient,
		RouteResolver: staticResolver{route: provider.Route{
			ProviderID: "provider_b",
			BaseURL:    "",
		}},
		ClientFactory: func(route provider.Route) llm.StreamClient {
			return routeClient
		},
	})

	_, finalCh, err := rt.StreamTurn(context.Background(), TurnRequest{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		AggregatedText: "hello",
	})
	if err != nil {
		t.Fatalf("StreamTurn failed: %v", err)
	}

	if got := readFinalText(t, finalCh); got != "default-hit" {
		t.Fatalf("final text = %q, want %q", got, "default-hit")
	}

	defaultCalls, _ := defaultClient.snapshot()
	if defaultCalls != 1 {
		t.Fatalf("default client calls = %d, want 1", defaultCalls)
	}
	routeCalls, _ := routeClient.snapshot()
	if routeCalls != 0 {
		t.Fatalf("route client should not be called, got %d calls", routeCalls)
	}
}

func TestRuntimeForwardsSystemPromptAndHistory(t *testing.T) {
	client := &captureStreamClient{chunks: []string{"ok"}}
	rt := NewRuntime(RuntimeDeps{ProviderClient: client})

	_, finalCh, err := rt.StreamTurn(context.Background(), TurnRequest{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		AggregatedText: "你还记得吗",
		SystemPrompt:   "你是GrokBot，要像朋友一样聊天。",
		History: []llm.ChatMessage{
			{Role: "assistant", Content: "我刚才说我最喜欢热拿铁。"},
		},
	})
	if err != nil {
		t.Fatalf("StreamTurn failed: %v", err)
	}
	_ = readFinalText(t, finalCh)

	_, req := client.snapshot()
	if req.SystemPrompt != "你是GrokBot，要像朋友一样聊天。" {
		t.Fatalf("system prompt = %q", req.SystemPrompt)
	}
	if len(req.History) != 1 || req.History[0].Content != "我刚才说我最喜欢热拿铁。" {
		t.Fatalf("history forwarded incorrectly: %#v", req.History)
	}
}

func TestRuntimePreservesExplicitMessageBoundaryProtocolInFinalTurn(t *testing.T) {
	client := &captureStreamClient{chunks: []string{"先去吃饭<<<MSG>>>吃完再和我说"}}
	rt := NewRuntime(RuntimeDeps{ProviderClient: client})

	_, finalCh, err := rt.StreamTurn(context.Background(), TurnRequest{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		AggregatedText: "你现在要去做什么",
	})
	if err != nil {
		t.Fatalf("StreamTurn failed: %v", err)
	}

	if got := readFinalText(t, finalCh); got != "先去吃饭<<<MSG>>>吃完再和我说" {
		t.Fatalf("final text = %q", got)
	}
}

func TestRuntimeUsesDedicatedSubtaskRouteWhenPurposeIsSubtask(t *testing.T) {
	defaultClient := &captureStreamClient{chunks: []string{"default-hit"}}
	routeClient := &captureStreamClient{chunks: []string{"subtask-hit"}}
	resolver := staticSubtaskResolver{
		chatRoute: provider.Route{
			ProviderID: "provider_chat",
			BaseURL:    "http://chat.local",
			APIKey:     "chat-key",
			Model:      "chat-model",
			APIMode:    "chat_completions",
		},
		subtaskRoute: provider.Route{
			ProviderID: "provider_subtask",
			BaseURL:    "http://subtask.local",
			APIKey:     "subtask-key",
			Model:      "subtask-model",
			APIMode:    "responses",
		},
	}

	var routeSeen provider.Route
	rt := NewRuntime(RuntimeDeps{
		ProviderClient: defaultClient,
		RouteResolver:  resolver,
		ClientFactory: func(route provider.Route) llm.StreamClient {
			routeSeen = route
			return routeClient
		},
	})

	_, finalCh, err := rt.StreamTurn(context.Background(), TurnRequest{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		AggregatedText: "提取记忆",
		Purpose:        "subtask",
	})
	if err != nil {
		t.Fatalf("StreamTurn failed: %v", err)
	}
	if got := readFinalText(t, finalCh); got != "subtask-hit" {
		t.Fatalf("final text = %q", got)
	}
	if routeSeen.ProviderID != "provider_subtask" {
		t.Fatalf("route provider = %q, want provider_subtask", routeSeen.ProviderID)
	}
	if routeSeen.Model != "subtask-model" {
		t.Fatalf("route model = %q", routeSeen.Model)
	}
}

func TestRuntimeUsesDedicatedSelfCognitionSummaryRoute(t *testing.T) {
	defaultClient := &captureStreamClient{chunks: []string{"default-hit"}}
	routeClient := &captureStreamClient{chunks: []string{"summary-hit"}}
	resolver := staticSubtaskResolver{
		chatRoute: provider.Route{
			ProviderID: "provider_chat",
			BaseURL:    "http://chat.local",
			APIKey:     "chat-key",
			Model:      "chat-model",
			APIMode:    "chat_completions",
		},
		selfCognitionSummaryRoute: provider.Route{
			ProviderID: "provider_summary",
			BaseURL:    "http://summary.local",
			APIKey:     "summary-key",
			Model:      "summary-model",
			APIMode:    "responses",
		},
	}

	var routeSeen provider.Route
	rt := NewRuntime(RuntimeDeps{
		ProviderClient: defaultClient,
		RouteResolver:  resolver,
		ClientFactory: func(route provider.Route) llm.StreamClient {
			routeSeen = route
			return routeClient
		},
	})

	_, finalCh, err := rt.StreamTurn(context.Background(), TurnRequest{
		UserID:         "user-1",
		BotID:          "bot-1",
		ConversationID: "conv-1",
		AggregatedText: "总结自我认知",
		Purpose:        "self_cognition_summary",
	})
	if err != nil {
		t.Fatalf("StreamTurn failed: %v", err)
	}
	if got := readFinalText(t, finalCh); got != "summary-hit" {
		t.Fatalf("final text = %q", got)
	}
	if routeSeen.ProviderID != "provider_summary" {
		t.Fatalf("route provider = %q, want provider_summary", routeSeen.ProviderID)
	}
	if routeSeen.Model != "summary-model" {
		t.Fatalf("route model = %q", routeSeen.Model)
	}
}
