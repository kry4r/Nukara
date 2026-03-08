package store

import "testing"

func TestEmailCodeLifecycle(t *testing.T) {
	s := NewStore()
	s.SaveEmailCode("tester@example.com", "register", "123456", 0)
	if !s.ValidateEmailCode("tester@example.com", "register", "123456") {
		t.Fatal("expected email code to validate")
	}
}

func TestFindUserByEmailReturnsCreatedUser(t *testing.T) {
	s := NewStore()
	created, err := s.CreateUser("tester@example.com", "tester")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, ok := s.FindUserByEmail("tester@example.com")
	if !ok {
		t.Fatal("expected user lookup by email to succeed")
	}
	if got.ID != created.ID {
		t.Fatalf("got user %q, want %q", got.ID, created.ID)
	}
	if got.Email != "tester@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
}
