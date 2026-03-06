package provider

import (
	"testing"

	"nukara/backend/internal/store"
)

type setting struct {
	providerID string
	model      string
}

type resolverStoreStub struct {
	providers      []store.Provider
	systemSettings map[string]string
	userSettings   map[string]setting
	botSettings    map[string]setting
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

func TestResolveEmbeddingRouteUsesEmbeddingSettings(t *testing.T) {
	router := NewRouter(resolverStoreStub{
		providers: []store.Provider{
			{ID: "chat_default", BaseURL: "https://chat.local/v1", APIKey: "chat-key", Models: []string{"chat-model"}, IsActive: true, Priority: 1},
			{ID: "embed_provider", BaseURL: "https://embed.local/v1", APIKey: "embed-key", Models: []string{"embed-model-fallback"}, IsActive: false, Priority: 2},
		},
		systemSettings: map[string]string{
			"default_chat_provider_id": "chat_default",
			"default_chat_model":       "chat-model",
			"embedding_provider_id":    "embed_provider",
			"embedding_model":          "text-embedding-3-small",
		},
	})

	route, err := router.ResolveEmbeddingRoute()
	if err != nil {
		t.Fatalf("ResolveEmbeddingRoute failed: %v", err)
	}
	if route.ProviderID != "embed_provider" {
		t.Fatalf("provider id = %s", route.ProviderID)
	}
	if route.Model != "text-embedding-3-small" {
		t.Fatalf("model = %s", route.Model)
	}
	if route.BaseURL != "https://embed.local/v1" {
		t.Fatalf("base url = %s", route.BaseURL)
	}
}
