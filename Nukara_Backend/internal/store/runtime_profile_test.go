package store

import (
	"testing"
	"time"
)

func TestRuntimeProfileStateUpsertAndGet(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 3, 8, 23, 30, 0, 0, time.UTC)

	state, err := s.UpsertBotRuntimeState(BotRuntimeState{
		UserID:          "user-1",
		BotID:           "bot-1",
		ActivityText:    "刚下晚班，在回去路上",
		BasisTags:       []string{"time", "event", "habit"},
		SourceMemoryIDs: []string{"mem-1", "mem-2"},
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("UpsertBotRuntimeState create failed: %v", err)
	}
	if state.ActivityText != "刚下晚班，在回去路上" {
		t.Fatalf("activity = %q", state.ActivityText)
	}

	updated, err := s.UpsertBotRuntimeState(BotRuntimeState{
		UserID:          "user-1",
		BotID:           "bot-1",
		ActivityText:    "回到住处，正在泡面",
		BasisTags:       []string{"time", "self_fact"},
		SourceMemoryIDs: []string{"mem-3"},
		UpdatedAt:       now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("UpsertBotRuntimeState update failed: %v", err)
	}
	if updated.ActivityText != "回到住处，正在泡面" {
		t.Fatalf("updated activity = %q", updated.ActivityText)
	}

	got, ok := s.GetBotRuntimeState("user-1", "bot-1")
	if !ok {
		t.Fatal("expected runtime state to exist")
	}
	if got.ActivityText != "回到住处，正在泡面" {
		t.Fatalf("stored activity = %q", got.ActivityText)
	}
	if len(got.BasisTags) != 2 || got.BasisTags[0] != "time" || got.BasisTags[1] != "self_fact" {
		t.Fatalf("basis tags = %v", got.BasisTags)
	}
	if len(got.SourceMemoryIDs) != 1 || got.SourceMemoryIDs[0] != "mem-3" {
		t.Fatalf("source memory ids = %v", got.SourceMemoryIDs)
	}
}

func TestPersonaChangeEventLifecycle(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)

	first, err := s.CreatePersonaChangeEvent(PersonaChangeEvent{
		UserID:        "user-1",
		BotID:         "bot-1",
		Field:         "identity",
		ChangeType:    "append",
		ProposedValue: "其实我是医生",
		SourceTurnID:  "turn-1",
		Risk:          "high",
		Status:        "pending",
		CreatedAt:     now,
	})
	if err != nil {
		t.Fatalf("CreatePersonaChangeEvent first failed: %v", err)
	}
	if first.ID == "" {
		t.Fatal("expected generated change id")
	}

	second, err := s.CreatePersonaChangeEvent(PersonaChangeEvent{
		UserID:        "user-1",
		BotID:         "bot-1",
		Field:         "expression_style",
		ChangeType:    "append",
		ProposedValue: "最近说话更黏人了",
		SourceTurnID:  "turn-2",
		Risk:          "low",
		Status:        "active",
		CreatedAt:     now.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreatePersonaChangeEvent second failed: %v", err)
	}

	pending := s.ListPersonaChangeEvents("user-1", "bot-1", "pending", 10)
	if len(pending) != 1 {
		t.Fatalf("pending events = %d, want 1", len(pending))
	}
	if pending[0].ID != first.ID {
		t.Fatalf("pending id = %q, want %q", pending[0].ID, first.ID)
	}

	all := s.ListPersonaChangeEvents("user-1", "bot-1", "", 10)
	if len(all) != 2 {
		t.Fatalf("all events = %d, want 2", len(all))
	}
	if all[0].ID != second.ID {
		t.Fatalf("newest-first id = %q, want %q", all[0].ID, second.ID)
	}

	updated, ok := s.UpdatePersonaChangeEventStatus(first.ID, "accepted", "user confirmed")
	if !ok {
		t.Fatal("expected status update to succeed")
	}
	if updated.Status != "accepted" {
		t.Fatalf("updated status = %q", updated.Status)
	}
	if updated.ReviewerNote != "user confirmed" {
		t.Fatalf("reviewer note = %q", updated.ReviewerNote)
	}

	accepted := s.ListPersonaChangeEvents("user-1", "bot-1", "accepted", 10)
	if len(accepted) != 1 || accepted[0].ID != first.ID {
		t.Fatalf("accepted events = %+v", accepted)
	}
}

func TestPersonaChangeEventsKeepLatestTwenty(t *testing.T) {
	s := NewStore()
	base := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		_, err := s.CreatePersonaChangeEvent(PersonaChangeEvent{
			UserID:        "user-1",
			BotID:         "bot-1",
			Field:         "life_context",
			ChangeType:    "append",
			ProposedValue: "变化-" + string(rune('A'+i)),
			Status:        "accepted",
			CreatedAt:     base.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("CreatePersonaChangeEvent #%d failed: %v", i, err)
		}
	}

	all := s.ListPersonaChangeEvents("user-1", "bot-1", "", 30)
	if len(all) != 20 {
		t.Fatalf("events retained = %d, want 20", len(all))
	}
	if all[0].ProposedValue != "变化-Y" {
		t.Fatalf("newest retained value = %q", all[0].ProposedValue)
	}
	if all[len(all)-1].ProposedValue != "变化-F" {
		t.Fatalf("oldest retained value = %q", all[len(all)-1].ProposedValue)
	}
}

func TestMemoryItemPersistsEntitiesAndRelations(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 3, 8, 12, 30, 0, 0, time.UTC)

	item, err := s.UpsertMemoryItem(MemoryItem{
		UserID:           "user-1",
		BotID:            "bot-1",
		Kind:             "self_fact",
		Owner:            "bot",
		Content:          "我养了一只猫叫小蜜",
		Importance:       88,
		OccurredAt:       now,
		Status:           "active",
		SemanticCategory: "life_context",
		Stability:        "stable",
		Entities: []Entity{{
			ID:   "pet-xiaomi",
			Type: "pet",
			Name: "小蜜",
			Role: "bot",
		}},
		Relations: []Relation{{
			SourceEntityID: "bot-self",
			TargetEntityID: "pet-xiaomi",
			RelationType:   "owns",
			RoleHint:       "first_person",
		}},
	})
	if err != nil {
		t.Fatalf("UpsertMemoryItem failed: %v", err)
	}
	if item.SemanticCategory != "life_context" || item.Stability != "stable" {
		t.Fatalf("unexpected memory item metadata: %+v", item)
	}
	if len(item.Entities) != 1 || item.Entities[0].Name != "小蜜" {
		t.Fatalf("entities = %+v", item.Entities)
	}
	if len(item.Relations) != 1 || item.Relations[0].RelationType != "owns" {
		t.Fatalf("relations = %+v", item.Relations)
	}

	got, ok := s.GetMemoryItem(item.ID)
	if !ok {
		t.Fatal("expected memory item to exist")
	}
	if got.SemanticCategory != "life_context" || got.Stability != "stable" {
		t.Fatalf("stored metadata = %+v", got)
	}
	if len(got.Entities) != 1 || got.Entities[0].ID != "pet-xiaomi" {
		t.Fatalf("stored entities = %+v", got.Entities)
	}
	if len(got.Relations) != 1 || got.Relations[0].TargetEntityID != "pet-xiaomi" {
		t.Fatalf("stored relations = %+v", got.Relations)
	}
}

func TestPersonaChangeEventDefaultsToAcceptedAuditStatus(t *testing.T) {
	s := NewStore()

	event, err := s.CreatePersonaChangeEvent(PersonaChangeEvent{
		UserID:        "user-1",
		BotID:         "bot-1",
		Field:         "identity",
		ChangeType:    "append",
		ProposedValue: "偏研究型的金融从业背景",
		Risk:          "low",
	})
	if err != nil {
		t.Fatalf("CreatePersonaChangeEvent failed: %v", err)
	}
	if event.Status != "accepted" {
		t.Fatalf("status = %q, want accepted", event.Status)
	}
}
