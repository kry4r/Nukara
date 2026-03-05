package bootstrap

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/llm"
	agentprovider "nukara/backend/internal/agentx/provider"
	"nukara/backend/internal/api"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func NewHandler(role string) (string, http.Handler) {
	var sharedStore store.DataStore
	postgresDSN := envOr("NUKARA_POSTGRES_DSN", "")
	redisAddr := envOr("NUKARA_REDIS_ADDR", envOr("NUKARA_REDIS_ADDRESS", ""))
	if strings.TrimSpace(postgresDSN) != "" {
		persistentStore, err := store.NewPostgresStore(postgresDSN, redisAddr)
		if err != nil {
			log.Printf("⚠️  [bootstrap] WARNING: PostgreSQL 连接失败，使用内存存储（数据重启后丢失）: %v", err)
			sharedStore = store.NewStore()
		} else {
			sharedStore = persistentStore
			log.Printf("[bootstrap] 持久化已启用: postgres + redis(metrics=%t)", strings.TrimSpace(redisAddr) != "")
		}
	} else {
		log.Printf("⚠️  [bootstrap] WARNING: NUKARA_POSTGRES_DSN 未设置，使用内存存储（数据重启后丢失）")
		sharedStore = store.NewStore()
	}

	log.Printf("[bootstrap] agentx runtime enabled")
	apnsClient := apns.NewClient(envOr("NUKARA_APNS_TOPIC", "com.nukara.app"))
	tokenSecret := envOr("NUKARA_JWT_SECRET", "nukara-dev-secret")

	server := api.NewServer(sharedStore, nil, apnsClient, tokenSecret, redisAddr)
	if runtime := buildChatRuntime(sharedStore); runtime != nil {
		server.SetChatRuntime(runtime)
	}
	handler := server.HandlerFor(role)

	if shouldStartScheduler(role) {
		intervalStr := envOr("NUKARA_PROACTIVE_INTERVAL", "5m")
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			interval = 5 * time.Minute
		}
		server.StartScheduler(interval)
		log.Printf("proactive scheduler started, interval=%s, role=%s", interval, role)
	}

	defaultPort := map[string]string{
		"gateway":      "8080",
		"account":      "8001",
		"bot":          "8002",
		"conversation": "8003",
		"proactive":    "8006",
		"analysis":     "8007",
	}[role]

	if defaultPort == "" {
		defaultPort = "8080"
	}

	addr := fmt.Sprintf(":%s", envOr(strings.ToUpper("nukara_"+role+"_port"), defaultPort))
	return addr, handler
}

type routeStore interface {
	ListProviders() ([]store.Provider, error)
	GetUserProviderSetting(userID string) (providerID, model string, ok bool)
	GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool)
	GetSystemSetting(key string) (value string, ok bool)
}

func buildChatRuntime(st store.DataStore) *agentx.Runtime {
	baseURL := strings.TrimSpace(envOr("NUKARA_CHAT_BASE_URL", envOr("NUKARA_ASTRON_BASE_URL", "")))
	apiKey := strings.TrimSpace(envOr("NUKARA_CHAT_API_KEY", envOr("NUKARA_ASTRON_API_KEY", "")))
	model := strings.TrimSpace(envOr("NUKARA_CHAT_MODEL", envOr("NUKARA_ASTRON_CHAT_MODEL", "")))

	deps := agentx.RuntimeDeps{}
	if rs, ok := st.(routeStore); ok {
		deps.RouteResolver = agentprovider.NewRouter(rs)
	}

	if baseURL != "" {
		deps.ProviderClient = llm.NewOpenAICompatClient(baseURL, apiKey, model, nil)
		log.Printf("[bootstrap] chat runtime provider configured: base=%s model=%s", baseURL, model)
	}

	if deps.ProviderClient == nil && deps.RouteResolver == nil {
		return nil
	}
	return agentx.NewRuntime(deps)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func shouldStartScheduler(role string) bool {
	switch strings.TrimSpace(role) {
	case "proactive":
		return true
	case "gateway":
		return strings.EqualFold(strings.TrimSpace(os.Getenv("NUKARA_GATEWAY_ENABLE_SCHEDULER")), "true")
	default:
		return false
	}
}
