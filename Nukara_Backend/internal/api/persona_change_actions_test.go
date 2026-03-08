package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"nukara/backend/internal/store"
)

func TestPersonaChangeActions(t *testing.T) {
	t.Run("accept applies pending persona change", func(t *testing.T) {
		server, token, userID, botID, _, st, closeFn := setupTestServerWithStore(t, fakeBotProfileNanobotHandler())
		defer closeFn()

		change, err := st.CreatePersonaChangeEvent(store.PersonaChangeEvent{
			UserID:        userID,
			BotID:         botID,
			Field:         "identity",
			ChangeType:    "append",
			ProposedValue: "其实是夜班护士",
			SourceTurnID:  "turn-pending",
			Risk:          "high",
			Status:        "pending",
		})
		if err != nil {
			t.Fatalf("create pending change failed: %v", err)
		}

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/bots/"+botID+"/persona-changes/"+change.ID+"/accept", bytes.NewReader([]byte(`{"reviewer_note":"同意合并"}`)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("accept request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("accept status=%d", resp.StatusCode)
		}

		var got map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode accept response failed: %v", err)
		}
		changeMap, _ := got["change"].(map[string]any)
		if changeMap == nil || changeMap["status"] != "accepted" {
			t.Fatalf("unexpected accepted change payload: %v", got["change"])
		}
		botMap, _ := got["bot"].(map[string]any)
		identity, _ := botMap["identity"].(string)
		if !strings.Contains(identity, "其实是夜班护士") {
			t.Fatalf("expected identity updated after accept, got=%v", botMap)
		}
		accepted := st.ListPersonaChangeEvents(userID, botID, "accepted", 10)
		if len(accepted) == 0 {
			t.Fatalf("expected accepted event persisted")
		}
	})

	t.Run("reject marks pending change without mutating persona", func(t *testing.T) {
		server, token, userID, botID, _, st, closeFn := setupTestServerWithStore(t, fakeBotProfileNanobotHandler())
		defer closeFn()

		before, found := st.GetBot(userID, botID)
		if !found {
			t.Fatalf("bot not found")
		}
		change, err := st.CreatePersonaChangeEvent(store.PersonaChangeEvent{
			UserID:        userID,
			BotID:         botID,
			Field:         "life_context",
			ChangeType:    "append",
			ProposedValue: "最近换到夜班节奏",
			SourceTurnID:  "turn-pending",
			Risk:          "high",
			Status:        "pending",
		})
		if err != nil {
			t.Fatalf("create pending change failed: %v", err)
		}

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/bots/"+botID+"/persona-changes/"+change.ID+"/reject", bytes.NewReader([]byte(`{"reviewer_note":"暂不采用"}`)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("reject request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("reject status=%d", resp.StatusCode)
		}

		var got map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode reject response failed: %v", err)
		}
		changeMap, _ := got["change"].(map[string]any)
		if changeMap == nil || changeMap["status"] != "rejected" {
			t.Fatalf("unexpected rejected change payload: %v", got["change"])
		}
		after, found := st.GetBot(userID, botID)
		if !found {
			t.Fatalf("bot not found after reject")
		}
		if after.LifeContext != before.LifeContext {
			t.Fatalf("expected reject not to mutate life_context, before=%q after=%q", before.LifeContext, after.LifeContext)
		}
	})
}
