package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type Server struct {
	db                *sql.DB
	router            *http.ServeMux
	configMu          sync.Mutex
	nanobotHTTPURL    string
	nanobotToken      string
	nanobotConfigPath string
	nanobotStatePath  string
	nanobotContainer  string
	nanobotRestartCmd string
	chatTestTimeout   time.Duration
}

func NewServer() *Server {
	var db *sql.DB
	if dsn := os.Getenv("NUKARA_POSTGRES_DSN"); dsn != "" {
		var err error
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to database: %v", err))
		}
	}

	s := &Server{
		db:                db,
		router:            http.NewServeMux(),
		nanobotHTTPURL:    envOrDefault("NUKARA_NANOBOT_HTTP_URL", "http://127.0.0.1:8081"),
		nanobotToken:      os.Getenv("NUKARA_NANOBOT_TOKEN"),
		nanobotConfigPath: envOrDefault("NUKARA_NANOBOT_CONFIG_PATH", "/root/.nanobot/config.json"),
		nanobotContainer:  envOrDefault("NUKARA_NANOBOT_CONTAINER", "configs-nanobot-1"),
		nanobotRestartCmd: strings.TrimSpace(os.Getenv("NUKARA_NANOBOT_RESTART_COMMAND")),
		chatTestTimeout:   envSecondsOrDefault("NUKARA_PROVIDER_CHAT_TIMEOUT_SECONDS", 90),
	}
	s.nanobotStatePath = envOrDefault("NUKARA_NANOBOT_STATE_PATH", defaultNanobotStatePath(s.nanobotConfigPath))

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Provider 管理
	s.router.HandleFunc("/api/admin/providers", s.authMiddleware(s.handleProviders))
	s.router.HandleFunc("/api/admin/providers/", s.authMiddleware(s.handleProviderByID))
	s.router.HandleFunc("/api/admin/runtime/restart-nanobot", s.authMiddleware(s.handleRestartNanobot))

	// 主动消息配置
	s.router.HandleFunc("/api/admin/proactive/config", s.authMiddleware(s.handleProactiveConfig))
	s.router.HandleFunc("/api/admin/proactive/trigger", s.authMiddleware(s.handleProactiveTrigger))
	s.router.HandleFunc("/api/admin/proactive/logs", s.authMiddleware(s.handleProactiveLogs))

	// 健康检查
	s.router.HandleFunc("/health", s.handleHealth)
}

func (s *Server) Start(port string) error {
	return http.ListenAndServe(":"+port, s.router)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envSecondsOrDefault(key string, fallback int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return time.Duration(fallback) * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return time.Duration(fallback) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func defaultNanobotStatePath(configPath string) string {
	ext := filepath.Ext(configPath)
	if ext == "" {
		return configPath + ".admin-state.json"
	}
	return strings.TrimSuffix(configPath, ext) + ".admin-state.json"
}

// Placeholder handlers for proactive endpoints (will be implemented in Phase 3)
func (s *Server) handleProactiveConfig(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"message":"proactive config endpoint"}`))
}

func (s *Server) handleProactiveTrigger(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"message":"proactive trigger endpoint"}`))
}

func (s *Server) handleProactiveLogs(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"message":"proactive logs endpoint"}`))
}
