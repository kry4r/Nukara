package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const temporalMemoryGraphSchema = `
DO $$
BEGIN
    BEGIN
        CREATE EXTENSION IF NOT EXISTS vector;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
END $$;

CREATE TABLE IF NOT EXISTS memory_nodes (
    id TEXT PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    session_id TEXT,
    node_type VARCHAR(40) NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    body_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    salience DOUBLE PRECISION NOT NULL DEFAULT 0,
    affect_weight DOUBLE PRECISION NOT NULL DEFAULT 0,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    stability DOUBLE PRECISION NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    occurred_at TIMESTAMP NOT NULL DEFAULT NOW(),
    observed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    valid_from TIMESTAMP NOT NULL DEFAULT NOW(),
    valid_to TIMESTAMP,
    last_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    source_turn_id TEXT,
    source_kind VARCHAR(40) NOT NULL DEFAULT '',
    semantic_category VARCHAR(40) NOT NULL DEFAULT '',
    stability_label VARCHAR(20) NOT NULL DEFAULT '',
    merge_key TEXT NOT NULL DEFAULT '',
    evidence_count INT NOT NULL DEFAULT 1,
    entities JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
DO $$
BEGIN
    BEGIN
        ALTER TABLE IF EXISTS memory_embeddings DROP CONSTRAINT IF EXISTS memory_embeddings_node_id_fkey;
        ALTER TABLE IF EXISTS memory_edges DROP CONSTRAINT IF EXISTS memory_edges_source_id_fkey;
        ALTER TABLE IF EXISTS memory_edges DROP CONSTRAINT IF EXISTS memory_edges_target_id_fkey;
        ALTER TABLE IF EXISTS memory_nodes ALTER COLUMN id TYPE TEXT USING id::text;
        ALTER TABLE IF EXISTS memory_nodes ALTER COLUMN session_id TYPE TEXT USING session_id::text;
        ALTER TABLE IF EXISTS memory_nodes ALTER COLUMN source_turn_id TYPE TEXT USING source_turn_id::text;
        ALTER TABLE IF EXISTS memory_nodes ADD COLUMN IF NOT EXISTS source_kind VARCHAR(40) NOT NULL DEFAULT '';
        ALTER TABLE IF EXISTS memory_nodes ADD COLUMN IF NOT EXISTS semantic_category VARCHAR(40) NOT NULL DEFAULT '';
        ALTER TABLE IF EXISTS memory_nodes ADD COLUMN IF NOT EXISTS stability_label VARCHAR(20) NOT NULL DEFAULT '';
        ALTER TABLE IF EXISTS memory_nodes ADD COLUMN IF NOT EXISTS merge_key TEXT NOT NULL DEFAULT '';
        ALTER TABLE IF EXISTS memory_nodes ADD COLUMN IF NOT EXISTS evidence_count INT NOT NULL DEFAULT 1;
        ALTER TABLE IF EXISTS memory_nodes ADD COLUMN IF NOT EXISTS entities JSONB NOT NULL DEFAULT '[]'::jsonb;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
END $$;
CREATE INDEX IF NOT EXISTS idx_memory_nodes_user_bot ON memory_nodes(user_id, bot_id, status, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_nodes_type ON memory_nodes(user_id, bot_id, node_type, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_nodes_session ON memory_nodes(user_id, bot_id, session_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_edges (
    id UUID PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES memory_nodes(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL REFERENCES memory_nodes(id) ON DELETE CASCADE,
    edge_type VARCHAR(40) NOT NULL,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1,
    evidence_count INT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
DO $$
BEGIN
    BEGIN
        ALTER TABLE IF EXISTS memory_edges ALTER COLUMN source_id TYPE TEXT USING source_id::text;
        ALTER TABLE IF EXISTS memory_edges ALTER COLUMN target_id TYPE TEXT USING target_id::text;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
    BEGIN
        ALTER TABLE memory_edges ADD CONSTRAINT memory_edges_source_id_fkey FOREIGN KEY (source_id) REFERENCES memory_nodes(id) ON DELETE CASCADE;
    EXCEPTION WHEN duplicate_object THEN
        NULL;
    END;
    BEGIN
        ALTER TABLE memory_edges ADD CONSTRAINT memory_edges_target_id_fkey FOREIGN KEY (target_id) REFERENCES memory_nodes(id) ON DELETE CASCADE;
    EXCEPTION WHEN duplicate_object THEN
        NULL;
    END;
END $$;
CREATE INDEX IF NOT EXISTS idx_memory_edges_source ON memory_edges(source_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_edges_target ON memory_edges(target_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_edges_type ON memory_edges(edge_type, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_embeddings (
    node_id TEXT PRIMARY KEY REFERENCES memory_nodes(id) ON DELETE CASCADE,
    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
DO $$
BEGIN
    BEGIN
        ALTER TABLE IF EXISTS memory_embeddings ALTER COLUMN node_id TYPE TEXT USING node_id::text;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
    BEGIN
        ALTER TABLE memory_embeddings ADD CONSTRAINT memory_embeddings_node_id_fkey FOREIGN KEY (node_id) REFERENCES memory_nodes(id) ON DELETE CASCADE;
    EXCEPTION WHEN duplicate_object THEN
        NULL;
    END;
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        BEGIN
            ALTER TABLE memory_embeddings ADD COLUMN IF NOT EXISTS embedding vector(1536);
        EXCEPTION WHEN OTHERS THEN
            NULL;
        END;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_memory_embeddings_model ON memory_embeddings(embedding_model, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_cards (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    card_type VARCHAR(40) NOT NULL,
    text TEXT NOT NULL,
    backing_node_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    freshness_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_memory_cards_user_bot ON memory_cards(user_id, bot_id, card_type, updated_at DESC);

CREATE TABLE IF NOT EXISTS activation_traces (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    conversation_id TEXT,
    turn_id TEXT,
    cue_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    seed_node_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    activated_node_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    selected_card_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    response_excerpt TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
DO $$
BEGIN
    BEGIN
        ALTER TABLE IF EXISTS activation_traces ALTER COLUMN conversation_id TYPE TEXT USING conversation_id::text;
        ALTER TABLE IF EXISTS activation_traces ALTER COLUMN turn_id TYPE TEXT USING turn_id::text;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
END $$;
CREATE INDEX IF NOT EXISTS idx_activation_traces_user_bot ON activation_traces(user_id, bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activation_traces_conversation ON activation_traces(conversation_id, created_at DESC);
`

func ensureTemporalMemoryGraphSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, temporalMemoryGraphSchema)
	return err
}

func (p *PostgresStore) CreateMemoryNode(node TemporalMemoryNode) (TemporalMemoryNode, error) {
	saved, err := p.Store.CreateMemoryNode(node)
	if err != nil {
		return TemporalMemoryNode{}, err
	}
	return saved, p.upsertMemoryNodeRow(saved)
}

func (p *PostgresStore) UpdateMemoryNode(node TemporalMemoryNode) (TemporalMemoryNode, error) {
	saved, err := p.Store.UpdateMemoryNode(node)
	if err != nil {
		return TemporalMemoryNode{}, err
	}
	return saved, p.upsertMemoryNodeRow(saved)
}

func (p *PostgresStore) upsertMemoryNodeRow(node TemporalMemoryNode) error {
	ctx, cancel := p.withTimeout()
	defer cancel()
	validTo := any(nil)
	if node.ValidTo != nil {
		validTo = node.ValidTo.UTC()
	}
	entities, _ := json.Marshal(node.Entities)
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO memory_nodes(
			id, user_id, bot_id, session_id, node_type, title, summary, body_json,
			salience, affect_weight, confidence, stability, status, occurred_at,
			observed_at, valid_from, valid_to, last_accessed_at, source_turn_id,
			source_kind, semantic_category, stability_label, merge_key, evidence_count, entities,
			created_at, updated_at
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8::jsonb,
			$9,$10,$11,$12,$13,$14,
			$15,$16,$17,$18,$19,
			$20,$21,$22,$23,$24,$25::jsonb,
			$26,$27
		)
		ON CONFLICT (id)
		DO UPDATE SET
			session_id=EXCLUDED.session_id,
			node_type=EXCLUDED.node_type,
			title=EXCLUDED.title,
			summary=EXCLUDED.summary,
			body_json=EXCLUDED.body_json,
			salience=EXCLUDED.salience,
			affect_weight=EXCLUDED.affect_weight,
			confidence=EXCLUDED.confidence,
			stability=EXCLUDED.stability,
			status=EXCLUDED.status,
			occurred_at=EXCLUDED.occurred_at,
			observed_at=EXCLUDED.observed_at,
			valid_from=EXCLUDED.valid_from,
			valid_to=EXCLUDED.valid_to,
			last_accessed_at=EXCLUDED.last_accessed_at,
			source_turn_id=EXCLUDED.source_turn_id,
			source_kind=EXCLUDED.source_kind,
			semantic_category=EXCLUDED.semantic_category,
			stability_label=EXCLUDED.stability_label,
			merge_key=EXCLUDED.merge_key,
			evidence_count=EXCLUDED.evidence_count,
			entities=EXCLUDED.entities,
			updated_at=EXCLUDED.updated_at
	`, node.ID, node.UserID, node.BotID, nullIfEmpty(node.SessionID), node.NodeType, node.Title, node.Summary,
		normalizeJSONOrObject(node.BodyJSON), node.Salience, node.AffectWeight, node.Confidence, node.Stability,
		node.Status, node.OccurredAt, node.ObservedAt, node.ValidFrom, validTo, node.LastAccessedAt,
		nullIfEmpty(node.SourceTurnID), node.SourceKind, node.SemanticCategory, node.StabilityLabel, node.MergeKey, node.EvidenceCount, string(entities),
		node.CreatedAt, node.UpdatedAt)
	if err != nil {
		log.Printf("upsert memory node failed: %v", err)
	}
	return err
}

func (p *PostgresStore) GetMemoryNode(nodeID string) (TemporalMemoryNode, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	var node TemporalMemoryNode
	var bodyRaw, entitiesRaw []byte
	var validTo sql.NullTime
	err := p.db.QueryRowContext(ctx, `
		SELECT id, user_id, bot_id, COALESCE(session_id::text, ''), node_type, title, summary, body_json,
			salience, affect_weight, confidence, stability, status, occurred_at, observed_at,
			valid_from, valid_to, last_accessed_at, COALESCE(source_turn_id::text, ''),
			COALESCE(source_kind, ''), COALESCE(semantic_category, ''), COALESCE(stability_label, ''), COALESCE(merge_key, ''), evidence_count, entities,
			created_at, updated_at
		FROM memory_nodes
		WHERE id=$1
	`, strings.TrimSpace(nodeID)).Scan(
		&node.ID, &node.UserID, &node.BotID, &node.SessionID, &node.NodeType, &node.Title, &node.Summary, &bodyRaw,
		&node.Salience, &node.AffectWeight, &node.Confidence, &node.Stability, &node.Status, &node.OccurredAt,
		&node.ObservedAt, &node.ValidFrom, &validTo, &node.LastAccessedAt, &node.SourceTurnID,
		&node.SourceKind, &node.SemanticCategory, &node.StabilityLabel, &node.MergeKey, &node.EvidenceCount, &entitiesRaw,
		&node.CreatedAt, &node.UpdatedAt,
	)
	if err == nil {
		hydrateTemporalMemoryNodeRow(&node, bodyRaw, entitiesRaw, validTo)
		return node, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get memory node failed: %v", err)
	}
	return p.Store.GetMemoryNode(nodeID)
}

func (p *PostgresStore) ListMemoryNodes(userID, botID string, filter TemporalMemoryNodeFilter) []TemporalMemoryNode {
	ctx, cancel := p.withTimeout()
	defer cancel()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	query := strings.Builder{}
	query.WriteString(`
		SELECT id, user_id, bot_id, COALESCE(session_id::text, ''), node_type, title, summary, body_json,
			salience, affect_weight, confidence, stability, status, occurred_at, observed_at,
			valid_from, valid_to, last_accessed_at, COALESCE(source_turn_id::text, ''),
			COALESCE(source_kind, ''), COALESCE(semantic_category, ''), COALESCE(stability_label, ''), COALESCE(merge_key, ''), evidence_count, entities,
			created_at, updated_at
		FROM memory_nodes
		WHERE user_id=$1 AND bot_id=$2`)
	args := []any{strings.TrimSpace(userID), strings.TrimSpace(botID)}
	argN := 3
	if status := strings.TrimSpace(filter.Status); status != "" {
		query.WriteString(fmt.Sprintf(" AND status=$%d", argN))
		args = append(args, status)
		argN++
	}
	if sessionID := strings.TrimSpace(filter.SessionID); sessionID != "" {
		query.WriteString(fmt.Sprintf(" AND session_id=$%d", argN))
		args = append(args, sessionID)
		argN++
	}
	if len(filter.NodeTypes) > 0 {
		query.WriteString(fmt.Sprintf(" AND node_type = ANY($%d)", argN))
		args = append(args, filter.NodeTypes)
		argN++
	}
	query.WriteString(" ORDER BY occurred_at DESC, updated_at DESC")
	query.WriteString(fmt.Sprintf(" LIMIT $%d", argN))
	args = append(args, limit)
	rows, err := p.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		log.Printf("list memory nodes failed: %v", err)
		return p.Store.ListMemoryNodes(userID, botID, filter)
	}
	defer rows.Close()
	out := make([]TemporalMemoryNode, 0, limit)
	for rows.Next() {
		var node TemporalMemoryNode
		var bodyRaw, entitiesRaw []byte
		var validTo sql.NullTime
		if scanErr := rows.Scan(
			&node.ID, &node.UserID, &node.BotID, &node.SessionID, &node.NodeType, &node.Title, &node.Summary, &bodyRaw,
			&node.Salience, &node.AffectWeight, &node.Confidence, &node.Stability, &node.Status, &node.OccurredAt,
			&node.ObservedAt, &node.ValidFrom, &validTo, &node.LastAccessedAt, &node.SourceTurnID,
			&node.SourceKind, &node.SemanticCategory, &node.StabilityLabel, &node.MergeKey, &node.EvidenceCount, &entitiesRaw,
			&node.CreatedAt, &node.UpdatedAt,
		); scanErr != nil {
			log.Printf("scan memory node failed: %v", scanErr)
			continue
		}
		hydrateTemporalMemoryNodeRow(&node, bodyRaw, entitiesRaw, validTo)
		out = append(out, node)
	}
	if len(out) == 0 {
		return p.Store.ListMemoryNodes(userID, botID, filter)
	}
	return out
}

func hydrateTemporalMemoryNodeRow(node *TemporalMemoryNode, bodyRaw, entitiesRaw []byte, validTo sql.NullTime) {
	node.BodyJSON = strings.TrimSpace(string(bodyRaw))
	_ = json.Unmarshal(entitiesRaw, &node.Entities)
	if validTo.Valid {
		value := validTo.Time.UTC()
		node.ValidTo = &value
	}
}

func (p *PostgresStore) CreateMemoryEdge(edge TemporalMemoryEdge) (TemporalMemoryEdge, error) {
	saved, err := p.Store.CreateMemoryEdge(edge)
	if err != nil {
		return TemporalMemoryEdge{}, err
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, dbErr := p.db.ExecContext(ctx, `
		INSERT INTO memory_edges(id, source_id, target_id, edge_type, weight, evidence_count, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id)
		DO UPDATE SET source_id=EXCLUDED.source_id, target_id=EXCLUDED.target_id, edge_type=EXCLUDED.edge_type,
			weight=EXCLUDED.weight, evidence_count=EXCLUDED.evidence_count, status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, saved.ID, saved.SourceID, saved.TargetID, saved.EdgeType, saved.Weight, saved.EvidenceCount, saved.Status, saved.CreatedAt, saved.UpdatedAt)
	if dbErr != nil {
		log.Printf("create memory edge failed: %v", dbErr)
	}
	return saved, dbErr
}

