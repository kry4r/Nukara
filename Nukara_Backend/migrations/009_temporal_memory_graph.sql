-- migrations/009_temporal_memory_graph.sql

DO $$
BEGIN
    BEGIN
        CREATE EXTENSION IF NOT EXISTS vector;
    EXCEPTION WHEN OTHERS THEN
        NULL;
    END;
END $$;

CREATE TABLE IF NOT EXISTS memory_nodes (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    session_id UUID,
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
    source_turn_id UUID,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_memory_nodes_user_bot ON memory_nodes(user_id, bot_id, status, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_nodes_type ON memory_nodes(user_id, bot_id, node_type, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_nodes_session ON memory_nodes(user_id, bot_id, session_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_edges (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES memory_nodes(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES memory_nodes(id) ON DELETE CASCADE,
    edge_type VARCHAR(40) NOT NULL,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1,
    evidence_count INT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_memory_edges_source ON memory_edges(source_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_edges_target ON memory_edges(target_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_edges_type ON memory_edges(edge_type, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_embeddings (
    node_id UUID PRIMARY KEY REFERENCES memory_nodes(id) ON DELETE CASCADE,
    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
DO $$
BEGIN
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
    conversation_id UUID,
    turn_id UUID,
    cue_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    seed_node_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    activated_node_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    selected_card_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    response_excerpt TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_activation_traces_user_bot ON activation_traces(user_id, bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activation_traces_conversation ON activation_traces(conversation_id, created_at DESC);
