package postprocess

import (
	"regexp"
	"strings"
)

var (
	thinkBlockRe = regexp.MustCompile(`(?is)<think>.*?</think>`)
	hiddenTagRe  = regexp.MustCompile(`\[(status|emotion|system|memory|internal|debug):[^\]]*\]`)
)

var startMarkers = []string{
	"<think>",
	"[status:",
	"[emotion:",
	"[system:",
	"[memory:",
	"[internal:",
	"[debug:",
}

type StreamSanitizer struct {
	inThink bool
	carry   string
}

func NewStreamSanitizer() *StreamSanitizer {
	return &StreamSanitizer{}
}

func (s *StreamSanitizer) Push(delta string) string {
	if s == nil || delta == "" {
		return ""
	}
	input := s.carry + delta
	s.carry = ""

	var out strings.Builder
	i := 0
	for i < len(input) {
		if s.inThink {
			end := strings.Index(input[i:], "</think>")
			if end < 0 {
				keep := len("</think>") - 1
				if len(input[i:]) < keep {
					keep = len(input[i:])
				}
				s.carry = input[len(input)-keep:]
				return out.String()
			}
			i += end + len("</think>")
			s.inThink = false
			continue
		}

		idx, marker := findFirstMarker(input[i:])
		if idx < 0 {
			remain := input[i:]
			keep := longestPrefixSuffix(remain)
			if keep > 0 {
				out.WriteString(remain[:len(remain)-keep])
				s.carry = remain[len(remain)-keep:]
			} else {
				out.WriteString(remain)
			}
			break
		}

		if idx > 0 {
			out.WriteString(input[i : i+idx])
			i += idx
		}

		switch marker {
		case "<think>":
			i += len(marker)
			s.inThink = true
		case "[status:", "[emotion:", "[system:", "[memory:", "[internal:", "[debug:":
			end := strings.IndexByte(input[i+len(marker):], ']')
			if end < 0 {
				s.carry = input[i:]
				return out.String()
			}
			i += len(marker) + end + 1
		default:
			out.WriteByte(input[i])
			i++
		}
	}
	return out.String()
}

func SanitizeVisible(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	withoutThink := thinkBlockRe.ReplaceAllString(text, "")
	withoutTags := hiddenTagRe.ReplaceAllString(withoutThink, "")
	lines := strings.Split(withoutTags, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func findFirstMarker(s string) (int, string) {
	bestIdx := -1
	bestMarker := ""
	for _, marker := range startMarkers {
		idx := strings.Index(s, marker)
		if idx < 0 {
			continue
		}
		if bestIdx == -1 || idx < bestIdx {
			bestIdx = idx
			bestMarker = marker
		}
	}
	return bestIdx, bestMarker
}

func longestPrefixSuffix(s string) int {
	maxLen := 0
	for _, marker := range startMarkers {
		limit := len(marker) - 1
		if limit < 1 {
			continue
		}
		if len(s) < limit {
			limit = len(s)
		}
		for n := limit; n >= 1; n-- {
			if strings.HasSuffix(s, marker[:n]) && n > maxLen {
				maxLen = n
				break
			}
		}
	}
	return maxLen
}
