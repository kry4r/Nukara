package api

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/agentx/memorygraph"
	"nukara/backend/internal/store"
)

const (
	runtimeRecentHistoryLimit   = 8
	runtimeMemoryRecallLimit    = 4
	runtimeMemoryCardCharBudget = 180
	runtimeMemoryCardLimit      = 5
)

func (s *Server) newTurnRequest(userID, botID, conversationID, prompt string, userMessageIDs []string, systemContext map[string]any) agentx.TurnRequest {
	return s.newTurnRequestWithProviderConversation(userID, botID, conversationID, conversationID, prompt, userMessageIDs, systemContext)
}

func (s *Server) newTurnRequestWithProviderConversation(userID, botID, conversationID, providerConversationID, prompt string, userMessageIDs []string, systemContext map[string]any) agentx.TurnRequest {
	enrichedContext := enrichLocaleSystemContext(systemContext, time.Now().UTC())
	localConversationID := strings.TrimSpace(conversationID)
	systemPrompt, history := s.buildRuntimeContext(userID, botID, localConversationID, prompt, userMessageIDs, enrichedContext)
	providerConversationID = strings.TrimSpace(providerConversationID)
	if providerConversationID == "" {
		providerConversationID = localConversationID
	}
	return agentx.TurnRequest{
		UserID:                 strings.TrimSpace(userID),
		BotID:                  strings.TrimSpace(botID),
		ConversationID:         localConversationID,
		ProviderConversationID: providerConversationID,
		AggregatedText:         strings.TrimSpace(prompt),
		UserMessageIDs:         append([]string(nil), userMessageIDs...),
		SystemContext:          enrichedContext,
		SystemPrompt:           systemPrompt,
		History:                history,
	}
}

func (s *Server) buildRuntimeContext(userID, botID, conversationID, prompt string, userMessageIDs []string, systemContext map[string]any) (string, []llm.ChatMessage) {
	history := s.buildRecentHistory(userID, conversationID, prompt, userMessageIDs, runtimeRecentHistoryLimit)
	recentTexts := historyContents(history)
	compactText := ""
	if compact, ok := s.store.GetConversationCompact(conversationID); ok {
		compactText = formatCompactContext(compact.CompactJSON)
	}
	runtimeStateText := ""
	if runtimeState, ok := s.store.GetBotRuntimeState(userID, botID); ok {
		runtimeStateText = strings.TrimSpace(runtimeState.ActivityText)
	}
	relationshipText := s.buildRelationshipContext(userID, botID, prompt)
	activeItems := s.store.ListMemoryItems(userID, botID, 24)
	promiseText := formatMemoryContext(selectRelevantPromises(activeItems, prompt, 3))
	memoryCardsText := s.selectRuntimeMemoryCards(userID, botID, conversationID, prompt, recentTexts)
	memoryText := ""
	if strings.TrimSpace(memoryCardsText) == "" {
		memoryText = formatMemoryContext(s.selectRuntimeMemories(userID, botID, prompt))
	}
	return formatSystemPrompt(systemContext, compactText, runtimeStateText, relationshipText, promiseText, memoryCardsText, memoryText), history
}