func (p *PostgresStore) ListMemoryEdges(nodeIDs []string, filter TemporalMemoryEdgeFilter) []TemporalMemoryEdge {
	ctx, cancel := p.withTimeout()
	defer cancel()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	query := strings.Builder{}
	query.WriteString(`SELECT id, source_id, target_id, edge_type, weight, evidence_count, status, created_at, updated_at FROM memory_edges WHERE 1=1`)
	args := make([]any, 0, 4)
	argN := 1
	if len(nodeIDs) > 0 {
		query.WriteString(fmt.Sprintf(" AND (source_id = ANY($%d) OR target_id = ANY($%d))", argN, argN))
		args = append(args, nodeIDs)
		argN++
	}
	if len(filter.EdgeTypes) > 0 {
		query.WriteString(fmt.Sprintf(" AND edge_type = ANY($%d)", argN))
		args = append(args, filter.EdgeTypes)
		argN++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query.WriteString(fmt.Sprintf(" AND status=$%d", argN))
		args = append(args, status)
		argN++
	}
	query.WriteString(" ORDER BY updated_at DESC, created_at DESC")
	query.WriteString(fmt.Sprintf(" LIMIT $%d", argN))
	args = append(args, limit)
	rows, err := p.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		log.Printf("list memory edges failed: %v", err)
		return p.Store.ListMemoryEdges(nodeIDs, filter)
	}
	defer rows.Close()
	out := make([]TemporalMemoryEdge, 0, limit)
	for rows.Next() {
		var edge TemporalMemoryEdge
		if scanErr := rows.Scan(&edge.ID, &edge.SourceID, &edge.TargetID, &edge.EdgeType, &edge.Weight, &edge.EvidenceCount, &edge.Status, &edge.CreatedAt, &edge.UpdatedAt); scanErr != nil {
			log.Printf("scan memory edge failed: %v", scanErr)
			continue
		}
		out = append(out, edge)
	}
	if len(out) == 0 {
		return p.Store.ListMemoryEdges(nodeIDs, filter)
	}
	return out
}

