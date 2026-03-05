package provider

import "nukara/backend/internal/store"

type Route struct {
	ProviderID string
	Model      string
	BaseURL    string
	APIKey     string
}

type ResolverStore interface {
	ListProviders() ([]store.Provider, error)
	GetUserProviderSetting(userID string) (providerID, model string, ok bool)
	GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool)
	GetSystemSetting(key string) (value string, ok bool)
}
