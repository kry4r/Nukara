-- migrations/011_memory_persona_redesign_metadata.sql
-- Persist semantic extraction metadata for runtime memory/persona redesign.

ALTER TABLE IF EXISTS memory_items
    ADD COLUMN IF NOT EXISTS semantic_category VARCHAR(40) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS memory_items
    ADD COLUMN IF NOT EXISTS stability VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS memory_items
    ADD COLUMN IF NOT EXISTS entities JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS memory_items
    ADD COLUMN IF NOT EXISTS relations JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS memory_nodes
    ADD COLUMN IF NOT EXISTS source_kind VARCHAR(40) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS memory_nodes
    ADD COLUMN IF NOT EXISTS semantic_category VARCHAR(40) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS memory_nodes
    ADD COLUMN IF NOT EXISTS stability_label VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS memory_nodes
    ADD COLUMN IF NOT EXISTS merge_key TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS memory_nodes
    ADD COLUMN IF NOT EXISTS evidence_count INT NOT NULL DEFAULT 1;

ALTER TABLE IF EXISTS memory_nodes
    ADD COLUMN IF NOT EXISTS entities JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS persona_change_events
    ALTER COLUMN status SET DEFAULT 'accepted';
