package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"nukara/backend/internal/store"
)

func TestBotProfileEndpoints(t *testing.T) {
	server, token, userID, botID, convID, st, closeFn := setupTestServerWithStore(t, fakeBotProfileNanobotHandler())
	defer closeFn()

	st.SaveDirective(store.Directive{
		UserID:   userID,
		BotID:    botID,
		Content:  "回复更简短",
		Category: "style",
		Source:   "conversation",
		Status:   "active",
	})

	t.Run("profile endpoint returns bot/directives/state", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/bots/"+botID+"/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("profile request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("profile status=%d", resp.StatusCode)
		}

		var got map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode profile response failed: %v", err)
		}

		botMap, _ := got["bot"].(map[string]any)
		if botMap == nil || botMap["id"] != botID {
			t.Fatalf("unexpected bot payload: %v", got["bot"])
		}
		if got["conversation_id"] != convID {
			t.Fatalf("unexpected conversation_id: %v", got["conversation_id"])
		}
		if _, ok := got["bot_state"]; !ok {
			t.Fatalf("missing bot_state")
		}
		directives, _ := got["directives"].([]any)
		if len(directives) == 0 {
			t.Fatalf("expected directives, got empty")
		}
	})

	t.Run("impression endpoint returns text", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/bots/"+botID+"/impression", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("impression request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("impression status=%d", resp.StatusCode)
		}

		var got map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode impression response failed: %v", err)
		}
		if got["impression"] == "" {
			t.Fatalf("expected impression text, got=%v", got)
		}
	})

	t.Run("iterate endpoint appends persona fields", func(t *testing.T) {
		_, _ = st.SaveMessage(userID, store.Message{
			ConversationID: convID,
			SenderType:     "user",
			ContentType:    "text",
			Content:        store.MessageContent{Type: "text", Text: "你最近在做什么？"},
		})
		_, _ = st.SaveMessage(userID, store.Message{
			ConversationID: convID,
			SenderType:     "bot",
			ContentType:    "text",
			Content:        store.MessageContent{Type: "text", Text: "我在整理照片。"},
		})

		reqBody, _ := json.Marshal(map[string]any{"message_limit": 20})
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/bots/"+botID+"/iterate", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("iterate request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("iterate status=%d", resp.StatusCode)
		}

		var got struct {
			SpeakingStyleAdds []string  `json:"speaking_style_adds"`
			BackgroundAdds    []string  `json:"background_adds"`
			TraitAdds         []string  `json:"trait_adds"`
			Gender            string    `json:"gender"`
			Bot               store.Bot `json:"bot"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode iterate response failed: %v", err)
		}
		if len(got.SpeakingStyleAdds) == 0 || len(got.BackgroundAdds) == 0 || len(got.TraitAdds) == 0 {
			t.Fatalf("expected non-empty adds, got=%+v", got)
		}
		if got.Bot.ID != botID {
			t.Fatalf("unexpected bot in iterate response: %+v", got.Bot)
		}
		if got.Gender != "female" {
			t.Fatalf("unexpected gender: %s", got.Gender)
		}
		if got.Bot.SpeakingStyle == "温柔" {
			t.Fatalf("expected speaking style updated, got=%s", got.Bot.SpeakingStyle)
		}
		if len(got.Bot.Traits) == 0 {
			t.Fatalf("expected traits updated")
		}
	})
}

func fakeBotProfileNanobotHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", fakeNanobotWS)
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		prompt := req.Content.Text

		reply := "ok"
		switch {
		case strings.Contains(prompt, "用户印象"):
			reply = "你给我的感觉是细腻、可靠。"
		case strings.Contains(prompt, "仅输出JSON"):
			reply = `{"speaking_style_adds":["会接梗"],"background_adds":["最近在学摄影"],"trait_adds":["观察细致"],"gender":"female"}`
		default:
			reply = "fake reply"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"conversation_id": "test",
			"content": map[string]string{
				"type": "text",
				"text": reply,
			},
		})
	})
	return mux
}