func (p *PostgresStore) UpsertMemoryCard(card MemoryCard) (MemoryCard, error) {
	saved, err := p.Store.UpsertMemoryCard(card)
	if err != nil {
		return MemoryCard{}, err
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	backingIDs, _ := json.Marshal(saved.BackingNodeIDs)
	_, dbErr := p.db.ExecContext(ctx, `
		INSERT INTO memory_cards(id, user_id, bot_id, card_type, text, backing_node_ids, freshness_score, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9)
		ON CONFLICT (id)
		DO UPDATE SET card_type=EXCLUDED.card_type, text=EXCLUDED.text, backing_node_ids=EXCLUDED.backing_node_ids,
			freshness_score=EXCLUDED.freshness_score, updated_at=EXCLUDED.updated_at
	`, saved.ID, saved.UserID, saved.BotID, saved.CardType, saved.Text, string(backingIDs), saved.FreshnessScore, saved.CreatedAt, saved.UpdatedAt)
	if dbErr != nil {
		log.Printf("upsert memory card failed: %v", dbErr)
	}
	return saved, dbErr
}

func (p *PostgresStore) ListMemoryCards(userID, botID string, filter MemoryCardFilter) []MemoryCard {
	ctx, cancel := p.withTimeout()
	defer cancel()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	query := strings.Builder{}
	query.WriteString(`SELECT id, user_id, bot_id, card_type, text, backing_node_ids, freshness_score, created_at, updated_at FROM memory_cards WHERE user_id=$1 AND bot_id=$2`)
	args := []any{strings.TrimSpace(userID), strings.TrimSpace(botID)}
	argN := 3
	if len(filter.CardTypes) > 0 {
		query.WriteString(fmt.Sprintf(" AND card_type = ANY($%d)", argN))
		args = append(args, filter.CardTypes)
		argN++
	}
	query.WriteString(" ORDER BY updated_at DESC")
	query.WriteString(fmt.Sprintf(" LIMIT $%d", argN))
	args = append(args, limit)
	rows, err := p.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		log.Printf("list memory cards failed: %v", err)
		return p.Store.ListMemoryCards(userID, botID, filter)
	}
	defer rows.Close()
	out := make([]MemoryCard, 0, limit)
	for rows.Next() {
		var card MemoryCard
		var backingRaw []byte
		if scanErr := rows.Scan(&card.ID, &card.UserID, &card.BotID, &card.CardType, &card.Text, &backingRaw, &card.FreshnessScore, &card.CreatedAt, &card.UpdatedAt); scanErr != nil {
			log.Printf("scan memory card failed: %v", scanErr)
			continue
		}
		_ = json.Unmarshal(backingRaw, &card.BackingNodeIDs)
		out = append(out, card)
	}
	if len(out) == 0 {
		return p.Store.ListMemoryCards(userID, botID, filter)
	}
	return out
}

