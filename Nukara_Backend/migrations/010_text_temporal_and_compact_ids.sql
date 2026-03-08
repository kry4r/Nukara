-- migrations/010_text_temporal_and_compact_ids.sql
-- Convert business-string identifiers in compact / temporal memory tables from UUID to TEXT.

ALTER TABLE IF EXISTS conversation_compacts
    ALTER COLUMN conversation_id TYPE TEXT USING conversation_id::text,
    ALTER COLUMN until_turn_id TYPE TEXT USING until_turn_id::text;

ALTER TABLE IF EXISTS persona_change_events
    ALTER COLUMN source_turn_id TYPE TEXT USING source_turn_id::text;

ALTER TABLE IF EXISTS memory_embeddings DROP CONSTRAINT IF EXISTS memory_embeddings_node_id_fkey;
ALTER TABLE IF EXISTS memory_edges DROP CONSTRAINT IF EXISTS memory_edges_source_id_fkey;
ALTER TABLE IF EXISTS memory_edges DROP CONSTRAINT IF EXISTS memory_edges_target_id_fkey;

ALTER TABLE IF EXISTS memory_nodes
    ALTER COLUMN id TYPE TEXT USING id::text,
    ALTER COLUMN session_id TYPE TEXT USING session_id::text,
    ALTER COLUMN source_turn_id TYPE TEXT USING source_turn_id::text;

ALTER TABLE IF EXISTS memory_edges
    ALTER COLUMN source_id TYPE TEXT USING source_id::text,
    ALTER COLUMN target_id TYPE TEXT USING target_id::text;

ALTER TABLE IF EXISTS memory_embeddings
    ALTER COLUMN node_id TYPE TEXT USING node_id::text;

DO $$ BEGIN
    ALTER TABLE memory_edges
        ADD CONSTRAINT memory_edges_source_id_fkey FOREIGN KEY (source_id) REFERENCES memory_nodes(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    ALTER TABLE memory_edges
        ADD CONSTRAINT memory_edges_target_id_fkey FOREIGN KEY (target_id) REFERENCES memory_nodes(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    ALTER TABLE memory_embeddings
        ADD CONSTRAINT memory_embeddings_node_id_fkey FOREIGN KEY (node_id) REFERENCES memory_nodes(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE IF EXISTS activation_traces
    ALTER COLUMN conversation_id TYPE TEXT USING conversation_id::text,
    ALTER COLUMN turn_id TYPE TEXT USING turn_id::text;
