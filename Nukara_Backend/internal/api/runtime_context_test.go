package api

import (
	"strings"
	"testing"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestRuntimeContextIncludesRuntimeStateAndPromises(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("13900139003", "runtime-context")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	bot := st.CreateBot(user.ID, store.Bot{
		Name:            "苏子衿",
		Identity:        "会认真接住你情绪的人",
		Personality:     []string{"细腻", "敏锐"},
		ExpressionStyle: "口语化，短句",
		LifeContext:     "住在东京，偶尔夜班",
	})
	conv, found := st.FindConversationByBot(user.ID, bot.ID)
	if !found {
		t.Fatalf("conversation not found")
	}
	if err := st.UpsertCompact(conv.ID, `{"summary":"最近常聊摄影","facts":[{"content":"周末会去拍街景"}]}`, "turn-1"); err != nil {
		t.Fatalf("upsert compact failed: %v", err)
	}
	if _, err := st.UpsertBotRuntimeState(store.BotRuntimeState{
		UserID:       user.ID,
		BotID:        bot.ID,
		ActivityText: "刚下晚班，在回去路上",
		BasisTags:    []string{"self_fact"},
	}); err != nil {
		t.Fatalf("upsert runtime state failed: %v", err)
	}
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		UserID:     user.ID,
		BotID:      bot.ID,
		Kind:       "promise",
		Owner:      "bot",
		Content:    "答应今晚把歌单发给你",
		Importance: 90,
		Status:     "active",
	}); err != nil {
		t.Fatalf("save promise failed: %v", err)
	}
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		UserID:     user.ID,
		BotID:      bot.ID,
		Kind:       "user_fact",
		Owner:      "user",
		Content:    "你下周一要考试",
		Importance: 88,
		Status:     "active",
	}); err != nil {
		t.Fatalf("save user_fact failed: %v", err)
	}

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	systemContext := agent.BuildSystemContext(bot, nil)
	prompt, _ := server.buildRuntimeContext(user.ID, bot.ID, conv.ID, "你答应我的歌单整理好了吗？你现在在做什么？", nil, systemContext)

	if !strings.Contains(prompt, "【角色设定】") {
		t.Fatalf("expected stable persona in prompt, got=%s", prompt)
	}
	if !strings.Contains(prompt, "【当前状态】") || !strings.Contains(prompt, "刚下晚班，在回去路上") {
		t.Fatalf("expected runtime state block in prompt, got=%s", prompt)
	}
	if !strings.Contains(prompt, "【进行中约定】") || !strings.Contains(prompt, "答应今晚把歌单发给你") {
		t.Fatalf("expected promise block in prompt, got=%s", prompt)
	}
}

func TestNewTurnRequestSeparatesProviderConversationIDFromLocalConversationID(t *testing.T) {
	st := store.NewStore()
	user, err := st.CreateUser("13900139004", "runtime-conversation-id")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	bot := st.CreateBot(user.ID, store.Bot{Name: "苏子衿", Identity: "温柔的人"})
	conv, found := st.FindConversationByBot(user.ID, bot.ID)
	if !found {
		t.Fatalf("conversation not found")
	}
	if err := st.UpsertCompact(conv.ID, `{"summary":"这是一段本地会话摘要"}`, "turn-1"); err != nil {
		t.Fatalf("upsert compact failed: %v", err)
	}

	server := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	providerConversationID := agent.NanobotConvID(user.ID, bot.ID, conv.ID)
	request := server.newTurnRequestWithProviderConversation(
		user.ID,
		bot.ID,
		conv.ID,
		providerConversationID,
		"你还记得我们刚才聊什么吗",
		nil,
		agent.BuildSystemContext(bot, nil),
	)

	if request.ConversationID != conv.ID {
		t.Fatalf("local conversation id = %q, want %q", request.ConversationID, conv.ID)
	}
	if request.ProviderConversationID != providerConversationID {
		t.Fatalf("provider conversation id = %q, want %q", request.ProviderConversationID, providerConversationID)
	}
	if !strings.Contains(request.SystemPrompt, "这是一段本地会话摘要") {
		t.Fatalf("expected compact from local conversation id, got=%s", request.SystemPrompt)
	}
}
