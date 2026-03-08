package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/memorygraph"
	"nukara/backend/internal/agentx/subtasks"
	"nukara/backend/internal/store"
)

const subtaskSystemPrompt = "你是系统内部的结构化子任务执行器。严格遵守用户给出的输出格式。不要寒暄，不要角色扮演，不要解释，不要补充格式外文本。"

func (s *Server) runSubtaskPrompt(ctx context.Context, in subtasks.Input, prompt, fallback string) (string, error) {
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
			SystemPrompt:   subtaskSystemPrompt,
			Purpose:        "subtask",
		})
		if err != nil {
			return "", err
		}
		return text, nil
	}
	if s.agent != nil {
		return s.agent.Chat(ctx, agent.NanobotConvID(in.UserID, in.BotID, in.ConversationID), "subtask", prompt, nil)
	}
	return fallback, nil
}

func (s *Server) buildMemoryCandidateContext(userID, botID, userText, botText string) string {
	query := strings.TrimSpace(strings.Join([]string{userText, botText}, "\n"))
	if s.temporalRecall != nil {
		if result, err := s.temporalRecall.Recall(context.Background(), memorygraph.RecallInput{
			UserID:          strings.TrimSpace(userID),
			BotID:           strings.TrimSpace(botID),
			ConversationID:  "",
			TurnID:          "subtask-candidate",
			QueryText:       query,
			RecentTexts:     []string{strings.TrimSpace(userText), strings.TrimSpace(botText)},
			ActivationLimit: 8,
			MaxDepth:        2,
			CardBudget:      memorygraph.CardBudget{MaxChars: 220, MaxCards: 6},
			Now:             time.Now().UTC(),
		}); err == nil && len(result.Activated) > 0 {
			lines := make([]string, 0, len(result.Activated))
			for _, item := range result.Activated {
				content := strings.TrimSpace(item.Node.Summary)
				if content == "" {
					content = strings.TrimSpace(item.Node.Title)
				}
				if content == "" || strings.TrimSpace(item.Node.ID) == "" {
					continue
				}
				lines = append(lines, fmt.Sprintf("- id=%s | type=%s | content=%s", strings.TrimSpace(item.Node.ID), strings.TrimSpace(item.Node.NodeType), content))
				if len(lines) >= 8 {
					break
				}
			}
			if len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
		}
	}

	items := s.store.ListMemoryItems(userID, botID, 24)
	if len(items) == 0 {
		return "（暂无）"
	}
	selected := selectRelevantMemories(items, query, 8)
	if len(selected) == 0 {
		selected = append([]store.MemoryItem(nil), items...)
		if len(selected) > 8 {
			selected = selected[:8]
		}
	}
	lines := make([]string, 0, len(selected))
	for _, item := range selected {
		content := strings.TrimSpace(item.Content)
		if content == "" || strings.TrimSpace(item.ID) == "" {
			continue
		}
		line := fmt.Sprintf("- id=%s | owner=%s | content=%s", strings.TrimSpace(item.ID), strings.TrimSpace(item.Owner), content)
		if len(item.Topics) > 0 {
			line += " | topics=" + strings.Join(item.Topics, ",")
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "（暂无）"
	}
	return strings.Join(lines, "\n")
}
