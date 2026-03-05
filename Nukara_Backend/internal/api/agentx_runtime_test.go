package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/agentx"
	"nukara/backend/internal/agentx/llm"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestWSChatUsesAgentXRuntimePipeline(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"，很高兴见到你\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer provider.Close()

	st := store.NewStore()
	user, err := st.CreateUser("13900139001", "runtime-user")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	bot := st.CreateBot(user.ID, store.Bot{Name: "测试 bot", Summary: "陪伴"})
	conv, ok := st.FindConversationByBot(user.ID, bot.ID)
	if !ok {
		t.Fatalf("conversation not found")
	}

	runtime := agentx.NewRuntime(agentx.RuntimeDeps{
		ProviderClient: llm.NewOpenAICompatClient(provider.URL, "test-key", "test-model", provider.Client()),
	})
	apiServer := NewServer(st, nil, apns.NewClient("com.nukara.app"), "test-secret", "")
	apiServer.SetChatRuntime(runtime)

	token, err := apiServer.issueToken(user.ID)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	httpServer := httptest.NewServer(apiServer.HandlerFor("gateway"))
	defer httpServer.Close()

	ws := mustDialWS(t, httpServer.URL, token)
	defer ws.Close()

	ws.SendJSON(t, map[string]any{
		"type":            "message",
		"conversation_id": conv.ID,
		"client_msg_id":   "runtime-client-msg",
		"content": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})

	seenChunk := false
	seenMessage := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		event := ws.ReadJSON(t, 2*time.Second)
		if event == nil {
			continue
		}
		switch event["type"] {
		case "stream_chunk":
			seenChunk = true
		case "message":
			seenMessage = true
			content, _ := event["content"].(map[string]any)
			text, _ := content["text"].(string)
			if strings.TrimSpace(text) == "" {
				t.Fatalf("empty final message")
			}
		}
		if seenChunk && seenMessage {
			return
		}
	}
	raw, _ := json.Marshal(map[string]bool{"stream_chunk": seenChunk, "message": seenMessage})
	t.Fatalf("agentx runtime websocket pipeline incomplete: %s", string(raw))
}
