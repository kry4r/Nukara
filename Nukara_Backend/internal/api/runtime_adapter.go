package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx"
)

var errRuntimeUnavailable = errors.New("chat runtime unavailable")

func (s *Server) runRuntimeChat(ctx context.Context, req agentx.TurnRequest) (text, emotion, statusEmoji, statusText string, err error) {
	if s.runtime == nil {
		return "", "", "", "", errRuntimeUnavailable
	}

	deltaCh, finalCh, err := s.runtime.StreamTurn(ctx, req)
	if err != nil {
		return "", "", "", "", err
	}
	for range deltaCh {
		// Drain stream chunks for sync call sites.
	}

	finalTurn, ok := <-finalCh
	if !ok || len(finalTurn.Segments) == 0 {
		return "", "", "", "", errors.New("empty runtime response")
	}

	segment := finalTurn.Segments[len(finalTurn.Segments)-1]
	text = strings.TrimSpace(segment.Text)
	if text == "" {
		return "", "", "", "", errors.New("empty runtime segment")
	}

	emotion = strings.TrimSpace(segment.EmotionTag)
	if emotion == "" {
		emotion = "gentle"
	}
	statusEmoji = strings.TrimSpace(segment.StatusEmoji)
	if statusEmoji == "" {
		statusEmoji = agent.EmotionDefaultEmoji(emotion)
	}
	statusText = strings.TrimSpace(segment.StatusText)
	if statusText == "" {
		statusText = "聊天中"
	}
	return text, emotion, statusEmoji, statusText, nil
}

func (s *Server) runRuntimeChatText(ctx context.Context, userID, botID, conversationID, prompt string, systemContext map[string]any) (string, string, string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return s.runRuntimeChat(ctx, agentx.TurnRequest{
		UserID:         strings.TrimSpace(userID),
		BotID:          strings.TrimSpace(botID),
		ConversationID: strings.TrimSpace(conversationID),
		AggregatedText: strings.TrimSpace(prompt),
		UserMessageIDs: nil,
		SystemContext:  systemContext,
	})
}

func (s *Server) runRuntimeProactive(ctx context.Context, userID, botID, conversationID, trigger string, systemContext map[string]any) (string, string, string, string, error) {
	prompt := proactivePrompt(trigger, systemContext)
	return s.runRuntimeChatText(ctx, userID, botID, conversationID, prompt, systemContext)
}

func proactivePrompt(trigger string, systemContext map[string]any) string {
	hints := make([]string, 0, 3)
	if trend, ok := systemContext["emotion_trend"].(string); ok && strings.TrimSpace(trend) != "" {
		hints = append(hints, fmt.Sprintf("用户最近情绪倾向：%s", strings.TrimSpace(trend)))
	}
	if lastMsg, ok := systemContext["last_user_message"].(string); ok && strings.TrimSpace(lastMsg) != "" {
		hints = append(hints, fmt.Sprintf("用户上次消息：%s", strings.TrimSpace(lastMsg)))
	}
	if since, ok := systemContext["time_since_last_user_message"].(string); ok && strings.TrimSpace(since) != "" {
		hints = append(hints, fmt.Sprintf("距上次用户消息：%s", strings.TrimSpace(since)))
	}

	contextHint := ""
	if len(hints) > 0 {
		contextHint = "（上下文：" + strings.Join(hints, "；") + "）"
	}
	safetyRule := "保持尊重边界，禁止任何让用户内疚或施压的表达（例如“你怎么不理我”）。"

	prompts := map[string]string{
		"morning_care":             "现在是早上。" + contextHint + "请用你的角色身份，像真正关心对方的人一样，生成一句自然的早安关怀。可以提到天气、早餐、今天的安排等日常话题。一句话就好，简短自然，不要解释，不要使用工具。" + safetyRule,
		"evening_care":             "现在是晚上。" + contextHint + "请用你的角色身份，生成一句自然的晚间关怀。可以问问对方今天过得怎么样、累不累、有没有吃晚饭等。一句话就好，简短温暖，不要解释，不要使用工具。" + safetyRule,
		"curiosity_after_silence":  "用户刚才聊了一会儿就没回消息了。" + contextHint + "请用你的角色身份，像好奇的朋友一样，自然地问问用户在忙什么。语气轻松随意，一句话就好，不要解释，不要使用工具。" + safetyRule,
		"worry_after_long_silence": "用户已经较长时间没有回复了。" + contextHint + "请用你的角色身份，从关心近况、轻量询问或分享轻松话题中自然选择一种方式开启对话，语气关切但不沉重。一句话就好，不要解释，不要使用工具。" + safetyRule,
		"random_share":             "现在是白天。" + contextHint + "请用你的角色身份，主动分享一件有趣的事、一个想法、或者一个小发现，就像朋友之间随手分享日常一样。一句话就好，自然有趣，不要解释，不要使用工具。" + safetyRule,
		"share_personal_moment":    "请用你的角色身份，像朋友突然想到对方一样，主动聊一个轻松日常话题（例如今天的小瞬间、吃到的东西、路上见闻）。一句话就好，要自然、像在找话题开启聊天，不要解释，不要使用工具。" + safetyRule,
		"share_interesting_fact":   "请用你的角色身份，主动抛出一个轻松有趣、适合聊天延展的小话题（比如冷知识、趣闻、生活观察）。一句话就好，要像在找共同话题，不要解释，不要使用工具。" + safetyRule,
		"share_emotion":            "请用你的角色身份，主动表达一个轻微真实的当下感受，并带一个开放式小尾巴方便对方接话。保持自然，一句话就好，不要解释，不要使用工具。" + safetyRule,
	}

	text := prompts[trigger]
	if text == "" {
		text = "请用你的角色身份，生成一句自然的主动关怀消息。只输出消息本身，不要解释，不要使用工具。" + safetyRule
	}
	return text
}