func (p *PostgresStore) SaveActivationTrace(trace ActivationTrace) (ActivationTrace, error) {
	saved, err := p.Store.SaveActivationTrace(trace)
	if err != nil {
		return ActivationTrace{}, err
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	seedIDs, _ := json.Marshal(saved.SeedNodeIDs)
	activatedIDs, _ := json.Marshal(saved.ActivatedNodeIDs)
	cardIDs, _ := json.Marshal(saved.SelectedCardIDs)
	_, dbErr := p.db.ExecContext(ctx, `
		INSERT INTO activation_traces(id, user_id, bot_id, conversation_id, turn_id, cue_json, seed_node_ids, activated_node_ids, selected_card_ids, response_excerpt, created_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9::jsonb,$10,$11)
	`, saved.ID, saved.UserID, saved.BotID, nullIfEmpty(saved.ConversationID), nullIfEmpty(saved.TurnID), normalizeJSONOrObject(saved.CueJSON), string(seedIDs), string(activatedIDs), string(cardIDs), saved.ResponseExcerpt, saved.CreatedAt)
	if dbErr != nil {
		log.Printf("save activation trace failed: %v", dbErr)
	}
	return saved, dbErr
}

func (p *PostgresStore) ListActivationTraces(userID, botID string, filter ActivationTraceFilter) []ActivationTrace {
	ctx, cancel := p.withTimeout()
	defer cancel()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	query := strings.Builder{}
	query.WriteString(`SELECT id, user_id, bot_id, COALESCE(conversation_id::text, ''), COALESCE(turn_id::text, ''), cue_json, seed_node_ids, activated_node_ids, selected_card_ids, response_excerpt, created_at FROM activation_traces WHERE user_id=$1 AND bot_id=$2`)
	args := []any{strings.TrimSpace(userID), strings.TrimSpace(botID)}
	argN := 3
	if conversationID := strings.TrimSpace(filter.ConversationID); conversationID != "" {
		query.WriteString(fmt.Sprintf(" AND conversation_id=$%d", argN))
		args = append(args, conversationID)
		argN++
	}
	if turnID := strings.TrimSpace(filter.TurnID); turnID != "" {
		query.WriteString(fmt.Sprintf(" AND turn_id=$%d", argN))
		args = append(args, turnID)
		argN++
	}
	query.WriteString(" ORDER BY created_at DESC")
	query.WriteString(fmt.Sprintf(" LIMIT $%d", argN))
	args = append(args, limit)
	rows, err := p.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		log.Printf("list activation traces failed: %v", err)
		return p.Store.ListActivationTraces(userID, botID, filter)
	}
	defer rows.Close()
	out := make([]ActivationTrace, 0, limit)
	for rows.Next() {
		var trace ActivationTrace
		var cueRaw, seedRaw, activatedRaw, cardsRaw []byte
		if scanErr := rows.Scan(&trace.ID, &trace.UserID, &trace.BotID, &trace.ConversationID, &trace.TurnID, &cueRaw, &seedRaw, &activatedRaw, &cardsRaw, &trace.ResponseExcerpt, &trace.CreatedAt); scanErr != nil {
			log.Printf("scan activation trace failed: %v", scanErr)
			continue
		}
		trace.CueJSON = strings.TrimSpace(string(cueRaw))
		_ = json.Unmarshal(seedRaw, &trace.SeedNodeIDs)
		_ = json.Unmarshal(activatedRaw, &trace.ActivatedNodeIDs)
		_ = json.Unmarshal(cardsRaw, &trace.SelectedCardIDs)
		out = append(out, trace)
	}
	if len(out) == 0 {
		return p.Store.ListActivationTraces(userID, botID, filter)
	}
	return out
}

