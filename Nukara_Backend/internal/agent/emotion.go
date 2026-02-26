package agent

import (
	"regexp"
	"strings"
)

var emotionRe = regexp.MustCompile(`\s*\[emotion:(\w+)\]\s*$`)
var statusRe = regexp.MustCompile(`\s*\[status:(.+?),(.+?)\]\s*$`)

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
