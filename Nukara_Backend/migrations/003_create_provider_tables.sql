-- migrations/003_create_provider_tables.sql

-- providers 表
CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    api_key TEXT NOT NULL,
    base_url TEXT NOT NULL,
    models JSONB DEFAULT '[]'::jsonb,
    is_active BOOLEAN DEFAULT false,
    priority INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- provider_health 表
CREATE TABLE IF NOT EXISTS provider_health (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES providers(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,
    latency_ms INT,
    last_check_at TIMESTAMP DEFAULT NOW(),
    error_message TEXT
);

-- provider_usage 表
CREATE TABLE IF NOT EXISTS provider_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES providers(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    request_count INT DEFAULT 0,
    token_count INT DEFAULT 0,
    error_count INT DEFAULT 0,
    UNIQUE(provider_id, date)
);

-- proactive_config 表
CREATE TABLE IF NOT EXISTS proactive_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enabled BOOLEAN DEFAULT true,
    check_interval VARCHAR(10) DEFAULT '5m',
    inactivity_threshold VARCHAR(10) DEFAULT '30m',
    cooldown VARCHAR(10) DEFAULT '60m',
    time_window_start TIME DEFAULT '08:00',
    time_window_end TIME DEFAULT '22:00',
    enabled_message_types JSONB DEFAULT '[
        "morning_care",
        "evening_care",
        "curiosity_after_silence",
        "worry_after_long_silence",
        "random_share"
    ]'::jsonb,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_provider_health_provider_id ON provider_health(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_usage_provider_date ON provider_usage(provider_id, date);

-- 插入默认配置
INSERT INTO proactive_config (id)
SELECT gen_random_uuid()
WHERE NOT EXISTS (SELECT 1 FROM proactive_config);
