package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestEmailAuthSendRequiresSMTPConfig(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")

	body := bytes.NewBufferString(`{"email":"tester@example.com","purpose":"register"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email/send", body)
	rec := httptest.NewRecorder()

	srv.HandlerFor("account").ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestEmailAuthRegisterUsesEmailCode(t *testing.T) {
	st := store.NewStore()
	st.SaveEmailCode("tester@example.com", "register", "123456", 0)
	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")

	body := bytes.NewBufferString(`{"email":"tester@example.com","email_code":"123456","nickname":"tester"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	rec := httptest.NewRecorder()

	srv.HandlerFor("account").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.AccessToken == "" || payload.User.ID == "" {
		t.Fatalf("unexpected auth payload: %s", rec.Body.String())
	}
	if _, ok := st.FindUserByEmail("tester@example.com"); !ok {
		t.Fatal("expected user to be created with email identity")
	}
}

func TestEmailAuthLoginUsesEmailCode(t *testing.T) {
	st := store.NewStore()
	_, err := st.CreateUser("tester@example.com", "tester")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	st.SaveEmailCode("tester@example.com", "login", "123456", 0)
	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")

	body := bytes.NewBufferString(`{"email":"tester@example.com","email_code":"123456"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	rec := httptest.NewRecorder()

	srv.HandlerFor("account").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
