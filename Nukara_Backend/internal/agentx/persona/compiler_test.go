package persona

import (
	"strings"
	"testing"

	"nukara/backend/internal/store"
)

func TestCompilePromptDeterministicAndTruncated(t *testing.T) {
	bot := store.Bot{
		Name:                 "小鹿",
		Identity:             strings.Repeat("总会认真接住你情绪的恋人。", 4),
		Personality:          []string{"细致", "体贴"},
		ExpressionStyle:      strings.Repeat("温柔、自然、简短。", 4),
		LifeContext:          strings.Repeat("住在东京，喜欢摄影和散步。", 4),
		TaboosAndPreferences: strings.Repeat("不喜欢被命令式对待，更喜欢被温柔回应。", 4),
	}
	got1 := CompilePrompt(bot, 220)
	got2 := CompilePrompt(bot, 220)

	if got1 != got2 {
		t.Fatalf("compile should be deterministic")
	}
	if len([]rune(got1)) > 220 {
		t.Fatalf("compiled prompt too long: %d", len([]rune(got1)))
	}
	for _, section := range []string{"【身份设定】", "【性格特征】", "【表达风格】", "【生活环境】", "【禁忌与偏好】"} {
		if !strings.Contains(got1, section) {
			t.Fatalf("compiled prompt missing section %s: %s", section, got1)
		}
	}
	if !strings.Contains(got1, "小鹿") || !strings.Contains(got1, "细致") {
		t.Fatalf("compiled prompt missing key fields: %s", got1)
	}
}
