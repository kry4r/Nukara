package neo4jadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeStore struct {
	pingErr error

	expandTopics []string
	expandLimit  int
	expandOut    []Topic
	expandErr    error

	upsertMemoryID string
	upsertTopics   []string
	upsertErr      error
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

func (f *fakeStore) ExpandTopics(_ context.Context, topics []string, limit int) ([]Topic, error) {
	f.expandTopics = append([]string(nil), topics...)
	f.expandLimit = limit
	return append([]Topic(nil), f.expandOut...), f.expandErr
}

func (f *fakeStore) UpsertMemoryTopics(_ context.Context, memoryID string, topics []string) error {
	f.upsertMemoryID = memoryID
	f.upsertTopics = append([]string(nil), topics...)
	return f.upsertErr
}

func TestHealthEndpointReturnsOK(t *testing.T) {
	store := &fakeStore{}
	srv := NewServer(store, "", "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("status payload = %v", payload["status"])
	}
}

func TestExpandTopicsRequiresAuthWhenConfigured(t *testing.T) {
	store := &fakeStore{}
	srv := NewServer(store, "neo4j", "secret")

	req := httptest.NewRequest(http.MethodPost, "/query/topics", strings.NewReader(`{"topics":["jazz"]}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestExpandTopicsNormalizesRequest(t *testing.T) {
	store := &fakeStore{expandOut: []Topic{{Name: "blues", Weight: 2.5}}}
	srv := NewServer(store, "neo4j", "secret")

	req := httptest.NewRequest(http.MethodPost, "/query/topics", strings.NewReader(`{"topics":[" jazz ","","rain","jazz"],"limit":3}`))
	req.SetBasicAuth("neo4j", "secret")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if strings.Join(store.expandTopics, ",") != "jazz,rain" {
		t.Fatalf("topics = %v, want [jazz rain]", store.expandTopics)
	}
	if store.expandLimit != 3 {
		t.Fatalf("limit = %d, want 3", store.expandLimit)
	}

	var payload struct {
		Topics []Topic `json:"topics"`
	}
	if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Topics) != 1 || payload.Topics[0].Name != "blues" {
		t.Fatalf("response topics = %+v", payload.Topics)
	}
}

func TestUpsertMemoryTopicsRequiresMemoryID(t *testing.T) {
	store := &fakeStore{}
	srv := NewServer(store, "", "")

	req := httptest.NewRequest(http.MethodPost, "/query/upsert-memory-topics", strings.NewReader(`{"topics":["jazz"]}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpsertMemoryTopicsNormalizesTopics(t *testing.T) {
	store := &fakeStore{}
	srv := NewServer(store, "", "")

	req := httptest.NewRequest(http.MethodPost, "/query/upsert-memory-topics", strings.NewReader(`{"memory_id":"mem-1","topics":[" jazz ","","rain","jazz"]}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}
	if store.upsertMemoryID != "mem-1" {
		t.Fatalf("memoryID = %q, want mem-1", store.upsertMemoryID)
	}
	if strings.Join(store.upsertTopics, ",") != "jazz,rain" {
		t.Fatalf("topics = %v, want [jazz rain]", store.upsertTopics)
	}
}
