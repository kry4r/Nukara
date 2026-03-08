package api

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/store"
)

const emotionBufferThreshold = 5

// emotionAnalysisResult is the expected JSON structure from the LLM.
type emotionAnalysisResult struct {
	Emotions []string `json:"emotions"`
	Trend    string   `json:"trend"`
	Tone     string   `json:"tone"`
}

// bufferAndAnalyzeEmotion appends a user message to the emotion buffer.
// When the buffer reaches the threshold, it triggers async LLM batch analysis.
func (s *Server) bufferAndAnalyzeEmotion(userID string, bot store.Bot, conv store.Conversation, userText string) {
	if strings.TrimSpace(userText) == "" {
		return
	}

	count := s.store.AppendEmotionBuffer(userID, bot.ID, userText)
	if count < emotionBufferThreshold {
		return
	}

	// Threshold reached — grab buffer and clear
	messages := s.store.GetEmotionBuffer(userID, bot.ID)
	s.store.ClearEmotionBuffer(userID, bot.ID)

	go s.runEmotionAnalysis(userID, bot, conv, messages)
}

// runEmotionAnalysis sends buffered messages to the LLM for batch emotion analysis.
func (s *Server) runEmotionAnalysis(userID string, bot store.Bot, conv store.Conversation, messages []string) {
	if s.runtime == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	prompt := buildEmotionPrompt(messages)
	providerConversationID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	sysCtx := agent.BuildSystemContext(bot, nil)
	reply, _, _, _, err := s.runRuntimeChatTextWithProviderConversation(ctx, userID, bot.ID, conv.ID, providerConversationID, prompt, sysCtx)
	if err != nil {
		log.Printf("[emotion] runtime analysis failed: user=%s bot=%s err=%v", userID, bot.ID, err)
		return
	}

	result := parseEmotionResult(reply)
	s.store.SaveEmotionContext(userID, bot.ID, store.EmotionContext{
		RecentEmotions: result.Emotions,
		EmotionTrend:   result.Trend,
		LastTone:       result.Tone,
	})
	log.Printf("[emotion] analysis complete: user=%s bot=%s trend=%s tone=%s", userID, bot.ID, result.Trend, result.Tone)
}

func buildEmotionPrompt(messages []string) string {
	var sb strings.Builder
	sb.WriteString("[system:emotion_analysis]\n")
	sb.WriteString("分析以下用户消息的情感状态，返回JSON格式：\n\n")
	for i, m := range messages {
		sb.WriteString(strconv.Itoa(i+1) + ". " + m + "\n")
	}
	sb.WriteString("\n请返回如下JSON（不要其他内容）：\n")
	sb.WriteString(`{"emotions":["每条消息的情感标签"],"trend":"positive/negative/neutral","tone":"整体语气描述"}`)
	return sb.String()
}

func parseEmotionResult(reply string) emotionAnalysisResult {
	reply = strings.TrimSpace(reply)

	// Try to extract JSON from markdown code block
	if idx := strings.Index(reply, "```"); idx >= 0 {
		start := strings.Index(reply[idx:], "\n")
		if start >= 0 {
			inner := reply[idx+start+1:]
			if end := strings.Index(inner, "```"); end >= 0 {
				reply = strings.TrimSpace(inner[:end])
			} else {
				// No closing ``` (truncated response) — use everything after opening line
				reply = strings.TrimSpace(inner)
			}
		}
	}

	// Fallback: extract first JSON object by braces
	if len(reply) > 0 && reply[0] != '{' {
		if start := strings.Index(reply, "{"); start >= 0 {
			if end := strings.LastIndex(reply, "}"); end > start {
				reply = reply[start : end+1]
			}
		}
	}

	var result emotionAnalysisResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		log.Printf("[emotion] parse failed, using defaults: %v reply=%s", err, reply[:min(len(reply), 200)])
		return emotionAnalysisResult{Trend: "neutral", Tone: "平静"}
	}

	if result.Trend == "" {
		result.Trend = "neutral"
	}
	if result.Tone == "" {
		result.Tone = "平静"
	}
	return result
}
