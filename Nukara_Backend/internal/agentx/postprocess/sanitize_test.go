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

func TestSanitize_RemovesInternalMemoryTags(t *testing.T) {
	in := "你好\n[memory:find_memory_cache]\n[system:debug]\n[internal:trace]\n继续聊"
	got := SanitizeVisible(in)
	if strings.Contains(got, "[memory:") || strings.Contains(got, "[system:") || strings.Contains(got, "[internal:") {
		t.Fatalf("internal tags leaked: %s", got)
	}
	if !strings.Contains(got, "你好") || !strings.Contains(got, "继续聊") {
		t.Fatalf("visible text missing: %s", got)
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

func TestStreamSanitizer_DoesNotLeakMemoryTags(t *testing.T) {
	s := NewStreamSanitizer()
	out1 := s.Push("我先帮你找找 [memo")
	out2 := s.Push("ry:find_memory_cache]")
	out3 := s.Push(" 已处理 [internal:trace]")

	combined := out1 + out2 + out3
	if strings.Contains(combined, "[memory:") || strings.Contains(combined, "[internal:") {
		t.Fatalf("internal stream tags leaked: %q", combined)
	}
	if !strings.Contains(combined, "我先帮你找找") || !strings.Contains(combined, "已处理") {
		t.Fatalf("visible text missing: %q", combined)
	}
}

func TestStreamSanitizer_StripsMessageBoundaryProtocolAcrossChunks(t *testing.T) {
	s := NewStreamSanitizer()
	out1 := s.Push("先去吃饭<<<")
	out2 := s.Push("MSG>>>吃完再和我说")

	combined := out1 + out2
	if strings.Contains(combined, MessageBoundaryToken) {
		t.Fatalf("message boundary leaked: %q", combined)
	}
	if combined != "先去吃饭吃完再和我说" {
		t.Fatalf("combined = %q", combined)
	}
}

func TestSanitizeVisible_StripsMessageBoundaryProtocol(t *testing.T) {
	in := "先去吃饭<<<MSG>>>吃完再和我说"
	got := SanitizeVisible(in)
	if strings.Contains(got, MessageBoundaryToken) {
		t.Fatalf("message boundary leaked: %q", got)
	}
	if got != "先去吃饭吃完再和我说" {
		t.Fatalf("sanitize visible = %q", got)
	}
}
