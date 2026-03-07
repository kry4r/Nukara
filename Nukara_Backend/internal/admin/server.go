package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Server struct {
	db              *sql.DB
	router          *http.ServeMux
	chatTestTimeout time.Duration
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
		db:              db,
		router:          http.NewServeMux(),
		chatTestTimeout: envSecondsOrDefault("NUKARA_PROVIDER_CHAT_TIMEOUT_SECONDS", 90),
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Provider 管理
	s.router.HandleFunc("/api/admin/providers", s.authMiddleware(s.handleProviders))
	s.router.HandleFunc("/api/admin/providers/", s.authMiddleware(s.handleProviderByID))
	s.router.HandleFunc("/api/admin/runtime/restart-agent-runtime", s.authMiddleware(s.handleRestartRuntime))
	s.router.HandleFunc("/api/admin/users/provider-settings", s.authMiddleware(s.handleUserProviderSettings))
	s.router.HandleFunc("/api/admin/users/provider-settings/", s.authMiddleware(s.handleUserProviderSettingByUser))
	s.router.HandleFunc("/api/admin/settings/embedding-config", s.authMiddleware(s.handleEmbeddingConfig))

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