func (p *PostgresStore) DeleteMemoryNode(nodeID, userID, botID string) error {
	ctx, cancel := p.withTimeout()
	defer cancel()
	nodeID = strings.TrimSpace(nodeID)
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	result, err := p.db.ExecContext(ctx,
		`DELETE FROM memory_nodes WHERE id = $1 AND user_id = $2 AND bot_id = $3`,
		nodeID, userID, botID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("memory node not found")
	}
	return nil
}

func (p *PostgresStore) UpsertEmbedding(nodeID string, embedding []float64, model string) error {
	if p.db == nil {
		return nil
	}
	ctx, cancel := p.withTimeout()
	defer cancel()

	embJSON, _ := json.Marshal(embedding)
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO memory_embeddings(node_id, embedding_model, embedding_json, updated_at)
		VALUES ($1, $2, $3::jsonb, NOW())
		ON CONFLICT (node_id)
		DO UPDATE SET embedding_model=EXCLUDED.embedding_model, embedding_json=EXCLUDED.embedding_json, updated_at=NOW()
	`, nodeID, model, embJSON)
	return err
}

func (p *PostgresStore) GetEmbedding(nodeID string) ([]float64, string, error) {
	if p.db == nil {
		return nil, "", errors.New("database not configured")
	}
	ctx, cancel := p.withTimeout()
	defer cancel()

	var embJSON []byte
	var model string
	err := p.db.QueryRowContext(ctx, `SELECT embedding_json, embedding_model FROM memory_embeddings WHERE node_id=$1`, nodeID).Scan(&embJSON, &model)
	if err != nil {
		return nil, "", err
	}

	var embedding []float64
	if err := json.Unmarshal(embJSON, &embedding); err != nil {
		return nil, "", err
	}
	return embedding, model, nil
}

func (p *PostgresStore) BatchGetEmbeddings(nodeIDs []string) (map[string][]float64, error) {
	if p.db == nil || len(nodeIDs) == 0 {
		return make(map[string][]float64), nil
	}
	ctx, cancel := p.withTimeout()
	defer cancel()

	idsJSON, _ := json.Marshal(nodeIDs)
	rows, err := p.db.QueryContext(ctx, `SELECT node_id, embedding_json FROM memory_embeddings WHERE node_id = ANY(SELECT jsonb_array_elements_text($1::jsonb))`, idsJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]float64)
	for rows.Next() {
		var nodeID string
		var embJSON []byte
		if err := rows.Scan(&nodeID, &embJSON); err != nil {
			continue
		}
		var embedding []float64
		if err := json.Unmarshal(embJSON, &embedding); err != nil {
			continue
		}
		result[nodeID] = embedding
	}
	return result, nil
}
