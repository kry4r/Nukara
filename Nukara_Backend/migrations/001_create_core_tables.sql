-- migrations/001_create_core_tables.sql
-- Core application tables required by later migrations.

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    phone VARCHAR(20) UNIQUE NOT NULL,
    nickname VARCHAR(50) NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sms_codes (
    id UUID PRIMARY KEY,
    phone VARCHAR(20) NOT NULL,
    purpose VARCHAR(20) NOT NULL,
    code VARCHAR(10) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bots (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    avatar_url TEXT,
    avatar_base64 TEXT,
    summary TEXT NOT NULL,
    speaking_style TEXT NOT NULL,
    background TEXT NOT NULL,
    traits JSONB NOT NULL DEFAULT '[]'::jsonb,
    gender VARCHAR(20) NOT NULL DEFAULT 'unknown',
    chat_background_style VARCHAR(30) NOT NULL DEFAULT 'lightPaper',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bot_states (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    status_emoji VARCHAR(16) NOT NULL DEFAULT '🙂',
    status_text VARCHAR(50) NOT NULL DEFAULT '在线',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, bot_id)
);

CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    last_message TEXT,
    last_message_at TIMESTAMP,
    unread_count INT NOT NULL DEFAULT 0,
    is_proactive_message BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, bot_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL,
    sender_type VARCHAR(10) NOT NULL,
    content_type VARCHAR(20) NOT NULL,
    content JSONB NOT NULL,
    is_proactive BOOLEAN NOT NULL DEFAULT FALSE,
    emotion_tag VARCHAR(30),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_devices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    platform VARCHAR(20) NOT NULL,
    device_token TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, device_token)
);

CREATE TABLE IF NOT EXISTS user_notification_settings (
    user_id UUID PRIMARY KEY,
    proactive_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    dnd_start VARCHAR(5),
    dnd_end VARCHAR(5),
    frequency VARCHAR(20) NOT NULL DEFAULT 'normal',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS proactive_logs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    trigger_type VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    sent_by_ws BOOLEAN NOT NULL DEFAULT TRUE,
    sent_by_apns BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
