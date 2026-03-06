package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"
)

func (p *PostgresStore) SetUserProviderSetting(userID, providerID, model string) error {
	if err := p.Store.SetUserProviderSetting(userID, providerID, model); err != nil {
		return err
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO user_provider_settings(user_id, provider_id, model, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET provider_id=EXCLUDED.provider_id, model=EXCLUDED.model, updated_at=NOW()
	`, userID, providerID, nullIfEmpty(model))
	if err != nil {
		log.Printf("set user provider setting failed: %v", err)
	}
	return err
}

func (p *PostgresStore) GetUserProviderSetting(userID string) (providerID, model string, ok bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	err := p.db.QueryRowContext(ctx, `
		SELECT provider_id, COALESCE(model, '')
		FROM user_provider_settings
		WHERE user_id=$1
	`, strings.TrimSpace(userID)).Scan(&providerID, &model)
	if err == nil {
		return providerID, model, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get user provider setting failed: %v", err)
	}
	return p.Store.GetUserProviderSetting(userID)
}

func (p *PostgresStore) SetBotProviderOverride(userID, botID, providerID, model string) error {
	if err := p.Store.SetBotProviderOverride(userID, botID, providerID, model); err != nil {
		return err
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO bot_provider_overrides(user_id, bot_id, provider_id, model, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, bot_id)
		DO UPDATE SET provider_id=EXCLUDED.provider_id, model=EXCLUDED.model, updated_at=NOW()
	`, userID, botID, providerID, nullIfEmpty(model))
	if err != nil {
		log.Printf("set bot provider override failed: %v", err)
	}
	return err
}

func (p *PostgresStore) GetBotProviderOverride(userID, botID string) (providerID, model string, ok bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	err := p.db.QueryRowContext(ctx, `
		SELECT provider_id, COALESCE(model, '')
		FROM bot_provider_overrides
		WHERE user_id=$1 AND bot_id=$2
	`, strings.TrimSpace(userID), strings.TrimSpace(botID)).Scan(&providerID, &model)
	if err == nil {
		return providerID, model, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get bot provider override failed: %v", err)
	}
	return p.Store.GetBotProviderOverride(userID, botID)
}

func (p *PostgresStore) SetSystemSetting(key, value string) error {
	if err := p.Store.SetSystemSetting(key, value); err != nil {
		return err
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO system_settings(key, value, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (key)
		DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()
	`, strings.TrimSpace(key), marshalSettingValue(value))
	if err != nil {
		log.Printf("set system setting failed: key=%s err=%v", key, err)
	}
	return err
}

func (p *PostgresStore) GetSystemSetting(key string) (value string, ok bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var raw []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT value
		FROM system_settings
		WHERE key=$1
	`, strings.TrimSpace(key)).Scan(&raw)
	if err == nil {
		return unmarshalSettingValue(raw), true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get system setting failed: key=%s err=%v", key, err)
	}
	return p.Store.GetSystemSetting(key)
}

func (p *PostgresStore) CreateTurn(turn AgentTurn) (AgentTurn, error) {
	created, err := p.Store.CreateTurn(turn)
	if err != nil {
		return AgentTurn{}, err
	}

	ctx, cancel := p.withTimeout()
	defer cancel()
	ids, _ := json.Marshal(created.UserMessageIDs)
	_, dbErr := p.db.ExecContext(ctx, `
		INSERT INTO agent_turns(id, user_id, bot_id, conversation_id, user_message_ids, aggregated_user_text, bot_reply_text, created_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8)
	`, created.ID, created.UserID, created.BotID, created.ConversationID, string(ids), created.AggregatedUserText, created.BotReplyText, created.CreatedAt)
	if dbErr != nil {
		log.Printf("create turn failed: %v", dbErr)
	}
	return created, dbErr
}

func (p *PostgresStore) UpsertCompact(conversationID, compactJSON, untilTurnID string) error {
	if err := p.Store.UpsertCompact(conversationID, compactJSON, untilTurnID); err != nil {
		return err
	}

	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO conversation_compacts(conversation_id, compact_json, until_turn_id, updated_at)
		VALUES ($1, $2::jsonb, $3, NOW())
		ON CONFLICT (conversation_id)
		DO UPDATE SET compact_json=EXCLUDED.compact_json, until_turn_id=EXCLUDED.until_turn_id, updated_at=NOW()
	`, strings.TrimSpace(conversationID), normalizeJSONOrObject(compactJSON), nullIfEmpty(untilTurnID))
	if err != nil {
		log.Printf("upsert compact failed: %v", err)
	}
	return err
}

func (p *PostgresStore) GetConversationCompact(conversationID string) (ConversationCompact, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var compact ConversationCompact
	var raw json.RawMessage
	err := p.db.QueryRowContext(ctx, `
		SELECT conversation_id, compact_json, COALESCE(until_turn_id::text, ''), updated_at
		FROM conversation_compacts
		WHERE conversation_id=$1
	`, strings.TrimSpace(conversationID)).Scan(&compact.ConversationID, &raw, &compact.UntilTurnID, &compact.UpdatedAt)
	if err == nil {
		compact.CompactJSON = strings.TrimSpace(string(raw))
		return compact, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get compact failed: %v", err)
	}
	return p.Store.GetConversationCompact(conversationID)
}

func (p *PostgresStore) UpsertMemoryItem(item MemoryItem) (MemoryItem, error) {
	saved, err := p.Store.UpsertMemoryItem(item)
	if err != nil {
		return MemoryItem{}, err
	}

	ctx, cancel := p.withTimeout()
	defer cancel()
	topics, _ := json.Marshal(saved.Topics)
	_, dbErr := p.db.ExecContext(ctx, `
		INSERT INTO memory_items(id, user_id, bot_id, kind, owner, content, importance, occurred_at, status, topics, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12)
		ON CONFLICT (id)
		DO UPDATE SET kind=EXCLUDED.kind, owner=EXCLUDED.owner, content=EXCLUDED.content, importance=EXCLUDED.importance,
		  occurred_at=EXCLUDED.occurred_at, status=EXCLUDED.status, topics=EXCLUDED.topics, updated_at=EXCLUDED.updated_at
	`, saved.ID, saved.UserID, saved.BotID, saved.Kind, saved.Owner, saved.Content, saved.Importance, saved.OccurredAt, saved.Status, string(topics), saved.CreatedAt, saved.UpdatedAt)
	if dbErr != nil {
		log.Printf("upsert memory item failed: %v", dbErr)
	}
	return saved, dbErr
}

func (p *PostgresStore) GetMemoryItem(memoryID string) (MemoryItem, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var item MemoryItem
	var topicsRaw []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT id, user_id, bot_id, kind, owner, content, importance, occurred_at, status, topics, created_at, updated_at
		FROM memory_items
		WHERE id=$1
	`, strings.TrimSpace(memoryID)).Scan(
		&item.ID, &item.UserID, &item.BotID, &item.Kind, &item.Owner, &item.Content, &item.Importance,
		&item.OccurredAt, &item.Status, &topicsRaw, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == nil {
		_ = json.Unmarshal(topicsRaw, &item.Topics)
		return item, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get memory item failed: %v", err)
	}
	return p.Store.GetMemoryItem(memoryID)
}

func (p *PostgresStore) ListMemoryItems(userID, botID string, limit int) []MemoryItem {
	ctx, cancel := p.withTimeout()
	defer cancel()

	if limit <= 0 {
		limit = 20
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, user_id, bot_id, kind, owner, content, importance, occurred_at, status, topics, created_at, updated_at
		FROM memory_items
		WHERE user_id=$1 AND bot_id=$2 AND status='active'
		ORDER BY importance DESC, occurred_at DESC
		LIMIT $3
	`, strings.TrimSpace(userID), strings.TrimSpace(botID), limit)
	if err != nil {
		log.Printf("list memory items failed: %v", err)
		return p.Store.ListMemoryItems(userID, botID, limit)
	}
	defer rows.Close()

	items := make([]MemoryItem, 0, limit)
	for rows.Next() {
		var item MemoryItem
		var topicsRaw []byte
		if scanErr := rows.Scan(
			&item.ID, &item.UserID, &item.BotID, &item.Kind, &item.Owner, &item.Content, &item.Importance,
			&item.OccurredAt, &item.Status, &topicsRaw, &item.CreatedAt, &item.UpdatedAt,
		); scanErr != nil {
			log.Printf("scan memory item failed: %v", scanErr)
			continue
		}
		_ = json.Unmarshal(topicsRaw, &item.Topics)
		items = append(items, item)
	}
	if len(items) == 0 {
		return p.Store.ListMemoryItems(userID, botID, limit)
	}
	return items
}

func marshalSettingValue(value string) string {
	raw, _ := json.Marshal(map[string]string{"value": strings.TrimSpace(value)})
	return string(raw)
}

func unmarshalSettingValue(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return strings.TrimSpace(string(raw))
	}
	if value, ok := payload["value"].(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(string(raw))
}

func normalizeJSONOrObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	var anyValue any
	if err := json.Unmarshal([]byte(raw), &anyValue); err != nil {
		fallback, _ := json.Marshal(map[string]string{"summary": raw})
		return string(fallback)
	}
	return raw
}

func withContextTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, timeout)
}
