package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

type countingEmailSender struct {
	calls int
}

func (s *countingEmailSender) SendVerificationCode(_ context.Context, _ string, _ string, _ time.Duration) error {
	s.calls++
	return nil
}

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

func TestEmailAuthSendSuppressesImmediateDuplicateEmails(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	sender := &countingEmailSender{}
	srv.emailSender = sender

	body := bytes.NewBufferString(`{"email":"tester@example.com","purpose":"register"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email/send", body)
	rec := httptest.NewRecorder()
	srv.HandlerFor("account").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	body2 := bytes.NewBufferString(`{"email":"tester@example.com","purpose":"register"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email/send", body2)
	rec2 := httptest.NewRecorder()
	srv.HandlerFor("account").ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d, body=%s", rec2.Code, http.StatusOK, rec2.Body.String())
	}
	if sender.calls != 1 {
		t.Fatalf("email sender calls = %d, want 1", sender.calls)
	}
}

func TestGatewayTestSessionCreatesUserWithoutSMTP(t *testing.T) {
	st := store.NewStore()
	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")

	body := bytes.NewBufferString(`{"email":"tester@example.com","nickname":"tester"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/test/session", body)
	rec := httptest.NewRecorder()

	srv.HandlerFor("gateway").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		Created     bool   `json:"created"`
		User        struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.AccessToken == "" || payload.User.ID == "" {
		t.Fatalf("unexpected auth payload: %s", rec.Body.String())
	}
	if !payload.Created {
		t.Fatalf("expected created=true payload: %s", rec.Body.String())
	}
	if payload.User.Nickname != "tester" {
		t.Fatalf("nickname = %q, want tester", payload.User.Nickname)
	}
	if _, ok := st.FindUserByEmail("tester@example.com"); !ok {
		t.Fatal("expected user to be created with email identity")
	}
}
