-- migrations/008_runtime_profile_state.sql
-- Add runtime living-state persistence and persona change event tracking.

CREATE TABLE IF NOT EXISTS bot_runtime_states (
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    activity_text TEXT NOT NULL,
    basis_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_memory_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, bot_id)
);

CREATE TABLE IF NOT EXISTS persona_change_events (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    field VARCHAR(40) NOT NULL,
    change_type VARCHAR(20) NOT NULL DEFAULT 'append',
    proposed_value TEXT NOT NULL,
    source_turn_id UUID,
    risk VARCHAR(20) NOT NULL DEFAULT 'low',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewer_note TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_persona_change_events_user_bot
    ON persona_change_events(user_id, bot_id, status, created_at DESC);
