package persona

import (
	"strings"
	"testing"

	"nukara/backend/internal/store"
)

func TestCompilePromptDeterministicAndTruncated(t *testing.T) {
	bot := store.Bot{
		Name:          "小鹿",
		Relationship:  "亲密朋友",
		Role:          strings.Repeat("角色设定", 80),
		SelfCognition: []string{"我会记得用户偏好", strings.Repeat("认知", 80)},
		SpeakingStyle: "温柔|自然|简短",
		Traits:        []string{"细致", "体贴"},
	}
	got1 := CompilePrompt(bot, 180)
	got2 := CompilePrompt(bot, 180)

	if got1 != got2 {
		t.Fatalf("compile should be deterministic")
	}
	if len([]rune(got1)) > 180 {
		t.Fatalf("compiled prompt too long: %d", len([]rune(got1)))
	}
	if !strings.Contains(got1, "小鹿") || !strings.Contains(got1, "亲密朋友") {
		t.Fatalf("compiled prompt missing key fields: %s", got1)
	}
}

