-- migrations/004_create_analysis_tables.sql

-- conversation_analysis 表 (renamed from bot_states to avoid conflict)
CREATE TABLE IF NOT EXISTS conversation_analysis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    mood VARCHAR(50) NOT NULL DEFAULT 'neutral',
    mood_intensity INT NOT NULL DEFAULT 5 CHECK (mood_intensity >= 1 AND mood_intensity <= 10),
    impression TEXT NOT NULL DEFAULT '',
    relationship_stage VARCHAR(50) NOT NULL DEFAULT 'stranger',
    interests JSONB NOT NULL DEFAULT '[]'::jsonb,
    topics JSONB NOT NULL DEFAULT '[]'::jsonb,
    message_count INT NOT NULL DEFAULT 0,
    last_analyzed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, bot_id, conversation_id)
);

-- analysis_cache 表
CREATE TABLE IF NOT EXISTS analysis_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    analysis_type VARCHAR(50),
    result JSONB,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- test_config 表 (singleton pattern with CHECK constraint)
CREATE TABLE IF NOT EXISTS test_config (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    auto_verify_enabled BOOLEAN NOT NULL DEFAULT false,
    auto_verify_code VARCHAR(10) NOT NULL DEFAULT '123456',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_conversation_analysis_user_bot ON conversation_analysis(user_id, bot_id);
CREATE INDEX IF NOT EXISTS idx_conversation_analysis_conversation ON conversation_analysis(conversation_id);
CREATE INDEX IF NOT EXISTS idx_cache_lookup ON analysis_cache(user_id, bot_id, analysis_type, expires_at);
CREATE INDEX IF NOT EXISTS idx_cache_expires ON analysis_cache(expires_at) WHERE expires_at IS NOT NULL;

-- 插入默认配置
INSERT INTO test_config (id, auto_verify_enabled, auto_verify_code, updated_at)
VALUES (1, false, '123456', NOW())
ON CONFLICT (id) DO NOTHING;
