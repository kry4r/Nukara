package api

import (
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestInferLocaleContextDefaultsToChina(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 30, 0, 0, time.UTC)
	ctx := InferLocaleContext("", now)

	if ctx.Timezone != "Asia/Shanghai" {
		t.Fatalf("timezone = %q, want %q", ctx.Timezone, "Asia/Shanghai")
	}
	if ctx.Region != "中国" {
		t.Fatalf("region = %q, want %q", ctx.Region, "中国")
	}
	if ctx.LocalTime != "2026-03-07 08:30" {
		t.Fatalf("local_time = %q, want %q", ctx.LocalTime, "2026-03-07 08:30")
	}
	if ctx.DayPhase != "早上" {
		t.Fatalf("day_phase = %q, want %q", ctx.DayPhase, "早上")
	}
}

func TestInferLocaleContextFromLifeContext(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 30, 0, 0, time.UTC)
	tests := []struct {
		name        string
		lifeContext string
		wantTZ      string
		wantRegion  string
		wantTime    string
		wantPhase   string
	}{
		{
			name:        "tokyo keywords",
			lifeContext: "现在住在东京，平时会在日本通勤和拍照",
			wantTZ:      "Asia/Tokyo",
			wantRegion:  "日本",
			wantTime:    "2026-03-07 09:30",
			wantPhase:   "早上",
		},
		{
			name:        "new york keywords",
			lifeContext: "最近搬去纽约，在美国东岸上班",
			wantTZ:      "America/New_York",
			wantRegion:  "美国东部",
			wantTime:    "2026-03-06 19:30",
			wantPhase:   "晚上",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := InferLocaleContext(tt.lifeContext, now)
			if ctx.Timezone != tt.wantTZ {
				t.Fatalf("timezone = %q, want %q", ctx.Timezone, tt.wantTZ)
			}
			if ctx.Region != tt.wantRegion {
				t.Fatalf("region = %q, want %q", ctx.Region, tt.wantRegion)
			}
			if ctx.LocalTime != tt.wantTime {
				t.Fatalf("local_time = %q, want %q", ctx.LocalTime, tt.wantTime)
			}
			if ctx.DayPhase != tt.wantPhase {
				t.Fatalf("day_phase = %q, want %q", ctx.DayPhase, tt.wantPhase)
			}
		})
	}
}

func TestInferLocaleContextInjectedIntoTurnRequest(t *testing.T) {
	st := store.NewStore()
	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	req := server.newTurnRequest(
		"user-1",
		"bot-1",
		"conv-1",
		"今天过得怎么样",
		nil,
		map[string]any{
			"persona":      "温柔朋友",
			"life_context": "现在住在东京，平时摄影、通勤、喝便利店咖啡",
		},
	)

	if req.SystemContext["local_timezone"] != "Asia/Tokyo" {
		t.Fatalf("local_timezone = %v", req.SystemContext["local_timezone"])
	}
	if strings.TrimSpace(stringifySystemContextValue(req.SystemContext["local_time"])) == "" {
		t.Fatalf("expected local_time injected into system context")
	}
	if strings.TrimSpace(stringifySystemContextValue(req.SystemContext["day_phase"])) == "" {
		t.Fatalf("expected day_phase injected into system context")
	}
	if !strings.Contains(req.SystemPrompt, "【本地时间】") {
		t.Fatalf("system prompt missing locale section: %s", req.SystemPrompt)
	}
	if !strings.Contains(req.SystemPrompt, "Asia/Tokyo") {
		t.Fatalf("system prompt missing timezone: %s", req.SystemPrompt)
	}
}
