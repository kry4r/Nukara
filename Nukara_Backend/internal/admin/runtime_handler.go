package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

func (s *Server) handleRestartNanobot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	if err := s.restartNanobot(); err != nil {
		http.Error(w, "Failed to restart nanobot: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.waitNanobotHealthy(60 * time.Second); err != nil {
		http.Error(w, "Nanobot restart timeout: "+err.Error(), http.StatusGatewayTimeout)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"message":        "Nanobot restarted",
		"latency_ms":     time.Since(start).Milliseconds(),
		"nanobot_health": strings.TrimRight(s.nanobotHTTPURL, "/") + "/health",
	})
}

func (s *Server) restartNanobot() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if s.nanobotRestartCmd != "" {
		cmd = exec.CommandContext(ctx, "sh", "-lc", s.nanobotRestartCmd)
	} else {
		if strings.TrimSpace(s.nanobotContainer) == "" {
			return errors.New("NUKARA_NANOBOT_CONTAINER is empty")
		}
		cmd = exec.CommandContext(ctx, "docker", "restart", s.nanobotContainer)
	}
	return cmd.Run()
}

func (s *Server) waitNanobotHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	healthURL := strings.TrimRight(s.nanobotHTTPURL, "/") + "/health"
	client := &http.Client{Timeout: 5 * time.Second}

	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = errors.New(resp.Status)
		} else {
			lastErr = err
		}
		time.Sleep(1 * time.Second)
	}
	if lastErr == nil {
		lastErr = errors.New("health check timeout")
	}
	return lastErr
}
