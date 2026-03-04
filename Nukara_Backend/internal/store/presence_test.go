package store

import (
	"testing"
	"time"
)

func TestPresenceOnlineExpires(t *testing.T) {
	st := NewStore()
	st.TouchWSPresence("u1", 50*time.Millisecond)
	if !st.IsUserWSOnline("u1") {
		t.Fatalf("expected online")
	}

	time.Sleep(80 * time.Millisecond)
	if st.IsUserWSOnline("u1") {
		t.Fatalf("expected offline after ttl")
	}
}

func TestLastUserMessageAt(t *testing.T) {
	st := NewStore()
	now := time.Now().UTC().Round(time.Second)
	st.SetLastUserMessageAt("u1", now)

	got, ok := st.GetLastUserMessageAt("u1")
	if !ok || got.IsZero() || !got.Equal(now) {
		t.Fatalf("unexpected last msg at: %v ok=%v", got, ok)
	}
}
