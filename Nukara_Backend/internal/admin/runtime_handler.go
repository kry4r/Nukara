package admin

import (
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleRestartRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"message":      "AgentX runtime is stateless; no restart required",
		"latency_ms":   time.Since(start).Milliseconds(),
		"runtime_mode": "agentx",
	})
}
