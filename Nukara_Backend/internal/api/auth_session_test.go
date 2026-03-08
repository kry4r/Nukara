package api

import (
	"net/http/httptest"
	"testing"

	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestAuthUserIDRejectsDeletedUserTokenFromBearerHeader(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("tester1@example.com", "tester")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	token, err := srv.issueToken(user.ID)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	srv.store = store.NewStore()

	req := httptest.NewRequest("GET", "/api/v1/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	userID, ok := srv.authUserID(rr, req)
	if ok {
		t.Fatalf("authUserID() unexpectedly accepted deleted user token: %s", userID)
	}
	if rr.Code != 401 {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAuthUserIDRejectsDeletedUserTokenFromQueryString(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("tester2@example.com", "tester")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	srv := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	token, err := srv.issueToken(user.ID)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	srv.store = store.NewStore()

	req := httptest.NewRequest("GET", "/ws/chat?token="+token, nil)
	rr := httptest.NewRecorder()

	userID, ok := srv.authUserID(rr, req)
	if ok {
		t.Fatalf("authUserID() unexpectedly accepted deleted websocket token: %s", userID)
	}
	if rr.Code != 401 {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
