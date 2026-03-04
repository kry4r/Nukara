package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/store"
)

const (
	defaultIterateMessageLimit = 30
	maxIterateMessageLimit     = 100
)

type iterateAdds struct {
	SpeakingStyleAdds []string `json:"speaking_style_adds"`
	BackgroundAdds    []string `json:"background_adds"`
	TraitAdds         []string `json:"trait_adds"`
	Gender            string   `json:"gender"`
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

	respondJSON(w, http.StatusOK, map[string]any{
		"bot":             bot,
		"bot_state":       state,
		"directives":      s.store.ListDirectives(userID, botID, "active"),
		"conversation_id": conversationID,
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

	directives := s.store.ListDirectives(userID, botID, "active")
	sysCtx := agent.BuildSystemContext(bot, directives)
	prompt := `[system:user_impression]
请基于你对用户的记忆和最近互动，输出一段“用户印象”摘要。
要求：
1) 仅输出正文，不要JSON，不要markdown。
2) 80字以内，语气自然。
3) 包含对方沟通风格/兴趣或情绪倾向中的1-2点。`

	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	raw, err := s.agent.Chat(context.Background(), convID, "default", prompt, sysCtx)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "impression generation failed"})
		return
	}

	impression := strings.TrimSpace(agent.SanitizeLLMReply(raw))
	if impression == "" {
		impression = "你给我的感觉很真诚，也很值得认真倾听。"
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"impression": impression,
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

	convID := agent.NanobotConvID(userID, bot.ID, conv.ID)
	raw, err := s.agent.Chat(context.Background(), convID, "default", prompt, sysCtx)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "iterate generation failed"})
		return
	}

	adds := parseIterateAdds(raw)

	var genderPtr *string
	if adds.Gender != "" {
		gender := adds.Gender
		genderPtr = &gender
	}
	updated, found := s.store.AppendBotPersona(userID, botID, adds.SpeakingStyleAdds, adds.BackgroundAdds, adds.TraitAdds, genderPtr)
	if !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"speaking_style_adds": adds.SpeakingStyleAdds,
		"background_adds":     adds.BackgroundAdds,
		"trait_adds":          adds.TraitAdds,
		"gender":              adds.Gender,
		"bot":                 updated,
	})
}

func buildIteratePrompt(messages []store.Message) string {
	var sb strings.Builder
	sb.WriteString(`[system:bot_iterate]
你要基于最近对话提出“角色自我迭代建议”。
仅输出JSON，不要解释，不要代码块。JSON格式如下：
{"speaking_style_adds":[],"background_adds":[],"trait_adds":[],"gender":""}
约束：
- 每个数组最多5项，每项20字以内
- 内容必须是可直接追加到人设的短语
- gender 仅可为 female/male/unknown 或空字符串

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
	adds.SpeakingStyleAdds = normalizeAdds(adds.SpeakingStyleAdds, 5, 20)
	adds.BackgroundAdds = normalizeAdds(adds.BackgroundAdds, 5, 20)
	adds.TraitAdds = normalizeAdds(adds.TraitAdds, 5, 20)
	adds.Gender = normalizeGender(adds.Gender)
	return adds
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

func normalizeGender(gender string) string {
	switch strings.ToLower(strings.TrimSpace(gender)) {
	case "female":
		return "female"
	case "male":
		return "male"
	case "unknown":
		return "unknown"
	default:
		return ""
	}
}
