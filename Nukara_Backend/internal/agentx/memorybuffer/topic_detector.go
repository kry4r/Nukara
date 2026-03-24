package memorybuffer

import (
	"context"
	"encoding/json"
	"strings"
)

// TopicDetector determines whether the current turn continues the same
// topic as the previous ones, and may extract new topic keywords.
type TopicDetector interface {
	// Detect reports whether userText+botText continues the ongoing topic
	// described by existingKeywords. If the topic has shifted, topicContinues
	// is false and newKeywords holds the keywords for the new topic.
	Detect(ctx context.Context, existingKeywords []string, userText, botText string) (topicContinues bool, newKeywords []string, err error)
}

// LLMTopicDetector uses an LLM to detect topic shifts.
type LLMTopicDetector struct {
	callLLM func(ctx context.Context, prompt string) (string, error)
}

// NewLLMTopicDetector creates a TopicDetector backed by an LLM call.
// callLLM is a function that sends the prompt and returns the LLM's raw reply.
func NewLLMTopicDetector(callLLM func(ctx context.Context, prompt string) (string, error)) *LLMTopicDetector {
	return &LLMTopicDetector{callLLM: callLLM}
}

// Detect implements TopicDetector using a small LLM request.
func (d *LLMTopicDetector) Detect(
	ctx context.Context,
	existingKeywords []string,
	userText, botText string,
) (bool, []string, error) {
	if d == nil || d.callLLM == nil {
		return true, nil, nil
	}
	prompt := buildTopicDetectPrompt(existingKeywords, userText, botText)
	raw, err := d.callLLM(ctx, prompt)
	if err != nil {
		// On error, assume topic continues to avoid excessive LLM calls.
		return true, nil, err
	}
	return parseTopicDetectResponse(raw)
}

func buildTopicDetectPrompt(existingKeywords []string, userText, botText string) string {
	kwStr := strings.Join(existingKeywords, "、")
	if kwStr == "" {
		kwStr = "（暂无）"
	}
	return `[system:topic_detect_json]
判断新一轮对话是否仍在讨论同一话题。

已有话题关键词：` + kwStr + `
新一轮用户：` + userText + `
新一轮机器人：` + botText + `

严格输出JSON：
{"topic_continues":true,"new_topic_keywords":[]}`
}

// topicDetectResponse is the expected JSON shape from the LLM.
type topicDetectResponse struct {
	TopicContinues   bool     `json:"topic_continues"`
	NewTopicKeywords []string `json:"new_topic_keywords"`
}

func parseTopicDetectResponse(raw string) (bool, []string, error) {
	// Find the JSON object bounds.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var parsed topicDetectResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Default: topic continues (conservative).
		return true, nil, nil
	}

	keywords := make([]string, 0, len(parsed.NewTopicKeywords))
	for _, k := range parsed.NewTopicKeywords {
		k = strings.TrimSpace(k)
		if k != "" {
			keywords = append(keywords, k)
		}
	}
	return parsed.TopicContinues, keywords, nil
}

// NoOpTopicDetector always reports that the topic continues.
// Useful as a zero-cost fallback when LLM is unavailable.
type NoOpTopicDetector struct{}

func (NoOpTopicDetector) Detect(_ context.Context, _ []string, _, _ string) (bool, []string, error) {
	return true, nil, nil
}
