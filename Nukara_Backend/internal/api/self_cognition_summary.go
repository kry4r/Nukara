package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/subtasks"
	"nukara/backend/internal/store"
)

const selfCognitionSummarySystemPrompt = "你是系统内部的人设动态总结器。严格遵守输出格式，只返回要求的 JSON。"

func (s *Server) runSelfCognitionSummaryPrompt(ctx context.Context, in subtasks.Input, prompt, fallback string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fallback, nil
	}
	if s.runtime != nil {
		text, _, _, _, err := s.runRuntimeChat(ctx, agentx.TurnRequest{
			UserID:         strings.TrimSpace(in.UserID),
			BotID:          strings.TrimSpace(in.BotID),
			ConversationID: strings.TrimSpace(in.ConversationID),
			AggregatedText: prompt,
			SystemPrompt:   selfCognitionSummarySystemPrompt,
			Purpose:        "self_cognition_summary",
		})
		if err != nil {
			return "", err
		}
		return text, nil
	}
	if s.agent != nil {
		return s.agent.Chat(ctx, agent.NanobotConvID(in.UserID, in.BotID, in.ConversationID), "self_cognition_summary", prompt, nil)
	}
	return fallback, nil
}

func (s *Server) refreshBotSelfCognition(ctx context.Context, in subtasks.Input, bot store.Bot, changes []store.PersonaChangeEvent) (store.Bot, error) {
	changeSummaries := make([]string, 0, len(changes))
	for _, change := range changes {
		if summary := summarizePersonaChangeValue(change.Field, change.ProposedValue); summary != "" {
			changeSummaries = append(changeSummaries, summary)
		}
	}
	prompt := buildSelfCognitionSummaryPrompt(bot.SelfCognition, changeSummaries)
	raw, err := s.runSelfCognitionSummaryPrompt(ctx, in, prompt, `{"self_cognition":""}`)
	if err != nil {
		return bot, err
	}
	summary := extractSelfCognitionSummary(raw)
	if summary == "" {
		summary = fallbackSelfCognitionSummary(bot.SelfCognition, changeSummaries)
	}
	if summary == "" {
		return bot, nil
	}
	updated, ok := s.store.UpdateBot(in.UserID, in.BotID, store.Bot{SelfCognition: []string{summary}})
	if !ok {
		return bot, errors.New("bot not found")
	}
	return updated, nil
}

func extractSelfCognitionSummary(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start >= 0 && end > start {
		cleaned = cleaned[start : end+1]
	}
	var payload struct {
		SelfCognition string `json:"self_cognition"`
	}
	if err := json.Unmarshal([]byte(cleaned), &payload); err == nil {
		return trimSummaryText(payload.SelfCognition, 50)
	}
	return trimSummaryText(agent.SanitizeLLMReply(raw), 50)
}

func fallbackSelfCognitionSummary(current []string, changes []string) string {
	if len(changes) > 0 {
		return trimSummaryText(changes[0], 50)
	}
	if len(current) > 0 {
		return trimSummaryText(strings.Join(current, "；"), 50)
	}
	return ""
}

func summarizePersonaChangeValue(field, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	prefix := map[string]string{
		"identity":               "我更明确了自己的身份：",
		"personality":            "我更意识到自己的性格一面：",
		"expression_style":       "我最近说话会更偏向：",
		"life_context":           "我最近的生活状态更像是：",
		"taboos_and_preferences": "我更清楚自己的边界是：",
	}[strings.TrimSpace(field)]
	if prefix == "" {
		return trimSummaryText(value, 42)
	}
	return trimSummaryText(prefix+value, 42)
}

func trimSummaryText(value string, limit int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}
