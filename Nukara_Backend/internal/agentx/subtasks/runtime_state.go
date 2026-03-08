package subtasks

import (
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type RuntimeStateInput struct {
	Now              time.Time
	ExplicitActivity string
	RecentEvent      string
}

func DeriveRuntimeState(previous store.BotRuntimeState, input RuntimeStateInput) store.BotRuntimeState {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	state := previous
	state.UserID = strings.TrimSpace(previous.UserID)
	state.BotID = strings.TrimSpace(previous.BotID)

	if explicit := strings.TrimSpace(input.ExplicitActivity); explicit != "" {
		state.ActivityText = explicit
		state.UpdatedAt = now
		return state
	}
	if event := strings.TrimSpace(input.RecentEvent); event != "" {
		state.ActivityText = event
		state.UpdatedAt = now
		return state
	}
	if previous.ActivityText != "" && !previous.UpdatedAt.IsZero() && now.Sub(previous.UpdatedAt) <= 2*time.Hour {
		state.UpdatedAt = now
		return state
	}
	if strings.Contains(previous.ActivityText, "上班") && now.Hour() >= 22 {
		state.ActivityText = "刚下晚班，在回去路上"
		state.UpdatedAt = now
		return state
	}
	state.ActivityText = defaultActivityByHour(now.Hour())
	state.UpdatedAt = now
	return state
}

func defaultActivityByHour(hour int) string {
	switch {
	case hour >= 22 || hour < 5:
		return "准备收尾休息，慢慢回到自己的生活里"
	case hour < 12:
		return "刚醒没多久，还在慢慢进入状态"
	case hour < 18:
		return "白天正忙着手头的事"
	default:
		return "在处理傍晚的琐事，节奏慢下来了"
	}
}
