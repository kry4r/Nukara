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
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		UserID:     userID,
		BotID:      botID,
		Kind:       "impression",
		Owner:      "bot",
		Content:    "你给我的感觉是理性又温柔。",
		Importance: 92,
	}); err != nil {
		t.Fatalf("save impression failed: %v", err)
	}
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		UserID:     userID,
		BotID:      botID,
		Kind:       "promise",
		Owner:      "bot",
		Content:    "答应这周给你整理歌单",
		Importance: 85,
	}); err != nil {
		t.Fatalf("save promise failed: %v", err)
	}
	if _, err := st.UpsertMemoryItem(store.MemoryItem{
		UserID:     userID,
		BotID:      botID,
		Kind:       "event",
		Owner:      "bot",
		Content:    "昨天去看了摄影展",
		Importance: 70,
	}); err != nil {
		t.Fatalf("save event failed: %v", err)
	}
	if _, err := st.UpsertBotRuntimeState(store.BotRuntimeState{
		UserID:       userID,
		BotID:        botID,
		ActivityText: "刚下晚班，在回去路上",
		BasisTags:    []string{"self_fact"},
	}); err != nil {
		t.Fatalf("save runtime state failed: %v", err)
	}
	if _, err := st.CreatePersonaChangeEvent(store.PersonaChangeEvent{
		UserID:        userID,
		BotID:         botID,
		Field:         "identity",
		ChangeType:    "append",
		ProposedValue: "其实是夜班护士",
		SourceTurnID:  "turn-accepted",
		Risk:          "high",
		Status:        "accepted",
	}); err != nil {
		t.Fatalf("create accepted persona change failed: %v", err)
	}
	if _, err := st.CreatePersonaChangeEvent(store.PersonaChangeEvent{
		UserID:        userID,
		BotID:         botID,
		Field:         "life_context",
		ChangeType:    "append",
		ProposedValue: "最近换到夜班节奏",
		SourceTurnID:  "turn-pending",
		Risk:          "high",
		Status:        "pending",
	}); err != nil {
		t.Fatalf("create pending persona change failed: %v", err)
	}

	t.Run("profile endpoint returns runtime portrait payload", func(t *testing.T) {
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
		if _, ok := got["directives"]; ok {
			t.Fatalf("expected directives field omitted, got=%v", got["directives"])
		}
		runtimeState, _ := got["runtime_state"].(map[string]any)
		if runtimeState == nil || runtimeState["activity_text"] != "刚下晚班，在回去路上" {
			t.Fatalf("unexpected runtime_state: %v", got["runtime_state"])
		}
		recentImpressions, _ := got["recent_impressions"].([]any)
		if len(recentImpressions) == 0 {
			t.Fatalf("expected recent_impressions, got=%v", got["recent_impressions"])
		}
		firstImpression, _ := recentImpressions[0].(map[string]any)
		if firstImpression == nil || firstImpression["kind"] != "impression" || firstImpression["content"] != "你给我的感觉是理性又温柔。" {
			t.Fatalf("unexpected first impression: %v", recentImpressions[0])
		}
		keyMemories, _ := got["key_memories"].([]any)
		if len(keyMemories) == 0 {
			t.Fatalf("expected key_memories, got=%v", got["key_memories"])
		}
		recentChanges, _ := got["recent_changes"].([]any)
		if len(recentChanges) == 0 {
			t.Fatalf("expected recent_changes, got=%v", got["recent_changes"])
		}
		firstRecentChange, _ := recentChanges[0].(map[string]any)
		if firstRecentChange == nil || strings.TrimSpace(firstRecentChange["summary_text"].(string)) == "" {
			t.Fatalf("expected summary_text on recent change, got=%v", recentChanges[0])
		}
		if _, ok := got["pending_persona_changes"]; ok {
			t.Fatalf("expected pending_persona_changes omitted, got=%v", got["pending_persona_changes"])
		}
	})

	t.Run("impression endpoint returns cached text", func(t *testing.T) {
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
		if got["impression"] != "你给我的感觉是理性又温柔。" {
			t.Fatalf("expected cached impression, got=%v", got)
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
			IdentityAdds             []string  `json:"identity_adds"`
			PersonalityAdds          []string  `json:"personality_adds"`
			ExpressionStyleAdds      []string  `json:"expression_style_adds"`
			LifeContextAdds          []string  `json:"life_context_adds"`
			TaboosAndPreferencesAdds []string  `json:"taboos_and_preferences_adds"`
			Bot                      store.Bot `json:"bot"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode iterate response failed: %v", err)
		}
		if len(got.IdentityAdds) == 0 || len(got.PersonalityAdds) == 0 || len(got.ExpressionStyleAdds) == 0 || len(got.LifeContextAdds) == 0 || len(got.TaboosAndPreferencesAdds) == 0 {
			t.Fatalf("expected non-empty v2 adds, got=%+v", got)
		}
		if got.Bot.ID != botID {
			t.Fatalf("unexpected bot in iterate response: %+v", got.Bot)
		}
		if !strings.Contains(got.Bot.Identity, "会认真接住情绪") {
			t.Fatalf("expected identity updated, got=%s", got.Bot.Identity)
		}
		if len(got.Bot.Personality) == 0 {
			t.Fatalf("expected personality updated")
		}
		if !strings.Contains(got.Bot.ExpressionStyle, "会接梗") {
			t.Fatalf("expected expression_style updated, got=%s", got.Bot.ExpressionStyle)
		}
		if !strings.Contains(got.Bot.LifeContext, "最近在学摄影") {
			t.Fatalf("expected life_context updated, got=%s", got.Bot.LifeContext)
		}
		if !strings.Contains(got.Bot.TaboosAndPreferences, "不喜欢被敷衍") {
			t.Fatalf("expected taboos_and_preferences updated, got=%s", got.Bot.TaboosAndPreferences)
		}
	})
}

func TestBotImpressionEndpointPersistsGeneratedImpression(t *testing.T) {
	server, token, userID, botID, _, st, closeFn := setupTestServerWithStore(t, fakeBotProfileNanobotHandler())
	defer closeFn()

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
	if got["impression"] != "你给我的感觉是细腻、可靠。" {
		t.Fatalf("unexpected impression response: %v", got)
	}

	items := st.ListMemoryItems(userID, botID, 10)
	if len(items) == 0 {
		t.Fatalf("expected generated impression persisted")
	}
	if items[0].Kind != "impression" || items[0].Content != "你给我的感觉是细腻、可靠。" {
		t.Fatalf("unexpected persisted impression: %+v", items[0])
	}

	profileReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/bots/"+botID+"/profile", nil)
	profileReq.Header.Set("Authorization", "Bearer "+token)
	profileResp, err := http.DefaultClient.Do(profileReq)
	if err != nil {
		t.Fatalf("profile request failed: %v", err)
	}
	defer profileResp.Body.Close()
	if profileResp.StatusCode != http.StatusOK {
		t.Fatalf("profile status=%d", profileResp.StatusCode)
	}

	var profile map[string]any
	if err := json.NewDecoder(profileResp.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile response failed: %v", err)
	}
	recentImpressions, _ := profile["recent_impressions"].([]any)
	if len(recentImpressions) == 0 {
		t.Fatalf("expected recent impressions after generation, got=%v", profile["recent_impressions"])
	}
	firstImpression, _ := recentImpressions[0].(map[string]any)
	if firstImpression == nil || firstImpression["kind"] != "impression" || firstImpression["content"] != "你给我的感觉是细腻、可靠。" {
		t.Fatalf("unexpected profile impression: %v", recentImpressions[0])
	}
}

func TestBotCRUDUsesPersonaV2(t *testing.T) {
	server, token, _, _, _, _, closeFn := setupTestServerWithStore(t, fakeBotProfileNanobotHandler())
	defer closeFn()

	createBody := []byte(`{
		"name":"苏子衿",
		"identity":"你的恋人，也是会认真接住你情绪的人",
		"personality":["细腻","敏锐"],
		"expression_style":"口语化，短句，会接梗",
		"life_context":"现在住在东京，平时摄影、通勤、喝便利店咖啡",
		"taboos_and_preferences":"不喜欢被命令式对待，更喜欢被温柔回应"
	}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/bots", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create bot request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create bot status=%d", resp.StatusCode)
	}

	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created["identity"] != "你的恋人，也是会认真接住你情绪的人" {
		t.Fatalf("identity = %v", created["identity"])
	}
	personality, _ := created["personality"].([]any)
	if len(personality) != 2 {
		t.Fatalf("personality = %v", created["personality"])
	}
	if created["expression_style"] != "口语化，短句，会接梗" {
		t.Fatalf("expression_style = %v", created["expression_style"])
	}
	if created["life_context"] != "现在住在东京，平时摄影、通勤、喝便利店咖啡" {
		t.Fatalf("life_context = %v", created["life_context"])
	}
	if created["taboos_and_preferences"] != "不喜欢被命令式对待，更喜欢被温柔回应" {
		t.Fatalf("taboos_and_preferences = %v", created["taboos_and_preferences"])
	}
	botID, _ := created["id"].(string)
	if botID == "" {
		t.Fatalf("missing bot id in create response: %v", created)
	}

	getReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/bots/"+botID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("get bot request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get bot status=%d", getResp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(getResp.Body).Decode(&got); err != nil {
		t.Fatalf("decode get response failed: %v", err)
	}
	if got["identity"] != "你的恋人，也是会认真接住你情绪的人" {
		t.Fatalf("get identity = %v", got["identity"])
	}

	updateBody := []byte(`{
		"name":"苏子衿",
		"identity":"会认真接住你情绪，也会轻轻逗你笑的人",
		"personality":["细腻","敏锐","有分寸感"],
		"expression_style":"更口语化，会接梗，也会追问细节",
		"life_context":"现在住在东京，工作日通勤，周末常去拍街景",
		"taboos_and_preferences":"不喜欢被命令式对待，更喜欢被温柔回应和商量"
	}`)
	updateReq, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/bots/"+botID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("update bot request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update bot status=%d", updateResp.StatusCode)
	}
	var updated map[string]any
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updated["identity"] != "会认真接住你情绪，也会轻轻逗你笑的人" {
		t.Fatalf("updated identity = %v", updated["identity"])
	}
	updatedPersonality, _ := updated["personality"].([]any)
	if len(updatedPersonality) != 3 {
		t.Fatalf("updated personality = %v", updated["personality"])
	}
	if updated["expression_style"] != "更口语化，会接梗，也会追问细节" {
		t.Fatalf("updated expression_style = %v", updated["expression_style"])
	}
	if updated["life_context"] != "现在住在东京，工作日通勤，周末常去拍街景" {
		t.Fatalf("updated life_context = %v", updated["life_context"])
	}
	if updated["taboos_and_preferences"] != "不喜欢被命令式对待，更喜欢被温柔回应和商量" {
		t.Fatalf("updated taboos_and_preferences = %v", updated["taboos_and_preferences"])
	}
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
			reply = `{"identity_adds":["会认真接住情绪"],"personality_adds":["观察细致"],"expression_style_adds":["会接梗"],"life_context_adds":["最近在学摄影"],"taboos_and_preferences_adds":["不喜欢被敷衍"]}`
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
