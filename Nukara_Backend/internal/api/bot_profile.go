package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"nukara/backend/internal/agent"
	agentpersona "nukara/backend/internal/agentx/persona"
	"nukara/backend/internal/agentx/subtasks"
	"nukara/backend/internal/store"
)

const (
	defaultIterateMessageLimit = 30
	maxIterateMessageLimit     = 100
)

type iterateAdds struct {
	IdentityAdds             []string `json:"identity_adds"`
	PersonalityAdds          []string `json:"personality_adds"`
	ExpressionStyleAdds      []string `json:"expression_style_adds"`
	LifeContextAdds          []string `json:"life_context_adds"`
	TaboosAndPreferencesAdds []string `json:"taboos_and_preferences_adds"`
}

type runtimeStateView struct {
	ActivityText    string    `json:"activity_text"`
	BasisTags       []string  `json:"basis_tags,omitempty"`
	SourceMemoryIDs []string  `json:"source_memory_ids,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

type profileMemoryView struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Owner      string    `json:"owner,omitempty"`
	Content    string    `json:"content"`
	Importance int       `json:"importance,omitempty"`
	Status     string    `json:"status,omitempty"`
	Topics     []string  `json:"topics,omitempty"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type personaChangeView struct {
	ID            string    `json:"id"`
	Field         string    `json:"field"`
	ChangeType    string    `json:"change_type"`
	ProposedValue string    `json:"proposed_value"`
	SummaryText   string    `json:"summary_text,omitempty"`
	Risk          string    `json:"risk,omitempty"`
	Status        string    `json:"status"`
	ReviewerNote  string    `json:"reviewer_note,omitempty"`
	SourceTurnID  string    `json:"source_turn_id,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

func (s *Server) handleBotProfile(w http.ResponseWriter, r *http.Request, userID, botID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	bot, found := s.store.GetBot(userID, botID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	state, ok := s.store.GetBotState(userID, botID)
	if !ok {
		state = store.BotState{
			UserID:      userID,
			BotID:       botID,
			StatusEmoji: "🙂",
			StatusText:  "在线",
		}
	}

	conversationID := ""
	if conv, found := s.store.FindConversationByBot(userID, botID); found {
		conversationID = conv.ID
	}

	runtimeState, recentImpressions, keyMemories, recentChanges := s.buildRuntimePortrait(userID, botID)

	respondJSON(w, http.StatusOK, map[string]any{
		"bot":                bot,
		"bot_state":          state,
		"conversation_id":    conversationID,
		"runtime_state":      runtimeState,
		"recent_impressions": recentImpressions,
		"key_memories":       keyMemories,
		"recent_changes":     recentChanges,
	})
}

func (s *Server) handleBotImpression(w http.ResponseWriter, r *http.Request, userID, botID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	bot, found := s.store.GetBot(userID, botID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}
	conv, found := s.store.FindConversationByBot(userID, botID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}

	cachedImpressions := selectRecentImpressions(s.store.ListMemoryItems(userID, botID, 12), 1)
	if len(cachedImpressions) > 0 {
		cachedView := newProfileMemoryView(cachedImpressions[0])
		respondJSON(w, http.StatusOK, map[string]any{
			"impression": cachedView.Content,
			"memory":     cachedView,
		})
		return
	}

	directives := s.store.ListDirectives(userID, botID, "active")
	sysCtx := agent.BuildSystemContext(bot, directives)
	prompt := `[system:user_impression]
请基于你对用户的记忆和最近互动，输出一段“用户印象”摘要。
要求：
1) 仅输出正文，不要JSON，不要markdown。
2) 80字以内，语气自然。
3) 包含对方沟通风格/兴趣或情绪倾向中的1-2点。`

	providerConversationID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	raw, _, _, _, err := s.runRuntimeChatTextWithProviderConversation(context.Background(), userID, bot.ID, conv.ID, providerConversationID, prompt, sysCtx)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "impression generation failed"})
		return
	}

	impression := strings.TrimSpace(agent.SanitizeLLMReply(raw))
	if impression == "" {
		impression = "你给我的感觉很真诚，也很值得认真倾听。"
	}

	saved, err := s.store.UpsertMemoryItem(store.MemoryItem{
		UserID:     userID,
		BotID:      botID,
		Kind:       "impression",
		Owner:      "bot",
		Content:    impression,
		Importance: 88,
		OccurredAt: time.Now().UTC(),
		Status:     "active",
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": "impression persistence failed"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"impression": impression,
		"memory":     newProfileMemoryView(saved),
	})
}

func (s *Server) handleBotIterate(w http.ResponseWriter, r *http.Request, userID, botID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		MessageLimit int `json:"message_limit"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil && !errors.Is(err, context.Canceled) {
			badRequest(w, err)
			return
		}
	}
	limit := req.MessageLimit
	if limit <= 0 || limit > maxIterateMessageLimit {
		limit = defaultIterateMessageLimit
	}

	bot, found := s.store.GetBot(userID, botID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}
	conv, found := s.store.FindConversationByBot(userID, botID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}

	messages, ok := s.store.ListMessages(userID, conv.ID, limit)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}

	directives := s.store.ListDirectives(userID, botID, "active")
	sysCtx := agent.BuildSystemContext(bot, directives)
	prompt := buildIteratePrompt(messages)

	providerConversationID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	raw, _, _, _, err := s.runRuntimeChatTextWithProviderConversation(context.Background(), userID, bot.ID, conv.ID, providerConversationID, prompt, sysCtx)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "iterate generation failed"})
		return
	}

	adds := parseIterateAdds(raw)
	patch := store.PersonaPatchInput{
		IdentityAdds:             adds.IdentityAdds,
		PersonalityAdds:          adds.PersonalityAdds,
		ExpressionStyleAdds:      adds.ExpressionStyleAdds,
		LifeContextAdds:          adds.LifeContextAdds,
		TaboosAndPreferencesAdds: adds.TaboosAndPreferencesAdds,
	}
	patch.PersonaPrompt = compilePersonaPrompt(previewBotWithPatch(bot, patch))

	updated, found := s.store.ApplyBotPersonaPatch(userID, botID, patch)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	s.wsHub.publishToUser(userID, map[string]any{
		"type":      "bot_persona_updated",
		"bot_id":    botID,
		"patch":     adds,
		"summary":   "人设已更新",
		"timestamp": updated.UpdatedAt.Unix(),
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"identity_adds":               adds.IdentityAdds,
		"personality_adds":            adds.PersonalityAdds,
		"expression_style_adds":       adds.ExpressionStyleAdds,
		"life_context_adds":           adds.LifeContextAdds,
		"taboos_and_preferences_adds": adds.TaboosAndPreferencesAdds,
		"bot":                         updated,
	})
}