func (s *Server) selectRuntimeMemories(userID, botID, prompt string) []store.MemoryItem {
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

func formatSystemPrompt(systemContext map[string]any, compactText, runtimeStateText, relationshipText, promiseText, memoryCardsText, memoryText string) string {
	sections := make([]string, 0, 9)
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
	appendLine("【关系上下文】", relationshipText)
	appendLine("【当前状态】", runtimeStateText)
	appendLine("【用户要求】", stringifySystemContextValue(systemContext["user_directives"]))
	appendLine("【用户状态】", stringifySystemContextValue(systemContext["user_status"]))
	appendLine("【本地时间】", formatLocaleContext(systemContext))
	appendLine("【状态标签规则】", stringifySystemContextValue(systemContext["status_instruction"]))
	appendLine("【工具策略】", stringifySystemContextValue(systemContext["tool_policy"]))
	appendLine("【聊天风格规则】", stringifySystemContextValue(systemContext["chat_style_skill"]))
	appendLine("【进行中约定】", promiseText)
	appendLine("【阶段摘要】", compactText)
	appendLine("【记忆卡片】", memoryCardsText)
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

func (s *Server) buildRelationshipContext(userID, botID, prompt string) string {
	nodes := s.store.ListMemoryNodes(userID, botID, store.TemporalMemoryNodeFilter{Status: "active", Limit: 24})
	if len(nodes) == 0 {
		return ""
	}
	prompt = strings.TrimSpace(prompt)
	hasParents := false
	hasPetXiaomi := false
	for _, node := range nodes {
		text := strings.TrimSpace(node.Title + " " + node.Summary)
		if strings.Contains(text, "爸妈") || strings.Contains(text, "父母") {
			hasParents = true
		}
		if strings.Contains(text, "小蜜") && strings.Contains(text, "猫") {
			hasPetXiaomi = true
		}
		for _, entity := range node.Entities {
			name := strings.TrimSpace(entity.Name)
			switch {
			case (name == "爸妈" || name == "父母") && strings.TrimSpace(entity.Type) == "person":
				hasParents = true
			case name == "小蜜" && strings.TrimSpace(entity.Type) == "pet":
				hasPetXiaomi = true
			}
		}
	}
	lines := make([]string, 0, 3)
	if strings.Contains(prompt, "叔叔阿姨") && hasParents {
		lines = append(lines, "- 用户提到的“叔叔阿姨”是指你的父母")
	}
	if strings.Contains(prompt, "小蜜") && hasPetXiaomi {
		lines = append(lines, "- 用户提到的“小蜜”是指你养的猫")
	}
	if len(lines) == 0 {
		return ""
	}
	lines = append(lines, "- 回复时请使用第一人称视角：“我的父母”“我养的猫”")
	return strings.Join(lines, "\n")
}

func (s *Server) selectRuntimeMemoryCards(userID, botID, conversationID, prompt string, recentTexts []string) string {
	if s.temporalRecall == nil {
		return ""
	}
	result, err := s.temporalRecall.Recall(context.Background(), memorygraph.RecallInput{
		UserID:          strings.TrimSpace(userID),
		BotID:           strings.TrimSpace(botID),
		ConversationID:  strings.TrimSpace(conversationID),
		TurnID:          "runtime",
		QueryText:       strings.TrimSpace(prompt),
		RecentTexts:     append([]string(nil), recentTexts...),
		ActivationLimit: runtimeMemoryRecallLimit + 2,
		MaxDepth:        2,
		CardBudget:      memorygraph.CardBudget{MaxChars: runtimeMemoryCardCharBudget, MaxCards: runtimeMemoryCardLimit},
		Now:             time.Now().UTC(),
	})
	if err != nil || len(result.Cards) == 0 {
		return ""
	}
	return formatPromptCards(result.Cards)
}

func formatPromptCards(cards []memorygraph.PromptCard) string {
	if len(cards) == 0 {
		return ""
	}
	lines := make([]string, 0, len(cards))
	for _, card := range cards {
		text := strings.TrimSpace(card.Text)
		if text == "" {
			continue
		}
		lines = append(lines, "- "+text)
	}
	return strings.Join(lines, "\n")
}

func historyContents(history []llm.ChatMessage) []string {
	out := make([]string, 0, len(history))
	for _, item := range history {
		text := strings.TrimSpace(item.Content)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

func selectRelevantMemories(items []store.MemoryItem, query string, limit int) []store.MemoryItem {
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = runtimeMemoryRecallLimit
	}
	query = strings.TrimSpace(query)
	hasRecallCue := strings.Contains(query, "记得") || strings.Contains(query, "喜欢") || strings.Contains(query, "讨厌") || strings.Contains(query, "偏好") || strings.Contains(query, "上次") || strings.Contains(query, "刚才") || strings.Contains(query, "说过") || strings.Contains(query, "答应") || strings.Contains(query, "约好")

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

func selectRelevantPromises(items []store.MemoryItem, query string, limit int) []store.MemoryItem {
	if limit <= 0 {
		limit = 3
	}
	promises := make([]store.MemoryItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Kind) != "promise" || strings.TrimSpace(item.Content) == "" {
			continue
		}
		if status := strings.TrimSpace(item.Status); status != "" && !strings.EqualFold(status, "active") {
			continue
		}
		promises = append(promises, item)
	}
	if len(promises) == 0 {
		return nil
	}
	selected := selectRelevantMemories(promises, query, limit)
	if len(selected) > 0 {
		return selected
	}
	if len(promises) > limit {
		promises = promises[:limit]
	}
	return append([]store.MemoryItem(nil), promises...)
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
