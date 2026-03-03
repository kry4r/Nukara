package analysis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nukara/backend/internal/store"
)

func TestNewServer(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.store != st {
		t.Error("store not set correctly")
	}
}

func TestHealthEndpoint(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}

	// In-memory store should report false for both connections
	if resp["postgres_connected"] != false {
		t.Errorf("expected postgres_connected=false for in-memory store, got %v", resp["postgres_connected"])
	}
	if resp["redis_connected"] != false {
		t.Errorf("expected redis_connected=false for in-memory store, got %v", resp["redis_connected"])
	}
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
