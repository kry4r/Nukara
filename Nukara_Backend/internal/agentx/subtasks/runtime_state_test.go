package subtasks

import (
	"testing"
	"time"

	"nukara/backend/internal/store"
)

func TestDeriveRuntimeStateKeepsRecentPlausibleState(t *testing.T) {
	now := time.Date(2026, 3, 8, 20, 0, 0, 0, time.UTC)
	prev := store.BotRuntimeState{
		UserID:       "user-1",
		BotID:        "bot-1",
		ActivityText: "在便利店值晚班，刚忙完一阵",
		UpdatedAt:    now.Add(-20 * time.Minute),
	}

	got := DeriveRuntimeState(prev, RuntimeStateInput{Now: now})
	if got.ActivityText != prev.ActivityText {
		t.Fatalf("activity = %q, want %q", got.ActivityText, prev.ActivityText)
	}
}

func TestDeriveRuntimeStatePrefersExplicitEvent(t *testing.T) {
	now := time.Date(2026, 3, 8, 20, 0, 0, 0, time.UTC)
	prev := store.BotRuntimeState{
		UserID:       "user-1",
		BotID:        "bot-1",
		ActivityText: "在便利店值晚班，刚忙完一阵",
		UpdatedAt:    now.Add(-30 * time.Minute),
	}

	got := DeriveRuntimeState(prev, RuntimeStateInput{
		Now:              now,
		ExplicitActivity: "刚下晚班，在回去路上",
	})
	if got.ActivityText != "刚下晚班，在回去路上" {
		t.Fatalf("activity = %q", got.ActivityText)
	}
}

func TestDeriveRuntimeStateShiftsByLateHour(t *testing.T) {
	now := time.Date(2026, 3, 8, 23, 30, 0, 0, time.UTC)
	prev := store.BotRuntimeState{
		UserID:       "user-1",
		BotID:        "bot-1",
		ActivityText: "在便利店上班中",
		UpdatedAt:    now.Add(-6 * time.Hour),
	}

	got := DeriveRuntimeState(prev, RuntimeStateInput{Now: now})
	if got.ActivityText != "刚下晚班，在回去路上" {
		t.Fatalf("activity = %q, want 刚下晚班，在回去路上", got.ActivityText)
	}
}
