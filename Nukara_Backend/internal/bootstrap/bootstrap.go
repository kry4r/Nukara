package bootstrap

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"nukara/backend/internal/agent"
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

	nanobotHTTP := envOr("NUKARA_NANOBOT_HTTP_URL", "http://localhost:9090")
	nanobotWS := envOr("NUKARA_NANOBOT_WS_URL", "ws://localhost:9090/ws/chat")
	nanobotToken := envOr("NUKARA_NANOBOT_TOKEN", "")
	agentClient := agent.NewAgent(agent.Config{
		NanobotHTTPURL: nanobotHTTP,
		NanobotWSURL:   nanobotWS,
		NanobotToken:   nanobotToken,
	})
	if err := agentClient.Connect(); err != nil {
		log.Printf("⚠️  [bootstrap] nanobot WS 连接失败: %v", err)
	}
	apnsClient := apns.NewClient(envOr("NUKARA_APNS_TOPIC", "com.nukara.app"))
	tokenSecret := envOr("NUKARA_JWT_SECRET", "nukara-dev-secret")

	log.Printf("[bootstrap] 服务配置: nanobot_http=%s nanobot_ws=%s", nanobotHTTP, nanobotWS)

	server := api.NewServer(sharedStore, agentClient, apnsClient, tokenSecret, redisAddr)
	handler := server.HandlerFor(role)

	if role == "proactive" {
		intervalStr := envOr("NUKARA_PROACTIVE_INTERVAL", "5m")
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			interval = 5 * time.Minute
		}
		server.StartScheduler(interval)
		log.Printf("proactive scheduler started, interval=%s", interval)
	}

	defaultPort := map[string]string{
		"gateway":      "8080",
		"account":      "8001",
		"bot":          "8002",
		"conversation": "8003",
		"proactive":    "8006",
	}[role]

	if defaultPort == "" {
		defaultPort = "8080"
	}

	addr := fmt.Sprintf(":%s", envOr(strings.ToUpper("nukara_"+role+"_port"), defaultPort))
	return addr, handler
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
