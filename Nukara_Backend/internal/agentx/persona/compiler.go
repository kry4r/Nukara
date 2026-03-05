package persona

import (
	"fmt"
	"strings"

	"nukara/backend/internal/store"
)

func CompilePrompt(bot store.Bot, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 400
	}
	parts := []string{
		fmt.Sprintf("你是%s。", strings.TrimSpace(bot.Name)),
	}
	if strings.TrimSpace(bot.Relationship) != "" {
		parts = append(parts, "与用户关系："+strings.TrimSpace(bot.Relationship)+"。")
	}
	if strings.TrimSpace(bot.Role) != "" {
		parts = append(parts, "角色设定："+strings.TrimSpace(bot.Role)+"。")
	}
	if len(bot.SelfCognition) > 0 {
		parts = append(parts, "自我认知："+strings.Join(bot.SelfCognition, "；")+"。")
	}
	if strings.TrimSpace(bot.SpeakingStyle) != "" {
		parts = append(parts, "说话风格："+strings.TrimSpace(strings.ReplaceAll(bot.SpeakingStyle, "|", "、"))+"。")
	}
	if len(bot.Traits) > 0 {
		parts = append(parts, "特质："+strings.Join(bot.Traits, "、")+"。")
	}
	if strings.TrimSpace(bot.Gender) != "" {
		parts = append(parts, "性别："+strings.TrimSpace(bot.Gender)+"。")
	}

	joined := strings.Join(parts, " ")
	runes := []rune(joined)
	if len(runes) <= maxRunes {
		return strings.TrimSpace(joined)
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}
