package provider

import (
	"testing"

	"nukara/backend/internal/store"
)

type routerTestStore struct {
	providers     []store.Provider
	userSettings  map[string]routeSetting
	botOverrides  map[string]routeSetting
	systemSetting map[string]string
}

type routeSetting struct {
	providerID string
	model      string
}

func (s *routerTestStore) ListProviders() ([]store.Provider, error) {
	return append([]store.Provider(nil), s.providers...), nil
}

func (s *routerTestStore) GetUserProviderSetting(userID string) (string, string, bool) {
	setting, ok := s.userSettings[userID]
	if !ok {
		return "", "", false
	}
	return setting.providerID, setting.model, true
}

func (s *routerTestStore) GetBotProviderOverride(userID, botID string) (string, string, bool) {
	setting, ok := s.botOverrides[userID+":"+botID]
	if !ok {
		return "", "", false
	}
	return setting.providerID, setting.model, true
}

func (s *routerTestStore) GetSystemSetting(key string) (string, bool) {
	value, ok := s.systemSetting[key]
	return value, ok
}

func TestResolveChatRoutePriority(t *testing.T) {
	st := &routerTestStore{
		providers: []store.Provider{
			{ID: "minimax_m2_5", Name: "MiniMax", BaseURL: "https://aigw-gzgy2.cucloud.cn:8443/v1", APIKey: "sk-minimax", Models: []string{"MiniMax-M2.5"}, IsActive: true, Priority: 1},
			{ID: "fallback", Name: "Fallback", BaseURL: "https://fallback.local/v1", APIKey: "sk-fallback", Models: []string{"fallback-model"}, IsActive: true, Priority: 2},
		},
		userSettings: map[string]routeSetting{
			"user-1": {providerID: "fallback", model: "fallback-user-model"},
		},
		botOverrides: map[string]routeSetting{
			"user-1:bot-1": {providerID: "minimax_m2_5", model: "MiniMax-M2.5-override"},
		},
		systemSetting: map[string]string{
			"default_chat_provider_id": "minimax_m2_5",
			"default_chat_model":       "MiniMax-M2.5",
		},
	}

	router := NewRouter(st)
	route, err := router.ResolveChatRoute("user-1", "bot-1")
	if err != nil {
		t.Fatalf("resolve chat route failed: %v", err)
	}
	if route.ProviderID != "minimax_m2_5" {
		t.Fatalf("provider = %s, want minimax_m2_5", route.ProviderID)
	}
	if route.Model != "MiniMax-M2.5-override" {
		t.Fatalf("model = %s, want MiniMax-M2.5-override", route.Model)
	}
}

func TestResolveChatRouteFallsBackToSystemDefault(t *testing.T) {
	st := &routerTestStore{
		providers: []store.Provider{
			{ID: "minimax_m2_5", Name: "MiniMax", BaseURL: "https://aigw-gzgy2.cucloud.cn:8443/v1", APIKey: "sk-minimax", Models: []string{"MiniMax-M2.5"}, IsActive: true, Priority: 1},
			{ID: "fallback", Name: "Fallback", BaseURL: "https://fallback.local/v1", APIKey: "sk-fallback", Models: []string{"fallback-model"}, IsActive: true, Priority: 2},
		},
		userSettings: map[string]routeSetting{
			"user-1": {providerID: "missing-provider", model: "bad-model"},
		},
		botOverrides: map[string]routeSetting{
			"user-1:bot-1": {providerID: "missing-provider", model: "bad-override-model"},
		},
		systemSetting: map[string]string{
			"default_chat_provider_id": "minimax_m2_5",
			"default_chat_model":       "MiniMax-M2.5",
		},
	}

	router := NewRouter(st)
	route, err := router.ResolveChatRoute("user-1", "bot-1")
	if err != nil {
		t.Fatalf("resolve chat route failed: %v", err)
	}
	if route.ProviderID != "minimax_m2_5" {
		t.Fatalf("provider = %s, want minimax_m2_5", route.ProviderID)
	}
	if route.Model != "MiniMax-M2.5" {
		t.Fatalf("model = %s, want MiniMax-M2.5", route.Model)
	}
}

func TestResolveChatRouteFallsBackToPriorityProvider(t *testing.T) {
	st := &routerTestStore{
		providers: []store.Provider{
			{ID: "minimax_m2_5", Name: "MiniMax", BaseURL: "https://aigw-gzgy2.cucloud.cn:8443/v1", APIKey: "sk-minimax", Models: []string{"MiniMax-M2.5"}, IsActive: true, Priority: 1},
			{ID: "fallback", Name: "Fallback", BaseURL: "https://fallback.local/v1", APIKey: "sk-fallback", Models: []string{"fallback-model"}, IsActive: true, Priority: 2},
		},
		userSettings:  map[string]routeSetting{},
		botOverrides:  map[string]routeSetting{},
		systemSetting: map[string]string{},
	}

	router := NewRouter(st)
	route, err := router.ResolveChatRoute("user-1", "bot-1")
	if err != nil {
		t.Fatalf("resolve chat route failed: %v", err)
	}
	if route.ProviderID != "minimax_m2_5" {
		t.Fatalf("provider = %s, want minimax_m2_5", route.ProviderID)
	}
	if route.Model != "MiniMax-M2.5" {
		t.Fatalf("model = %s, want MiniMax-M2.5", route.Model)
	}
}
