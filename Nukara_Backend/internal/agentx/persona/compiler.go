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
	parts := []string{}
	if strings.TrimSpace(bot.Name) != "" {
		parts = append(parts, fmt.Sprintf("你是%s。", strings.TrimSpace(bot.Name)))
	}
	appendSection := func(title, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, title+value)
		}
	}
	appendSection("【身份设定】", bot.Identity)
	if len(bot.Personality) > 0 {
		appendSection("【性格特征】", strings.Join(bot.Personality, "、"))
	}
	appendSection("【表达风格】", bot.ExpressionStyle)
	appendSection("【生活环境】", bot.LifeContext)
	appendSection("【禁忌与偏好】", bot.TaboosAndPreferences)

	joined := strings.Join(parts, "\n")
	runes := []rune(joined)
	if len(runes) <= maxRunes {
		return strings.TrimSpace(joined)
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}
