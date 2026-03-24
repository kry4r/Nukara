package memorybuffer

import (
	"testing"
	"time"
)

func TestBuffer_AccumulatesTurns(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{MaxTurns: 3, MaxAge: 10 * time.Minute, MaxChars: 2000})
	svc.now = func() time.Time { return now }

	// First turn — no flush.
	r1 := svc.Append("conv-1", "turn-1", "我喜欢散步", "很好的习惯", true, []string{"散步"})
	if r1.Flushed != nil {
		t.Fatal("expected no flush on first turn")
	}
	if r1.BufferedCount != 1 {
		t.Fatalf("expected 1 buffered, got %d", r1.BufferedCount)
	}

	// Second turn — still same topic.
	r2 := svc.Append("conv-1", "turn-2", "每天晚上都走", "坚持下去吧", true, nil)
	if r2.Flushed != nil {
		t.Fatal("expected no flush on second turn")
	}
	if r2.BufferedCount != 2 {
		t.Fatalf("expected 2 buffered, got %d", r2.BufferedCount)
	}
}

func TestBuffer_FlushOnMaxTurns(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{MaxTurns: 2, MaxAge: 10 * time.Minute, MaxChars: 2000})
	svc.now = func() time.Time { return now }

	svc.Append("conv-2", "turn-1", "用户A", "机器人A", true, nil)
	r := svc.Append("conv-2", "turn-2", "用户B", "机器人B", true, nil)

	// When the 2nd turn is appended and MaxTurns=2 is reached, all turns
	// including turn-2 are flushed together.
	if r.Flushed == nil {
		t.Fatal("expected flush when MaxTurns reached")
	}
	if r.Flushed.Reason != FlushReasonMaxTurns {
		t.Fatalf("expected flush reason MaxTurns, got %s", r.Flushed.Reason)
	}
	if len(r.Flushed.TurnIDs) != 2 {
		t.Fatalf("expected 2 turn IDs in flush, got %d", len(r.Flushed.TurnIDs))
	}
	// Buffer is cleared after flush — next turn starts fresh.
	if r.BufferedCount != 0 {
		t.Fatalf("expected 0 turns remaining in buffer after flush, got %d", r.BufferedCount)
	}
}

func TestBuffer_FlushOnTopicChange(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{MaxTurns: 5, MaxAge: 10 * time.Minute, MaxChars: 2000})
	svc.now = func() time.Time { return now }

	svc.Append("conv-3", "turn-1", "我喜欢摄影", "很有意思", true, []string{"摄影"})
	r := svc.Append("conv-3", "turn-2", "我喜欢散步", "也不错", false, []string{"散步"})

	if r.Flushed == nil {
		t.Fatal("expected flush on topic change")
	}
	if r.Flushed.Reason != FlushReasonTopicChange {
		t.Fatalf("expected reason TopicChange, got %s", r.Flushed.Reason)
	}
	// After flush the new turn is buffered.
	if r.BufferedCount != 1 {
		t.Fatalf("expected 1 turn buffered after topic-change flush, got %d", r.BufferedCount)
	}
}

func TestBuffer_AggregatedTextFormat(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{MaxTurns: 3, MaxAge: 10 * time.Minute, MaxChars: 2000})
	svc.now = func() time.Time { return now }

	svc.Append("conv-4", "turn-1", "用户说A", "机器人说A", true, nil)
	svc.Append("conv-4", "turn-2", "用户说B", "机器人说B", true, nil)
	flushed := svc.ForceFlush("conv-4")
	if flushed == nil {
		t.Fatal("expected flush result")
	}
	want := "用户：用户说A\n机器人：机器人说A\n用户：用户说B\n机器人：机器人说B"
	if flushed.AggregatedText != want {
		t.Fatalf("aggregated text mismatch.\nwant: %q\ngot:  %q", want, flushed.AggregatedText)
	}
}

func TestBuffer_ForceFlushEmpty(t *testing.T) {
	svc := NewService(Options{MaxTurns: 5, MaxAge: 10 * time.Minute})
	result := svc.ForceFlush("nonexistent")
	if result != nil {
		t.Fatal("expected nil for empty/nonexistent buffer")
	}
}

func TestBuffer_FlushStale(t *testing.T) {
	base := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	tick := base
	svc := NewService(Options{MaxTurns: 10, MaxAge: 5 * time.Minute, MaxChars: 2000})
	svc.now = func() time.Time { return tick }

	svc.Append("stale-conv", "turn-1", "hello", "hi", true, nil)

	// Advance time past MaxAge.
	tick = base.Add(6 * time.Minute)

	stale := svc.FlushStale()
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale flush, got %d", len(stale))
	}
	if stale[0].Reason != FlushReasonMaxAge {
		t.Fatalf("expected MaxAge reason, got %s", stale[0].Reason)
	}
}

func TestBuffer_MergeKeywords(t *testing.T) {
	result := mergeKeywords([]string{"散步", "夜晚"}, []string{"夜晚", "跑步"})
	if len(result) != 3 {
		t.Fatalf("expected 3 keywords, got %d: %v", len(result), result)
	}
	seen := map[string]bool{}
	for _, k := range result {
		seen[k] = true
	}
	for _, k := range []string{"散步", "夜晚", "跑步"} {
		if !seen[k] {
			t.Fatalf("keyword %q missing from merged result", k)
		}
	}
}