func (s *Server) handleBotPersonaChangeAction(w http.ResponseWriter, r *http.Request, userID, botID, changeID, action string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	change, ok := s.findPersonaChangeEvent(userID, botID, changeID)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "persona change not found"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(change.Status), "pending") {
		respondJSON(w, http.StatusConflict, map[string]any{"error": "persona change already resolved"})
		return
	}

	var req struct {
		ReviewerNote string `json:"reviewer_note"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil && !errors.Is(err, context.Canceled) {
			badRequest(w, err)
			return
		}
	}

	bot, found := s.store.GetBot(userID, botID)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	status := "rejected"
	if action == "accept" {
		status = "accepted"
		patch := buildPersonaPatchFromChange(change)
		patch.PersonaPrompt = compilePersonaPrompt(previewBotWithPatch(bot, patch))
		updatedBot, ok := s.store.ApplyBotPersonaPatch(userID, botID, patch)
		if !ok {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
			return
		}
		bot = updatedBot
	}

	updatedChange, ok := s.store.UpdatePersonaChangeEventStatus(changeID, status, req.ReviewerNote)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "persona change not found"})
		return
	}
	if action == "accept" {
		conversationID := ""
		if conv, found := s.store.FindConversationByBot(userID, botID); found {
			conversationID = conv.ID
		}
		if updatedBot, err := s.refreshBotSelfCognition(r.Context(), subtasks.Input{
			UserID:         userID,
			BotID:          botID,
			ConversationID: conversationID,
			TurnID:         updatedChange.SourceTurnID,
		}, bot, []store.PersonaChangeEvent{updatedChange}); err == nil {
			bot = updatedBot
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"change": newPersonaChangeView(updatedChange),
		"bot":    bot,
	})
}

func buildIteratePrompt(messages []store.Message) string {
	var sb strings.Builder
	sb.WriteString(`[system:bot_iterate]
你要基于最近对话提出“角色自我迭代建议”。
仅输出JSON，不要解释，不要代码块。JSON格式如下：
{"identity_adds":[],"personality_adds":[],"expression_style_adds":[],"life_context_adds":[],"taboos_and_preferences_adds":[]}
约束：
- 每个数组最多5项，每项20字以内
- 内容必须是可直接追加到人设的短语
- 如果某一项没有新增，就返回空数组

最近对话：
`)

	for _, msg := range messages {
		plain := messagePlainText(msg)
		if plain == "" {
			continue
		}
		role := "Bot"
		if msg.SenderType == "user" {
			role = "User"
		}
		sb.WriteString("- " + role + ": " + plain + "\n")
	}
	return sb.String()
}

func messagePlainText(msg store.Message) string {
	text := strings.TrimSpace(msg.Content.Text)
	if text != "" {
		return text
	}
	switch msg.Content.Type {
	case "image":
		return "[图片]"
	case "location":
		if strings.TrimSpace(msg.Content.Name) != "" {
			return "📍" + strings.TrimSpace(msg.Content.Name)
		}
		return "📍位置"
	default:
		return ""
	}
}

func (s *Server) buildRuntimePortrait(userID, botID string) (*runtimeStateView, []profileMemoryView, []profileMemoryView, []personaChangeView) {
	var runtimeState *runtimeStateView
	if state, ok := s.store.GetBotRuntimeState(userID, botID); ok {
		runtimeState = &runtimeStateView{
			ActivityText:    strings.TrimSpace(state.ActivityText),
			BasisTags:       append([]string(nil), state.BasisTags...),
			SourceMemoryIDs: append([]string(nil), state.SourceMemoryIDs...),
			UpdatedAt:       state.UpdatedAt,
		}
	}

	items := s.store.ListMemoryItems(userID, botID, 24)
	recentImpressions := buildProfileMemoryViews(selectRecentImpressions(items, 3))
	keyMemories := buildProfileMemoryViews(selectProfileMemories(items, 6, func(item store.MemoryItem) bool {
		switch strings.TrimSpace(item.Kind) {
		case "promise", "event", "self_fact", "user_fact", "habit":
			return true
		default:
			return false
		}
	}))
	recentChanges := buildPersonaChangeViews(s.store.ListPersonaChangeEvents(userID, botID, "", 20))
	return runtimeState, recentImpressions, keyMemories, recentChanges
}

func (s *Server) cachedBotImpression(userID, botID string) string {
	items := selectRecentImpressions(s.store.ListMemoryItems(userID, botID, 12), 1)
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[0].Content)
}

func (s *Server) findPersonaChangeEvent(userID, botID, changeID string) (store.PersonaChangeEvent, bool) {
	for _, item := range s.store.ListPersonaChangeEvents(userID, botID, "", 100) {
		if item.ID == strings.TrimSpace(changeID) {
			return item, true
		}
	}
	return store.PersonaChangeEvent{}, false
}

func buildProfileMemoryViews(items []store.MemoryItem) []profileMemoryView {
	views := make([]profileMemoryView, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		views = append(views, newProfileMemoryView(item))
	}
	return views
}

func buildPersonaChangeViews(items []store.PersonaChangeEvent) []personaChangeView {
	views := make([]personaChangeView, 0, len(items))
	for _, item := range items {
		views = append(views, newPersonaChangeView(item))
	}
	return views
}

func newPersonaChangeView(item store.PersonaChangeEvent) personaChangeView {
	return personaChangeView{
		ID:            item.ID,
		Field:         strings.TrimSpace(item.Field),
		ChangeType:    strings.TrimSpace(item.ChangeType),
		ProposedValue: strings.TrimSpace(item.ProposedValue),
		SummaryText:   summarizePersonaChangeValue(item.Field, item.ProposedValue),
		Risk:          strings.TrimSpace(item.Risk),
		Status:        strings.TrimSpace(item.Status),
		ReviewerNote:  strings.TrimSpace(item.ReviewerNote),
		SourceTurnID:  strings.TrimSpace(item.SourceTurnID),
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func newProfileMemoryView(item store.MemoryItem) profileMemoryView {
	return profileMemoryView{
		ID:         item.ID,
		Kind:       strings.TrimSpace(item.Kind),
		Owner:      strings.TrimSpace(item.Owner),
		Content:    strings.TrimSpace(item.Content),
		Importance: item.Importance,
		Status:     strings.TrimSpace(item.Status),
		Topics:     append([]string(nil), item.Topics...),
		OccurredAt: item.OccurredAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func selectProfileMemories(items []store.MemoryItem, limit int, keep func(store.MemoryItem) bool) []store.MemoryItem {
	if limit <= 0 {
		limit = len(items)
	}
	selected := make([]store.MemoryItem, 0, limit)
	for _, item := range items {
		if keep != nil && !keep(item) {
			continue
		}
		selected = append(selected, item)
		if len(selected) >= limit {
			break
		}
	}
	return selected
}

func selectRecentImpressions(items []store.MemoryItem, limit int) []store.MemoryItem {
	selected := selectProfileMemories(items, limit, func(item store.MemoryItem) bool {
		return strings.TrimSpace(item.Kind) == "impression" && strings.TrimSpace(item.Content) != ""
	})
	if len(selected) > 0 {
		return selected
	}
	return selectProfileMemories(items, limit, func(item store.MemoryItem) bool {
		return strings.TrimSpace(item.Kind) == "user_fact" && strings.TrimSpace(item.Content) != ""
	})
}

func buildPersonaPatchFromChange(change store.PersonaChangeEvent) store.PersonaPatchInput {
	adds := splitTextValues(change.ProposedValue)
	patch := store.PersonaPatchInput{}
	switch strings.TrimSpace(change.Field) {
	case "identity":
		patch.IdentityAdds = adds
	case "personality":
		patch.PersonalityAdds = adds
	case "expression_style":
		patch.ExpressionStyleAdds = adds
	case "life_context":
		patch.LifeContextAdds = adds
	case "taboos_and_preferences":
		patch.TaboosAndPreferencesAdds = adds
	}
	return patch
}

func previewBotWithPatch(bot store.Bot, patch store.PersonaPatchInput) store.Bot {
	preview := bot
	preview.Identity = appendTextAdds(preview.Identity, patch.IdentityAdds)
	preview.Personality = appendUnique(preview.Personality, patch.PersonalityAdds, 10)
	preview.ExpressionStyle = appendTextAdds(preview.ExpressionStyle, patch.ExpressionStyleAdds)
	preview.LifeContext = appendTextAdds(preview.LifeContext, patch.LifeContextAdds)
	preview.TaboosAndPreferences = appendTextAdds(preview.TaboosAndPreferences, patch.TaboosAndPreferencesAdds)
	return preview
}

func compilePersonaPrompt(bot store.Bot) string {
	return agentpersona.CompilePrompt(bot, 420)
}

func parseIterateAdds(raw string) iterateAdds {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start >= 0 && end > start {
		cleaned = cleaned[start : end+1]
	}

	var adds iterateAdds
	if err := json.Unmarshal([]byte(cleaned), &adds); err != nil {
		return iterateAdds{}
	}
	adds.IdentityAdds = normalizeAdds(adds.IdentityAdds, 5, 40)
	adds.PersonalityAdds = normalizeAdds(adds.PersonalityAdds, 5, 20)
	adds.ExpressionStyleAdds = normalizeAdds(adds.ExpressionStyleAdds, 5, 20)
	adds.LifeContextAdds = normalizeAdds(adds.LifeContextAdds, 5, 40)
	adds.TaboosAndPreferencesAdds = normalizeAdds(adds.TaboosAndPreferencesAdds, 5, 40)
	return adds
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func splitPipe(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func appendUnique(base, adds []string, max int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(base)+len(adds))
	push := func(values []string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
			if max > 0 && len(out) >= max {
				return
			}
		}
	}
	push(base)
	if max <= 0 || len(out) < max {
		push(adds)
	}
	return out
}

func normalizeAdds(values []string, maxCount, maxLen int) []string {
	if maxCount <= 0 || maxLen <= 0 {
		return nil
	}
	out := make([]string, 0, min(maxCount, len(values)))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		runes := []rune(value)
		if len(runes) > maxLen {
			value = string(runes[:maxLen])
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= maxCount {
			break
		}
	}
	return out
}

func appendTextAdds(base string, adds []string) string {
	parts := appendUnique(splitTextValues(base), adds, 10)
	return strings.Join(parts, "；")
}

func splitTextValues(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.FieldsFunc(v, func(r rune) bool {
		switch r {
		case '|', '；', ';', '\n':
			return true
		default:
			return false
		}
	})
	return appendUnique(nil, parts, 0)
}
