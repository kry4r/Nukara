package provider

import (
	"errors"
	"sort"
	"strings"

	"nukara/backend/internal/store"
)

var errNoProviderAvailable = errors.New("no provider available")

type Router struct {
	store ResolverStore
}

func NewRouter(st ResolverStore) *Router {
	return &Router{store: st}
}

func (r *Router) ResolveChatRoute(userID, botID string) (Route, error) {
	sorted, err := r.sortedProviders()
	if err != nil {
		return Route{}, err
	}

	defaultProviderID, _ := r.store.GetSystemSetting("default_chat_provider_id")
	defaultModel, _ := r.store.GetSystemSetting("default_chat_model")

	if providerID, model, ok := r.store.GetBotProviderOverride(userID, botID); ok {
		if provider, found := findProvider(sorted, providerID); found {
			return buildRoute(provider, firstNonEmpty(model, defaultModel)), nil
		}
	}

	if providerID, model, ok := r.store.GetUserProviderSetting(userID); ok {
		if provider, found := findProvider(sorted, providerID); found {
			return buildRoute(provider, firstNonEmpty(model, defaultModel)), nil
		}
	}

	if provider, found := findProvider(sorted, defaultProviderID); found {
		return buildRoute(provider, defaultModel), nil
	}

	for _, provider := range sorted {
		if provider.IsActive {
			return buildRoute(provider, defaultModel), nil
		}
	}
	return buildRoute(sorted[0], defaultModel), nil
}

func (r *Router) ResolveEmbeddingRoute() (Route, error) {
	sorted, err := r.sortedProviders()
	if err != nil {
		return Route{}, err
	}

	embeddingBaseURL, _ := r.store.GetSystemSetting("embedding_base_url")
	embeddingAPIKey, _ := r.store.GetSystemSetting("embedding_api_key")
	embeddingProviderID, _ := r.store.GetSystemSetting("embedding_provider_id")
	embeddingModel, _ := r.store.GetSystemSetting("embedding_model")
	defaultProviderID, _ := r.store.GetSystemSetting("default_chat_provider_id")
	defaultModel, _ := r.store.GetSystemSetting("default_chat_model")

	if baseURL := strings.TrimSpace(embeddingBaseURL); baseURL != "" {
		return Route{
			ProviderID: firstNonEmpty(embeddingProviderID, "embedding_custom"),
			Model:      firstNonEmpty(embeddingModel, defaultModel),
			BaseURL:    strings.TrimRight(baseURL, "/"),
			APIKey:     strings.TrimSpace(embeddingAPIKey),
			APIMode:    "chat_completions",
		}, nil
	}

	if provider, found := findProvider(sorted, embeddingProviderID); found {
		return buildRoute(provider, firstNonEmpty(embeddingModel, defaultModel)), nil
	}
	if provider, found := findProvider(sorted, defaultProviderID); found {
		return buildRoute(provider, firstNonEmpty(embeddingModel, defaultModel)), nil
	}
	for _, provider := range sorted {
		if provider.IsActive {
			return buildRoute(provider, firstNonEmpty(embeddingModel, defaultModel)), nil
		}
	}
	return buildRoute(sorted[0], firstNonEmpty(embeddingModel, defaultModel)), nil
}

func (r *Router) sortedProviders() ([]store.Provider, error) {
	if r == nil || r.store == nil {
		return nil, errNoProviderAvailable
	}
	providers, err := r.store.ListProviders()
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, errNoProviderAvailable
	}

	sorted := make([]store.Provider, 0, len(providers))
	for _, provider := range providers {
		if strings.TrimSpace(provider.ID) == "" {
			continue
		}
		sorted = append(sorted, provider)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Priority == sorted[j].Priority {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Priority < sorted[j].Priority
	})
	if len(sorted) == 0 {
		return nil, errNoProviderAvailable
	}
	return sorted, nil
}

func buildRoute(provider store.Provider, candidateModel string) Route {
	model := strings.TrimSpace(candidateModel)
	if model == "" && len(provider.Models) > 0 {
		model = strings.TrimSpace(provider.Models[0])
	}
	return Route{
		ProviderID: provider.ID,
		Model:      model,
		BaseURL:    provider.BaseURL,
		APIKey:     provider.APIKey,
		APIMode:    normalizeRouteAPIMode(provider.APIMode),
	}
}

func normalizeRouteAPIMode(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "responses", "auto":
		return value
	default:
		return "chat_completions"
	}
}

func findProvider(providers []store.Provider, id string) (store.Provider, bool) {
	target := strings.TrimSpace(id)
	if target == "" {
		return store.Provider{}, false
	}
	for _, provider := range providers {
		if provider.ID == target {
			return provider, true
		}
	}
	return store.Provider{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
