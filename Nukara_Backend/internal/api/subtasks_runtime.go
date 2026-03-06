package api

import (
	"context"
	"fmt"
	"strings"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/agentx"
	agentxmemory "nukara/backend/internal/agentx/memory"
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
	selected := make([]store.MemoryItem, 0, 8)
	if s.memoryRecall != nil {
		if items, err := s.memoryRecall.Build(context.Background(), agentxmemory.RecallInput{
			UserID:     strings.TrimSpace(userID),
			BotID:      strings.TrimSpace(botID),
			QueryText:  query,
			Limit:      8,
			WithExpand: false,
		}); err == nil && len(items) > 0 {
			selected = append(selected, items...)
		}
	}
	if len(selected) == 0 {
		items := s.store.ListMemoryItems(userID, botID, 24)
		if len(items) == 0 {
			return "（暂无）"
		}
		selected = selectRelevantMemories(items, query, 8)
		if len(selected) == 0 {
			selected = append([]store.MemoryItem(nil), items...)
			if len(selected) > 8 {
				selected = selected[:8]
			}
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
