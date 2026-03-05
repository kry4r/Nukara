package postprocess

import "strings"

func SplitSegments(text string, maxRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxRunes <= 0 {
		maxRunes = 180
	}

	sentences := splitBySentence(text)
	if len(sentences) > 1 {
		segments := make([]string, 0, len(sentences))
		for _, sentence := range sentences {
			runes := []rune(sentence)
			if len(runes) <= maxRunes {
				segments = append(segments, strings.TrimSpace(sentence))
				continue
			}
			segments = append(segments, splitLongByMaxRunes(sentence, maxRunes)...)
		}
		return compactSegments(segments)
	}

	return compactSegments(splitLongByMaxRunes(text, maxRunes))
}

func splitBySentence(text string) []string {
	var out []string
	var buf strings.Builder
	for _, r := range text {
		buf.WriteRune(r)
		switch r {
		case '。', '！', '？', '.', '!', '?', '\n':
			segment := strings.TrimSpace(buf.String())
			if segment != "" {
				out = append(out, segment)
			}
			buf.Reset()
		}
	}
	if tail := strings.TrimSpace(buf.String()); tail != "" {
		out = append(out, tail)
	}
	return out
}

func splitLongByMaxRunes(text string, maxRunes int) []string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}

	var segments []string
	start := 0
	for start < len(runes) {
		end := start + maxRunes
		if end >= len(runes) {
			segments = append(segments, strings.TrimSpace(string(runes[start:])))
			break
		}

		cut := end
		for i := end; i > start+maxRunes/2; i-- {
			switch runes[i-1] {
			case '。', '！', '？', '.', '!', '?', '\n':
				cut = i
				goto found
			}
		}
	found:
		segments = append(segments, strings.TrimSpace(string(runes[start:cut])))
		start = cut
	}
	return segments
}

func compactSegments(segments []string) []string {
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(segment))
	}
	return out
}

