package postprocess

import "testing"

func TestSplitSegmentsSplitBySentenceEvenWhenShort(t *testing.T) {
	text := "第一句。第二句！第三句？"
	got := SplitSegments(text, 220)
	if len(got) != 3 {
		t.Fatalf("expected 3 segments, got %d: %#v", len(got), got)
	}
	if got[0] != "第一句。" || got[1] != "第二句！" || got[2] != "第三句？" {
		t.Fatalf("unexpected sentence split: %#v", got)
	}
}

func TestSplitSegmentsSplitLongTextByMaxRunes(t *testing.T) {
	text := "这是一段没有句号但是会很长很长很长很长很长很长很长很长很长很长很长很长很长的文本"
	got := SplitSegments(text, 20)
	if len(got) < 2 {
		t.Fatalf("expected split into multiple segments, got %#v", got)
	}
	for i, segment := range got {
		if len([]rune(segment)) > 20 {
			t.Fatalf("segment %d exceeds max runes: %q", i, segment)
		}
	}
}

