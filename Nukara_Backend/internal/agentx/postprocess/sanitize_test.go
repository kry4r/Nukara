package postprocess

import (
	"strings"
	"testing"
)

func TestSanitize_RemovesThinkAndTags(t *testing.T) {
	in := "hi<think>secret</think> ok\n[status:💭,在想你]\n"
	got := SanitizeVisible(in)
	if strings.Contains(got, "secret") || strings.Contains(got, "[status:") {
		t.Fatalf("leak detected: %s", got)
	}
	if !strings.Contains(got, "hi") || !strings.Contains(got, "ok") {
		t.Fatalf("unexpected sanitize output: %s", got)
	}
}

func TestStreamSanitizer_DoesNotLeakPartialHiddenTags(t *testing.T) {
	s := NewStreamSanitizer()
	out1 := s.Push("你好<th")
	out2 := s.Push("ink>secret</thi")
	out3 := s.Push("nk> 世界 [status:💭,在想你")
	out4 := s.Push("]")

	combined := out1 + out2 + out3 + out4
	if strings.Contains(combined, "secret") {
		t.Fatalf("think content leaked: %q", combined)
	}
	if strings.Contains(combined, "[status:") {
		t.Fatalf("status tag leaked: %q", combined)
	}
	if !strings.Contains(combined, "你好") || !strings.Contains(combined, "世界") {
		t.Fatalf("visible text missing: %q", combined)
	}
}
