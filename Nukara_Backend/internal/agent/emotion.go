package agent

import (
	"regexp"
	"strings"
)

var emotionRe = regexp.MustCompile(`\s*\[emotion:(\w+)\]\s*$`)
var statusRe = regexp.MustCompile(`\s*\[status:(.+?),(.+?)\]\s*$`)

// toolCallRe matches leaked LLM tool call XML tags (e.g. <minimax:tool_call>...</minimax:tool_call>).
var toolCallRe = regexp.MustCompile(`(?s)<\w+:tool_call>.*?</\w+:tool_call>`)

// thinkTagRe matches leaked CoT tags (e.g. <think>...</think>).
var thinkTagRe = regexp.MustCompile(`(?is)<think>.*?</think>`)

// reasoningBlockRe matches leaked markdown reasoning headers (e.g. "**Reflection:**", "**Results:**").
// Strips the header line and any immediately following non-empty lines.
var reasoningBlockRe = regexp.MustCompile(`(?m)^\*\*(Reflection|Results|Next Steps|Analysis|Thinking|思考|分析|结果)[:：]\*\*.*$`)

// systemTagRe matches leaked system/internal tags like [system:...], [memory:...], etc.
var systemTagRe = regexp.MustCompile(`\[(?:system|memory|internal|debug):[^\]]*\]`)

// SanitizeLLMReply strips leaked tool call XML, reasoning blocks, and other non-user-facing artifacts from LLM output.
func SanitizeLLMReply(text string) string {
	text = toolCallRe.ReplaceAllString(text, "")
	text = thinkTagRe.ReplaceAllString(text, "")
	text = reasoningBlockRe.ReplaceAllString(text, "")
	text = systemTagRe.ReplaceAllString(text, "")
	// Collapse multiple blank lines left by stripping.
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

// ExtractEmotion strips the trailing [emotion:tag] from the LLM reply
// and returns the cleaned reply text and the emotion tag separately.
func ExtractEmotion(raw string) (reply, emotion string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "neutral"
	}

	loc := emotionRe.FindStringSubmatchIndex(raw)
	if loc == nil {
		return raw, "gentle"
	}

	reply = strings.TrimSpace(raw[:loc[0]])
	emotion = raw[loc[2]:loc[3]]

	if reply == "" {
		return raw, "gentle"
	}
	return reply, emotion
}

// ExtractStatus strips the trailing [status:emoji,text] from the LLM reply.
// Returns cleaned text, emoji, status text. Falls back to emotion-based defaults.
func ExtractStatus(raw, emotion string) (cleaned, emoji, statusText string) {
	raw = strings.TrimSpace(raw)
	loc := statusRe.FindStringSubmatchIndex(raw)
	if loc == nil {
		return raw, EmotionDefaultEmoji(emotion), "聊天中"
	}
	cleaned = strings.TrimSpace(raw[:loc[0]])
	emoji = strings.TrimSpace(raw[loc[2]:loc[3]])
	statusText = strings.TrimSpace(raw[loc[4]:loc[5]])
	if cleaned == "" {
		cleaned = raw
	}
	return cleaned, emoji, statusText
}

func EmotionDefaultEmoji(emotion string) string {
	switch emotion {
	case "happy":
		return "😊"
	case "love":
		return "💕"
	case "sad":
		return "🌧"
	case "excited":
		return "🤩"
	case "angry":
		return "😤"
	case "anxious":
		return "😟"
	default:
		return "☕️"
	}
}
