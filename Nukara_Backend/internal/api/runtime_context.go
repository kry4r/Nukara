package api

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"nukara/backend/internal/agentx"
	agentxmemory "nukara/backend/internal/agentx/memory"
	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/store"
)

const (
	runtimeRecentHistoryLimit = 8
	runtimeMemoryRecallLimit  = 4
)

func (s *Server) newTurnRequest(userID, botID, conversationID, prompt string, userMessageIDs []string, systemContext map[string]any) agentx.TurnRequest {
	systemPrompt, history := s.buildRuntimeContext(userID, botID, conversationID, prompt, userMessageIDs, systemContext)
	return agentx.TurnRequest{
		UserID:         strings.TrimSpace(userID),
		BotID:          strings.TrimSpace(botID),
		ConversationID: strings.TrimSpace(conversationID),
		AggregatedText: strings.TrimSpace(prompt),
		UserMessageIDs: append([]string(nil), userMessageIDs...),
		SystemContext:  systemContext,
		SystemPrompt:   systemPrompt,
		History:        history,
	}
}

func (s *Server) buildRuntimeContext(userID, botID, conversationID, prompt string, userMessageIDs []string, systemContext map[string]any) (string, []llm.ChatMessage) {
	history := s.buildRecentHistory(userID, conversationID, prompt, userMessageIDs, runtimeRecentHistoryLimit)
	compactText := ""
	if compact, ok := s.store.GetConversationCompact(conversationID); ok {
		compactText = formatCompactContext(compact.CompactJSON)
	}
	memoryText := formatMemoryContext(s.selectRuntimeMemories(userID, botID, prompt))
	return formatSystemPrompt(systemContext, compactText, memoryText), history
}

func (s *Server) selectRuntimeMemories(userID, botID, prompt string) []store.MemoryItem {
	if s.memoryRecall != nil {
		items, err := s.memoryRecall.Build(context.Background(), agentxmemory.RecallInput{
			UserID:     strings.TrimSpace(userID),
			BotID:      strings.TrimSpace(botID),
			QueryText:  strings.TrimSpace(prompt),
			Limit:      runtimeMemoryRecallLimit,
			WithExpand: true,
		})
		if err == nil && len(items) > 0 {
			return items
		}
	}
	return selectRelevantMemories(s.store.ListMemoryItems(userID, botID, 24), prompt, runtimeMemoryRecallLimit)
}

func (s *Server) buildRecentHistory(userID, conversationID, prompt string, userMessageIDs []string, limit int) []llm.ChatMessage {
	messages, ok := s.store.ListMessages(userID, conversationID, limit+len(userMessageIDs)+2)
	if !ok || len(messages) == 0 {
		return nil
	}
	skip := map[string]struct{}{}
	for _, id := range userMessageIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			skip[id] = struct{}{}
		}
	}
	trimmedPrompt := strings.TrimSpace(prompt)
	out := make([]llm.ChatMessage, 0, limit)
	for _, message := range messages {
		if _, found := skip[message.ID]; found {
			continue
		}
		text := strings.TrimSpace(message.Content.Text)
		if text == "" {
			continue
		}
		if trimmedPrompt != "" && message.SenderType == "user" && text == trimmedPrompt {
			continue
		}
		role := "assistant"
		if strings.EqualFold(strings.TrimSpace(message.SenderType), "user") {
			role = "user"
		}
		out = append(out, llm.ChatMessage{Role: role, Content: text})
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func formatSystemPrompt(systemContext map[string]any, compactText, memoryText string) string {
	sections := make([]string, 0, 6)
	appendLine := func(title, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		sections = append(sections, title+"\n"+value)
	}

	appendLine("【角色设定】", stringifySystemContextValue(systemContext["persona"]))
	appendLine("【说话风格】", stringifySystemContextValue(systemContext["speaking_style"]))
	appendLine("【角色背景】", stringifySystemContextValue(systemContext["background"]))
	appendLine("【角色特质】", stringifySystemContextValue(systemContext["traits"]))
	appendLine("【用户要求】", stringifySystemContextValue(systemContext["user_directives"]))
	appendLine("【用户状态】", stringifySystemContextValue(systemContext["user_status"]))
	appendLine("【状态标签规则】", stringifySystemContextValue(systemContext["status_instruction"]))
	appendLine("【工具策略】", stringifySystemContextValue(systemContext["tool_policy"]))
	appendLine("【聊天风格规则】", stringifySystemContextValue(systemContext["chat_style_skill"]))
	appendLine("【阶段摘要】", compactText)
	appendLine("【相关记忆】", memoryText)

	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func formatCompactContext(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var payload struct {
		Summary string `json:"summary"`
		Facts   []any  `json:"facts"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return raw
	}
	parts := make([]string, 0, 1+len(payload.Facts))
	if summary := strings.TrimSpace(payload.Summary); summary != "" {
		parts = append(parts, summary)
	}
	for _, fact := range payload.Facts {
		switch value := fact.(type) {
		case string:
			value = strings.TrimSpace(value)
			if value != "" {
				parts = append(parts, "- "+value)
			}
		case map[string]any:
			if content, ok := value["content"].(string); ok && strings.TrimSpace(content) != "" {
				parts = append(parts, "- "+strings.TrimSpace(content))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func formatMemoryContext(items []store.MemoryItem) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		line := "- " + content
		if len(item.Topics) > 0 {
			line += "（" + strings.Join(item.Topics, "、") + "）"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func selectRelevantMemories(items []store.MemoryItem, query string, limit int) []store.MemoryItem {
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = runtimeMemoryRecallLimit
	}
	query = strings.TrimSpace(query)
	hasRecallCue := strings.Contains(query, "记得") || strings.Contains(query, "喜欢") || strings.Contains(query, "讨厌") || strings.Contains(query, "偏好") || strings.Contains(query, "上次") || strings.Contains(query, "刚才") || strings.Contains(query, "说过")

	type scored struct {
		item  store.MemoryItem
		score int
		match bool
	}
	scoredItems := make([]scored, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		score := item.Importance / 10
		matched := false
		if query != "" && (strings.Contains(query, content) || strings.Contains(content, query)) {
			score += 12
			matched = true
		}
		for _, topic := range item.Topics {
			topic = strings.TrimSpace(topic)
			if topic == "" {
				continue
			}
			if query != "" && (strings.Contains(query, topic) || strings.Contains(topic, query)) {
				score += 10
				matched = true
			}
		}
		if hasRecallCue {
			score += 2
		}
		scoredItems = append(scoredItems, scored{item: item, score: score, match: matched})
	}
	if len(scoredItems) == 0 {
		return nil
	}
	sort.Slice(scoredItems, func(i, j int) bool {
		if scoredItems[i].score != scoredItems[j].score {
			return scoredItems[i].score > scoredItems[j].score
		}
		return scoredItems[i].item.OccurredAt.After(scoredItems[j].item.OccurredAt)
	})

	out := make([]store.MemoryItem, 0, limit)
	for _, item := range scoredItems {
		if !item.match && !hasRecallCue {
			continue
		}
		out = append(out, item.item)
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 && hasRecallCue {
		for _, item := range scoredItems {
			out = append(out, item.item)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func stringifySystemContextValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return strings.Join(typed, "；")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringifySystemContextValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "；")
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(raw))
	}
}
