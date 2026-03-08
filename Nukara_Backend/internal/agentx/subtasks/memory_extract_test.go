package subtasks

import "testing"

func TestParseMemoryItemsFiltersLowValueUserFacts(t *testing.T) {
	raw := `{"items":[
		{"kind":"user_fact","owner":"user","content":"你好呀"},
		{"kind":"user_fact","owner":"user","content":"我下周要考试，别半夜给我发消息"}
	]}`

	items, err := ParseMemoryItems(raw)
	if err != nil {
		t.Fatalf("ParseMemoryItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Kind != "user_fact" {
		t.Fatalf("kind = %q", items[0].Kind)
	}
	if items[0].Content != "我下周要考试，别半夜给我发消息" {
		t.Fatalf("content = %q", items[0].Content)
	}
}

func TestParseMemoryItemsNormalizesPromiseStatus(t *testing.T) {
	raw := `{"items":[{"kind":"promise","owner":"bot","content":"我明晚告诉你结果"}]}`

	items, err := ParseMemoryItems(raw)
	if err != nil {
		t.Fatalf("ParseMemoryItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Status != "active" {
		t.Fatalf("status = %q, want active", items[0].Status)
	}
}

func TestParseMemoryItemsKeepsUsefulSelfFacts(t *testing.T) {
	raw := `{"items":[
		{"kind":"self_fact","owner":"bot","content":"我最近总是凌晨两点才睡"},
		{"kind":"self_fact","owner":"bot","content":"我有点困"}
	]}`

	items, err := ParseMemoryItems(raw)
	if err != nil {
		t.Fatalf("ParseMemoryItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Content != "我最近总是凌晨两点才睡" {
		t.Fatalf("content = %q", items[0].Content)
	}
}

func TestParseMemoryItemsKeepsExplicitHighRiskUserFactsForLaterRiskRouting(t *testing.T) {
	raw := `{"items":[{"kind":"user_fact","owner":"user","content":"我喜欢被束缚"}]}`

	items, err := ParseMemoryItems(raw)
	if err != nil {
		t.Fatalf("ParseMemoryItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Kind != "user_fact" {
		t.Fatalf("kind = %q", items[0].Kind)
	}
}

func TestParseMemoryItemsKeepsLowRiskHabitLikeUserFacts(t *testing.T) {
	raw := `{"items":[{"kind":"user_fact","owner":"user","content":"我最近开始把散步当作下班后的固定习惯"}]}`

	items, err := ParseMemoryItems(raw)
	if err != nil {
		t.Fatalf("ParseMemoryItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Content != "我最近开始把散步当作下班后的固定习惯" {
		t.Fatalf("content = %q", items[0].Content)
	}
}
