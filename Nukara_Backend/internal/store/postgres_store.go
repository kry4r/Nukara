package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

const (
	metricKeyRequests       = "metrics:requests_total"
	metricKeyWS             = "metrics:active_ws_connections"
	metricKeyProactive      = "metrics:proactive_message_sent"
	presenceWSKeyPrefix     = "presence:ws:"
	lastUserMessageAtPrefix = "presence:last_user_message_at:"
)

type PostgresStore struct {
	*Store
	db               *sql.DB
	redis            *redis.Client
	metricsKeyPrefix string
}

func NewPostgresStore(postgresDSN, redisAddr string) (*PostgresStore, error) {
	if strings.TrimSpace(postgresDSN) == "" {
		return nil, errors.New("postgres dsn is empty")
	}

	db, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(80)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensurePostgresSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &PostgresStore{
		Store:            NewStore(),
		db:               db,
		metricsKeyPrefix: "nukara:",
	}
	if strings.TrimSpace(redisAddr) != "" {
		client := redis.NewClient(&redis.Options{Addr: strings.TrimSpace(redisAddr)})
		if err := client.Ping(context.Background()).Err(); err != nil {
			log.Printf("redis unavailable, fallback to in-memory metrics: %v", err)
			_ = client.Close()
		} else {
			store.redis = client
		}
	}
	return store, nil
}

func (p *PostgresStore) Close() {
	if p.redis != nil {
		_ = p.redis.Close()
	}
	if p.db != nil {
		_ = p.db.Close()
	}
}

// DB returns the underlying database connection
func (p *PostgresStore) DB() *sql.DB {
	return p.db
}

// HasRedis returns true if Redis client is available
func (p *PostgresStore) HasRedis() bool {
	if p.redis == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return p.redis.Ping(ctx).Err() == nil
}

func ensurePostgresSchema(ctx context.Context, db *sql.DB) error {
	const schema = `
DO $$
DECLARE
    providers_id_type TEXT;
    provider_health_type TEXT;
    provider_usage_type TEXT;
BEGIN
    SELECT data_type INTO providers_id_type
    FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'providers' AND column_name = 'id';

    IF providers_id_type = 'uuid' THEN
        IF to_regclass('public.provider_health') IS NOT NULL THEN
            EXECUTE 'ALTER TABLE provider_health DROP CONSTRAINT IF EXISTS provider_health_provider_id_fkey';
        END IF;
        IF to_regclass('public.provider_usage') IS NOT NULL THEN
            EXECUTE 'ALTER TABLE provider_usage DROP CONSTRAINT IF EXISTS provider_usage_provider_id_fkey';
        END IF;

        EXECUTE 'ALTER TABLE providers ALTER COLUMN id DROP DEFAULT';
        EXECUTE 'ALTER TABLE providers ALTER COLUMN id TYPE TEXT USING id::text';

        SELECT data_type INTO provider_health_type
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'provider_health' AND column_name = 'provider_id';
        IF provider_health_type = 'uuid' THEN
            EXECUTE 'ALTER TABLE provider_health ALTER COLUMN provider_id TYPE TEXT USING provider_id::text';
        END IF;
        IF to_regclass('public.provider_health') IS NOT NULL THEN
            EXECUTE 'ALTER TABLE provider_health ADD CONSTRAINT provider_health_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE';
        END IF;

        SELECT data_type INTO provider_usage_type
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'provider_usage' AND column_name = 'provider_id';
        IF provider_usage_type = 'uuid' THEN
            EXECUTE 'ALTER TABLE provider_usage ALTER COLUMN provider_id TYPE TEXT USING provider_id::text';
        END IF;
        IF to_regclass('public.provider_usage') IS NOT NULL THEN
            EXECUTE 'ALTER TABLE provider_usage ADD CONSTRAINT provider_usage_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE';
        END IF;
    END IF;
END $$;

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
    relationship TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    self_cognition JSONB NOT NULL DEFAULT '[]'::jsonb,
    persona_prompt TEXT NOT NULL DEFAULT '',
    persona_version INT NOT NULL DEFAULT 1,
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
    reply_group_id VARCHAR(20),
    sequence INT NOT NULL DEFAULT 0,
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

-- Migrations: add reply_group_id and sequence to messages (idempotent)
DO $$ BEGIN
    ALTER TABLE messages ADD COLUMN reply_group_id VARCHAR(20);
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;
DO $$ BEGIN
    ALTER TABLE messages ADD COLUMN sequence INT NOT NULL DEFAULT 0;
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;

-- Migration: add turn_count to bot_states
DO $$ BEGIN
    ALTER TABLE bot_states ADD COLUMN turn_count INT NOT NULL DEFAULT 0;
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;

DO $$ BEGIN
    ALTER TABLE bots ADD COLUMN relationship TEXT NOT NULL DEFAULT '';
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;
DO $$ BEGIN
    ALTER TABLE bots ADD COLUMN role TEXT NOT NULL DEFAULT '';
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;
DO $$ BEGIN
    ALTER TABLE bots ADD COLUMN self_cognition JSONB NOT NULL DEFAULT '[]'::jsonb;
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;
DO $$ BEGIN
    ALTER TABLE bots ADD COLUMN persona_prompt TEXT NOT NULL DEFAULT '';
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;
DO $$ BEGIN
    ALTER TABLE bots ADD COLUMN persona_version INT NOT NULL DEFAULT 1;
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS user_statuses (
    user_id UUID PRIMARY KEY,
    emoji VARCHAR(16) NOT NULL DEFAULT '',
    text VARCHAR(100) NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bot_directives (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    content TEXT NOT NULL,
    category VARCHAR(30) NOT NULL DEFAULT 'style',
    source VARCHAR(30) NOT NULL DEFAULT 'conversation',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    original_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_bot_directives_user_bot ON bot_directives(user_id, bot_id, status);

CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    api_mode TEXT NOT NULL DEFAULT 'chat_completions',
    models JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    priority INT NOT NULL DEFAULT 100,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
DO $$ BEGIN
    ALTER TABLE providers ADD COLUMN api_mode TEXT NOT NULL DEFAULT 'chat_completions';
EXCEPTION WHEN duplicate_column THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS user_provider_settings (
    user_id UUID PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES providers(id),
    model TEXT,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bot_provider_overrides (
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    provider_id TEXT NOT NULL REFERENCES providers(id),
    model TEXT,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, bot_id)
);

CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_turns (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    user_message_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    aggregated_user_text TEXT NOT NULL DEFAULT '',
    bot_reply_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS conversation_compacts (
    conversation_id UUID PRIMARY KEY,
    compact_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    until_turn_id UUID,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS memory_items (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    kind VARCHAR(40) NOT NULL DEFAULT 'fact',
    owner VARCHAR(20) NOT NULL DEFAULT 'user',
    content TEXT NOT NULL,
    importance INT NOT NULL DEFAULT 0,
    occurred_at TIMESTAMP NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    topics JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_memory_items_user_bot ON memory_items(user_id, bot_id, status, occurred_at DESC);

CREATE TABLE IF NOT EXISTS agent_traces (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    conversation_id UUID,
    trace_type VARCHAR(40) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO providers(id, name, api_key, base_url, api_mode, models, is_active, priority)
VALUES (
    'minimax_m2_5',
    'MiniMax M2.5',
    'sk-sp-EVlIgxw4GopFgkn6qlniBjL2n77hoLBK',
    'https://aigw-gzgy2.cucloud.cn:8443/v1',
    'chat_completions',
    '["MiniMax-M2.5"]'::jsonb,
    TRUE,
    1
) ON CONFLICT (id) DO NOTHING;

INSERT INTO system_settings(key, value, updated_at)
VALUES
    ('default_chat_provider_id', '{"value":"minimax_m2_5"}'::jsonb, NOW()),
    ('default_chat_model', '{"value":"MiniMax-M2.5"}'::jsonb, NOW()),
    ('embedding_provider_id', '{"value":"minimax_m2_5"}'::jsonb, NOW()),
    ('embedding_model', '{"value":"MiniMax-M2.5"}'::jsonb, NOW())
ON CONFLICT (key) DO NOTHING;
`
	_, err := db.ExecContext(ctx, schema)
	return err
}

func (p *PostgresStore) withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (p *PostgresStore) metricKey(suffix string) string {
	return p.metricsKeyPrefix + suffix
}

func (p *PostgresStore) presenceWSKey(userID string) string {
	return p.metricKey(presenceWSKeyPrefix + userID)
}

func (p *PostgresStore) lastUserMessageAtKey(userID string) string {
	return p.metricKey(lastUserMessageAtPrefix + userID)
}

func (p *PostgresStore) IncrementRequests() {
	p.Store.IncrementRequests()
	if p.redis == nil {
		return
	}
	if err := p.redis.Incr(context.Background(), p.metricKey(metricKeyRequests)).Err(); err != nil {
		log.Printf("redis incr requests failed: %v", err)
	}
}

func (p *PostgresStore) SetWSConnections(count int) {
	p.Store.SetWSConnections(count)
	if p.redis == nil {
		return
	}
	if err := p.redis.Set(context.Background(), p.metricKey(metricKeyWS), count, 0).Err(); err != nil {
		log.Printf("redis set ws connections failed: %v", err)
	}
}

func (p *PostgresStore) TouchWSPresence(userID string, ttl time.Duration) {
	p.Store.TouchWSPresence(userID, ttl)
	userID = strings.TrimSpace(userID)
	if p.redis == nil || userID == "" {
		return
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	if err := p.redis.Set(context.Background(), p.presenceWSKey(userID), "1", ttl).Err(); err != nil {
		log.Printf("redis set ws presence failed for user=%s: %v", userID, err)
	}
}

func (p *PostgresStore) IsUserWSOnline(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	if p.redis == nil {
		return p.Store.IsUserWSOnline(userID)
	}

	exists, err := p.redis.Exists(context.Background(), p.presenceWSKey(userID)).Result()
	if err != nil {
		log.Printf("redis read ws presence failed for user=%s: %v", userID, err)
		return p.Store.IsUserWSOnline(userID)
	}
	return exists > 0
}

func (p *PostgresStore) SetLastUserMessageAt(userID string, at time.Time) {
	p.Store.SetLastUserMessageAt(userID, at)
	userID = strings.TrimSpace(userID)
	if p.redis == nil || userID == "" || at.IsZero() {
		return
	}
	if err := p.redis.Set(context.Background(), p.lastUserMessageAtKey(userID), at.UTC().Unix(), 0).Err(); err != nil {
		log.Printf("redis set last user message at failed for user=%s: %v", userID, err)
	}
}

func (p *PostgresStore) GetLastUserMessageAt(userID string) (time.Time, bool) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return time.Time{}, false
	}
	if p.redis == nil {
		return p.Store.GetLastUserMessageAt(userID)
	}

	value, err := p.redis.Get(context.Background(), p.lastUserMessageAtKey(userID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return time.Time{}, false
		}
		log.Printf("redis get last user message at failed for user=%s: %v", userID, err)
		return p.Store.GetLastUserMessageAt(userID)
	}

	unixValue, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if parseErr != nil {
		log.Printf("redis parse last user message at failed for user=%s value=%q: %v", userID, value, parseErr)
		return p.Store.GetLastUserMessageAt(userID)
	}
	return time.Unix(unixValue, 0).UTC(), true
}

func (p *PostgresStore) SnapshotMetrics() Metrics {
	metrics := p.Store.SnapshotMetrics()
	if p.redis == nil {
		return metrics
	}

	ctx := context.Background()
	if value, err := p.redis.Get(ctx, p.metricKey(metricKeyRequests)).Int(); err == nil {
		metrics.RequestsTotal = value
	}
	if value, err := p.redis.Get(ctx, p.metricKey(metricKeyWS)).Int(); err == nil {
		metrics.ActiveWSConnections = value
	}
	if value, err := p.redis.Get(ctx, p.metricKey(metricKeyProactive)).Int(); err == nil {
		metrics.ProactiveSentTotal = value
	}
	return metrics
}

func (p *PostgresStore) SaveSMSCode(phone, purpose, code string, ttl time.Duration) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO sms_codes(id, phone, purpose, code, expires_at, used, created_at)
		 VALUES($1,$2,$3,$4,$5,FALSE,NOW())`,
		NewID(), phone, purpose, code, time.Now().UTC().Add(ttl),
	)
	if err != nil {
		log.Printf("save sms code to postgres failed: %v", err)
	}
}

func (p *PostgresStore) ValidateSMSCode(phone, purpose, code string) bool {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var id string
	var dbCode string
	var expiresAt time.Time
	err := p.db.QueryRowContext(ctx,
		`SELECT id, code, expires_at
		 FROM sms_codes
		 WHERE phone=$1 AND purpose=$2 AND used=FALSE
		 ORDER BY created_at DESC
		 LIMIT 1`,
		phone, purpose,
	).Scan(&id, &dbCode, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p.Store.ValidateSMSCode(phone, purpose, code)
		}
		log.Printf("validate sms query failed: %v", err)
		return p.Store.ValidateSMSCode(phone, purpose, code)
	}
	if dbCode != code || time.Now().UTC().After(expiresAt) {
		return false
	}
	_, _ = p.db.ExecContext(ctx, `UPDATE sms_codes SET used=TRUE WHERE id=$1`, id)
	return true
}

func (p *PostgresStore) FindUserByPhone(phone string) (User, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var user User
	var avatar sql.NullString
	err := p.db.QueryRowContext(ctx,
		`SELECT id, phone, nickname, avatar_url, created_at
		 FROM users
		 WHERE phone=$1`, phone,
	).Scan(&user.ID, &user.Phone, &user.Nickname, &avatar, &user.CreatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("find user by phone failed: %v", err)
		}
		return p.Store.FindUserByPhone(phone)
	}
	user.Avatar = avatar.String
	return user, true
}

func (p *PostgresStore) CreateUser(phone, nickname string) (User, error) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	id := NewID()
	now := time.Now().UTC()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return p.Store.CreateUser(phone, nickname)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users(id, phone, nickname, created_at, updated_at)
		 VALUES($1,$2,$3,$4,$4)`,
		id, phone, nickname, now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return User{}, errors.New("phone already registered")
		}
		return p.Store.CreateUser(phone, nickname)
	}

	_, _ = tx.ExecContext(ctx,
		`INSERT INTO user_notification_settings(user_id, proactive_enabled, frequency, updated_at)
		 VALUES($1, TRUE, 'normal', $2)
		 ON CONFLICT (user_id) DO UPDATE SET updated_at = EXCLUDED.updated_at`,
		id, now,
	)

	if err := tx.Commit(); err != nil {
		return p.Store.CreateUser(phone, nickname)
	}

	created := User{ID: id, Phone: phone, Nickname: nickname, CreatedAt: now}
	return created, nil
}

func (p *PostgresStore) UpsertDeviceToken(userID, token, platform string) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	now := time.Now().UTC()
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO user_devices(id, user_id, platform, device_token, is_active, created_at, updated_at)
		 VALUES($1,$2,$3,$4,TRUE,$5,$5)
		 ON CONFLICT (user_id, device_token)
		 DO UPDATE SET platform=EXCLUDED.platform, is_active=TRUE, updated_at=EXCLUDED.updated_at`,
		NewID(), userID, platform, token, now,
	)
	if err != nil {
		log.Printf("upsert device token failed: %v", err)
	}
}

func (p *PostgresStore) GetDeviceToken(userID string) (DeviceToken, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var out DeviceToken
	err := p.db.QueryRowContext(ctx,
		`SELECT user_id, device_token, platform, updated_at
		 FROM user_devices
		 WHERE user_id=$1 AND is_active=TRUE
		 ORDER BY updated_at DESC
		 LIMIT 1`, userID,
	).Scan(&out.UserID, &out.Token, &out.Platform, &out.UpdatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("get device token failed: %v", err)
		}
		return p.Store.GetDeviceToken(userID)
	}
	return out, true
}

func (p *PostgresStore) UpdateNotificationSettings(userID string, input NotificationSettings) NotificationSettings {
	ctx, cancel := p.withTimeout()
	defer cancel()

	if strings.TrimSpace(input.Frequency) == "" {
		input.Frequency = "normal"
	}
	input.UserID = userID
	input.UpdatedAt = time.Now().UTC()

	_, err := p.db.ExecContext(ctx,
		`INSERT INTO user_notification_settings(user_id, proactive_enabled, dnd_start, dnd_end, frequency, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (user_id)
		 DO UPDATE SET proactive_enabled=EXCLUDED.proactive_enabled,
		               dnd_start=EXCLUDED.dnd_start,
		               dnd_end=EXCLUDED.dnd_end,
		               frequency=EXCLUDED.frequency,
		               updated_at=EXCLUDED.updated_at`,
		input.UserID, input.ProactiveEnabled, nullIfEmpty(input.DNDStart), nullIfEmpty(input.DNDEnd), input.Frequency, input.UpdatedAt,
	)
	if err != nil {
		log.Printf("update notification settings failed: %v", err)
	}
	return input
}

func (p *PostgresStore) GetNotificationSettings(userID string) NotificationSettings {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var out NotificationSettings
	var dndStart, dndEnd sql.NullString
	err := p.db.QueryRowContext(ctx,
		`SELECT user_id, proactive_enabled, dnd_start, dnd_end, frequency, updated_at
		 FROM user_notification_settings
		 WHERE user_id=$1`, userID,
	).Scan(&out.UserID, &out.ProactiveEnabled, &dndStart, &dndEnd, &out.Frequency, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NotificationSettings{UserID: userID, ProactiveEnabled: true, Frequency: "normal", UpdatedAt: time.Now().UTC()}
		}
		log.Printf("get notification settings failed: %v", err)
		return p.Store.GetNotificationSettings(userID)
	}
	out.DNDStart = dndStart.String
	out.DNDEnd = dndEnd.String
	if strings.TrimSpace(out.Frequency) == "" {
		out.Frequency = "normal"
	}
	return out
}

func (p *PostgresStore) CreateBot(userID string, bot Bot) Bot {
	if bot.ChatBackgroundStyle == "" {
		bot.ChatBackgroundStyle = "lightPaper"
	}
	if bot.Gender == "" {
		bot.Gender = "unknown"
	}
	if strings.TrimSpace(bot.Relationship) == "" {
		bot.Relationship = strings.TrimSpace(bot.Summary)
	}
	if strings.TrimSpace(bot.Role) == "" {
		bot.Role = strings.TrimSpace(bot.Background)
	}
	if bot.PersonaVersion <= 0 {
		bot.PersonaVersion = 1
	}

	now := time.Now().UTC()
	bot.ID = NewID()
	bot.UserID = userID
	bot.CreatedAt = now
	bot.UpdatedAt = now

	traitsRaw, _ := json.Marshal(bot.Traits)

	ctx, cancel := p.withTimeout()
	defer cancel()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return p.Store.CreateBot(userID, bot)
	}
	defer func() { _ = tx.Rollback() }()

	selfCognitionRaw, _ := json.Marshal(bot.SelfCognition)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO bots(id, user_id, name, avatar_url, avatar_base64, summary, relationship, role, self_cognition, persona_prompt, persona_version, speaking_style, background, traits, gender, chat_background_style, created_at, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$17)`,
		bot.ID, userID, bot.Name, nullIfEmpty(bot.Avatar), nullIfEmpty(bot.AvatarBase64), bot.Summary, bot.Relationship, bot.Role,
		selfCognitionRaw, bot.PersonaPrompt, bot.PersonaVersion, bot.SpeakingStyle, bot.Background, traitsRaw, bot.Gender, bot.ChatBackgroundStyle, now,
	)
	if err != nil {
		log.Printf("insert bot failed: %v", err)
		return p.Store.CreateBot(userID, bot)
	}

	_, _ = tx.ExecContext(ctx,
		`INSERT INTO bot_states(id, user_id, bot_id, status_emoji, status_text, updated_at)
		 VALUES($1,$2,$3,'🙂','在线',$4)
		 ON CONFLICT (user_id, bot_id)
		 DO UPDATE SET status_emoji=EXCLUDED.status_emoji, status_text=EXCLUDED.status_text, updated_at=EXCLUDED.updated_at`,
		NewID(), userID, bot.ID, now,
	)

	convID := NewID()
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO conversations(id, user_id, bot_id, last_message, last_message_at, unread_count, is_proactive_message, created_at, updated_at)
		 VALUES($1,$2,$3,'',$4,0,FALSE,$4,$4)
		 ON CONFLICT (user_id, bot_id) DO NOTHING`,
		convID, userID, bot.ID, now,
	)

	if err := tx.Commit(); err != nil {
		log.Printf("commit create bot failed: %v", err)
		return p.Store.CreateBot(userID, bot)
	}

	return bot
}

func (p *PostgresStore) ListBots(userID string) []Bot {
	ctx, cancel := p.withTimeout()
	defer cancel()

	rows, err := p.db.QueryContext(ctx,
		`SELECT id, user_id, name, avatar_url, avatar_base64, summary, relationship, role, self_cognition, persona_prompt, persona_version, speaking_style, background, traits, gender, chat_background_style, created_at, updated_at
		 FROM bots
		 WHERE user_id=$1
		 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		log.Printf("list bots failed: %v", err)
		return p.Store.ListBots(userID)
	}
	defer rows.Close()

	out := []Bot{}
	for rows.Next() {
		bot, ok := scanBotRow(rows)
		if ok {
			out = append(out, bot)
		}
	}
	if len(out) == 0 {
		return p.Store.ListBots(userID)
	}
	return out
}

func (p *PostgresStore) GetBot(userID, botID string) (Bot, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	row := p.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, avatar_url, avatar_base64, summary, relationship, role, self_cognition, persona_prompt, persona_version, speaking_style, background, traits, gender, chat_background_style, created_at, updated_at
		 FROM bots
		 WHERE user_id=$1 AND id=$2`, userID, botID,
	)
	bot, ok := scanBotSingleRow(row)
	if !ok {
		return p.Store.GetBot(userID, botID)
	}
	return bot, true
}

func (p *PostgresStore) GetBotState(userID, botID string) (BotState, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	var state BotState
	err := p.db.QueryRowContext(ctx,
		`SELECT user_id, bot_id, status_emoji, status_text, turn_count, updated_at
		 FROM bot_states
		 WHERE user_id=$1 AND bot_id=$2`, userID, botID,
	).Scan(&state.UserID, &state.BotID, &state.StatusEmoji, &state.StatusText, &state.TurnCount, &state.UpdatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("get bot state failed: %v", err)
		}
		return p.Store.GetBotState(userID, botID)
	}
	return state, true
}

func (p *PostgresStore) UpdateBot(userID, botID string, patch Bot) (Bot, bool) {
	bot, found := p.GetBot(userID, botID)
	if !found {
		return Bot{}, false
	}

	if strings.TrimSpace(patch.Name) != "" {
		bot.Name = strings.TrimSpace(patch.Name)
	}
	if patch.Summary != "" {
		bot.Summary = strings.TrimSpace(patch.Summary)
		bot.Relationship = strings.TrimSpace(patch.Summary)
	}
	if patch.Relationship != "" {
		bot.Relationship = strings.TrimSpace(patch.Relationship)
	}
	if patch.SpeakingStyle != "" {
		bot.SpeakingStyle = strings.TrimSpace(patch.SpeakingStyle)
	}
	if patch.Background != "" {
		bot.Background = strings.TrimSpace(patch.Background)
		bot.Role = strings.TrimSpace(patch.Background)
	}
	if patch.Role != "" {
		bot.Role = strings.TrimSpace(patch.Role)
	}
	if patch.Gender != "" {
		bot.Gender = patch.Gender
	}
	if len(patch.Traits) > 0 {
		bot.Traits = patch.Traits
	}
	bot.UpdatedAt = time.Now().UTC()

	traitsRaw, _ := json.Marshal(bot.Traits)
	selfCognitionRaw, _ := json.Marshal(bot.SelfCognition)
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`UPDATE bots
		 SET name=$1, summary=$2, relationship=$3, role=$4, self_cognition=$5, persona_prompt=$6, persona_version=$7,
		     speaking_style=$8, background=$9, traits=$10, gender=$11, updated_at=$12
		 WHERE user_id=$13 AND id=$14`,
		bot.Name, bot.Summary, bot.Relationship, bot.Role, selfCognitionRaw, bot.PersonaPrompt, bot.PersonaVersion,
		bot.SpeakingStyle, bot.Background, traitsRaw, bot.Gender, bot.UpdatedAt, userID, botID,
	)
	if err != nil {
		log.Printf("update bot failed: %v", err)
		return p.Store.UpdateBot(userID, botID, patch)
	}
	return bot, true
}

func (p *PostgresStore) AppendBotPersona(userID, botID string, speakingAdds, backgroundAdds, traitAdds []string, gender *string) (Bot, bool) {
	bot, found := p.GetBot(userID, botID)
	if !found {
		return Bot{}, false
	}

	bot.SpeakingStyle = strings.Join(dedup(append(splitSegments(bot.SpeakingStyle), speakingAdds...)), "|")
	bot.Background = strings.Join(dedup(append(splitSegments(bot.Background), backgroundAdds...)), "|")
	bot.Role = bot.Background
	bot.Traits = dedup(append(bot.Traits, traitAdds...))
	if gender != nil && strings.TrimSpace(*gender) != "" {
		bot.Gender = strings.TrimSpace(*gender)
	}
	bot.UpdatedAt = time.Now().UTC()

	traitsRaw, _ := json.Marshal(bot.Traits)
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`UPDATE bots
		 SET speaking_style=$1, background=$2, role=$3, traits=$4, gender=$5, updated_at=$6
		 WHERE user_id=$7 AND id=$8`,
		bot.SpeakingStyle, bot.Background, bot.Role, traitsRaw, bot.Gender, bot.UpdatedAt, userID, botID,
	)
	if err != nil {
		log.Printf("append bot persona update failed: %v", err)
		return p.Store.AppendBotPersona(userID, botID, speakingAdds, backgroundAdds, traitAdds, gender)
	}
	return bot, true
}

func (p *PostgresStore) ApplyBotPersonaPatch(userID, botID string, input PersonaPatchInput) (Bot, bool) {
	bot, found := p.Store.ApplyBotPersonaPatch(userID, botID, input)
	if !found {
		return Bot{}, false
	}

	selfCognitionRaw, _ := json.Marshal(bot.SelfCognition)
	traitsRaw, _ := json.Marshal(bot.Traits)
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx, `
		UPDATE bots
		SET summary=$1, relationship=$2, role=$3, self_cognition=$4, persona_prompt=$5, persona_version=$6,
		    speaking_style=$7, background=$8, traits=$9, gender=$10, updated_at=$11
		WHERE user_id=$12 AND id=$13
	`, bot.Summary, bot.Relationship, bot.Role, selfCognitionRaw, bot.PersonaPrompt, bot.PersonaVersion,
		bot.SpeakingStyle, bot.Background, traitsRaw, bot.Gender, bot.UpdatedAt, userID, botID)
	if err != nil {
		log.Printf("apply bot persona patch failed: %v", err)
		return p.Store.ApplyBotPersonaPatch(userID, botID, input)
	}
	return bot, true
}

func (p *PostgresStore) ListConversations(userID string) []Conversation {
	ctx, cancel := p.withTimeout()
	defer cancel()

	rows, err := p.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, c.bot_id, b.name, COALESCE(b.avatar_url,''), COALESCE(b.avatar_base64,''),
		        COALESCE(c.last_message,''), COALESCE(c.last_message_at, NOW()), c.unread_count, c.is_proactive_message
		 FROM conversations c
		 JOIN bots b ON b.id = c.bot_id
		 WHERE c.user_id=$1
		 ORDER BY c.last_message_at DESC NULLS LAST, c.updated_at DESC`, userID,
	)
	if err != nil {
		log.Printf("list conversations failed: %v", err)
		return p.Store.ListConversations(userID)
	}
	defer rows.Close()

	out := []Conversation{}
	for rows.Next() {
		conv, ok := scanConversationRow(rows)
		if ok {
			out = append(out, conv)
		}
	}
	if len(out) == 0 {
		return p.Store.ListConversations(userID)
	}
	return out
}

func (p *PostgresStore) FindConversationByBot(userID, botID string) (Conversation, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()

	row := p.db.QueryRowContext(ctx,
		`SELECT c.id, c.user_id, c.bot_id, b.name, COALESCE(b.avatar_url,''), COALESCE(b.avatar_base64,''),
		        COALESCE(c.last_message,''), COALESCE(c.last_message_at, NOW()), c.unread_count, c.is_proactive_message
		 FROM conversations c
		 JOIN bots b ON b.id = c.bot_id
		 WHERE c.user_id=$1 AND c.bot_id=$2
		 LIMIT 1`, userID, botID,
	)
	conv, ok := scanConversationSingleRow(row)
	if !ok {
		return p.Store.FindConversationByBot(userID, botID)
	}
	return conv, true
}

func (p *PostgresStore) EnsureConversation(userID, botID, botName, botAvatar, botAvatarBase64 string) Conversation {
	if conv, found := p.FindConversationByBot(userID, botID); found {
		return conv
	}

	now := time.Now().UTC()
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, _ = p.db.ExecContext(ctx,
		`INSERT INTO conversations(id, user_id, bot_id, last_message, last_message_at, unread_count, is_proactive_message, created_at, updated_at)
		 VALUES($1,$2,$3,'',$4,0,FALSE,$4,$4)
		 ON CONFLICT (user_id, bot_id) DO NOTHING`,
		NewID(), userID, botID, now,
	)
	if conv, found := p.FindConversationByBot(userID, botID); found {
		return conv
	}
	return p.Store.EnsureConversation(userID, botID, botName, botAvatar, botAvatarBase64)
}

func (p *PostgresStore) GetConversation(userID, conversationID string) (Conversation, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	row := p.db.QueryRowContext(ctx,
		`SELECT c.id, c.user_id, c.bot_id, b.name, COALESCE(b.avatar_url,''), COALESCE(b.avatar_base64,''),
		        COALESCE(c.last_message,''), COALESCE(c.last_message_at, NOW()), c.unread_count, c.is_proactive_message
		 FROM conversations c
		 JOIN bots b ON b.id = c.bot_id
		 WHERE c.user_id=$1 AND c.id=$2`, userID, conversationID,
	)
	conv, ok := scanConversationSingleRow(row)
	if !ok {
		return p.Store.GetConversation(userID, conversationID)
	}
	return conv, true
}

func (p *PostgresStore) ListMessages(userID, conversationID string, limit int) ([]Message, bool) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	if _, found := p.GetConversation(userID, conversationID); !found {
		return nil, false
	}

	ctx, cancel := p.withTimeout()
	defer cancel()
	rows, err := p.db.QueryContext(ctx,
		`SELECT id, conversation_id, sender_type, content_type, content, is_proactive, COALESCE(emotion_tag,''), COALESCE(reply_group_id,''), sequence, created_at
		 FROM messages
		 WHERE conversation_id=$1
		 ORDER BY created_at DESC
		 LIMIT $2`, conversationID, limit,
	)
	if err != nil {
		log.Printf("list messages failed: %v", err)
		return p.Store.ListMessages(userID, conversationID, limit)
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var msg Message
		var contentRaw []byte
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderType, &msg.ContentType, &contentRaw, &msg.IsProactive, &msg.EmotionTag, &msg.ReplyGroupID, &msg.Sequence, &msg.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(contentRaw, &msg.Content)
		messages = append(messages, msg)
	}

	sort.Slice(messages, func(i, j int) bool { return messages[i].CreatedAt.Before(messages[j].CreatedAt) })
	return messages, true
}

func (p *PostgresStore) MarkConversationRead(userID, conversationID string) bool {
	ctx, cancel := p.withTimeout()
	defer cancel()
	result, err := p.db.ExecContext(ctx,
		`UPDATE conversations
		 SET unread_count=0, updated_at=$1
		 WHERE user_id=$2 AND id=$3`,
		time.Now().UTC(), userID, conversationID,
	)
	if err != nil {
		log.Printf("mark conversation read failed: %v", err)
		return p.Store.MarkConversationRead(userID, conversationID)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return false
	}
	return true
}

func (p *PostgresStore) SaveMessage(userID string, message Message) (Message, bool) {
	conv, found := p.GetConversation(userID, message.ConversationID)
	if !found {
		return Message{}, false
	}

	now := time.Now().UTC()
	if strings.TrimSpace(message.ID) == "" {
		message.ID = NewID()
	}
	message.CreatedAt = now
	if strings.TrimSpace(message.ContentType) == "" {
		message.ContentType = message.Content.Type
	}

	contentRaw, _ := json.Marshal(message.Content)
	ctx, cancel := p.withTimeout()
	defer cancel()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return p.Store.SaveMessage(userID, message)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO messages(id, conversation_id, sender_type, content_type, content, is_proactive, emotion_tag, reply_group_id, sequence, created_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		message.ID, message.ConversationID, message.SenderType, message.ContentType, contentRaw, message.IsProactive, nullIfEmpty(message.EmotionTag), nullIfEmpty(message.ReplyGroupID), message.Sequence, message.CreatedAt,
	)
	if err != nil {
		log.Printf("save message insert failed: %v", err)
		return p.Store.SaveMessage(userID, message)
	}

	unreadDelta := 0
	if message.SenderType == "bot" {
		unreadDelta = 1
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE conversations
		 SET last_message=$1,
		     last_message_at=$2,
		     is_proactive_message=$3,
		     unread_count=GREATEST(0, unread_count + $4),
		     updated_at=$2
		 WHERE id=$5 AND user_id=$6`,
		previewText(message), message.CreatedAt, message.IsProactive, unreadDelta, conv.ID, userID,
	)
	if err != nil {
		log.Printf("save message conversation update failed: %v", err)
		return p.Store.SaveMessage(userID, message)
	}

	if err := tx.Commit(); err != nil {
		return p.Store.SaveMessage(userID, message)
	}
	return message, true
}

func (p *PostgresStore) SaveBotStatus(userID, botID, emoji, text string) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO bot_states(id, user_id, bot_id, status_emoji, status_text, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (user_id, bot_id)
		 DO UPDATE SET status_emoji=EXCLUDED.status_emoji, status_text=EXCLUDED.status_text, updated_at=EXCLUDED.updated_at`,
		NewID(), userID, botID, emoji, text, time.Now().UTC(),
	)
	if err != nil {
		log.Printf("save bot status failed: %v", err)
	}
}

func (p *PostgresStore) IncrementTurnCount(userID, botID string) int {
	ctx, cancel := p.withTimeout()
	defer cancel()
	var count int
	err := p.db.QueryRowContext(ctx,
		`INSERT INTO bot_states(id, user_id, bot_id, turn_count, updated_at)
		 VALUES($1,$2,$3,1,$4)
		 ON CONFLICT (user_id, bot_id)
		 DO UPDATE SET turn_count=bot_states.turn_count+1, updated_at=EXCLUDED.updated_at
		 RETURNING turn_count`,
		NewID(), userID, botID, time.Now().UTC(),
	).Scan(&count)
	if err != nil {
		log.Printf("increment turn count failed: %v", err)
		return p.Store.IncrementTurnCount(userID, botID)
	}
	return count
}

func (p *PostgresStore) SaveUserStatus(userID, emoji, text string) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO user_statuses(user_id, emoji, text, updated_at)
		 VALUES($1,$2,$3,$4)
		 ON CONFLICT (user_id)
		 DO UPDATE SET emoji=EXCLUDED.emoji, text=EXCLUDED.text, updated_at=EXCLUDED.updated_at`,
		userID, emoji, text, time.Now().UTC(),
	)
	if err != nil {
		log.Printf("save user status failed: %v", err)
	}
}

func (p *PostgresStore) GetUserStatus(userID string) (UserStatus, bool) {
	ctx, cancel := p.withTimeout()
	defer cancel()
	var st UserStatus
	err := p.db.QueryRowContext(ctx,
		`SELECT user_id, emoji, text, updated_at FROM user_statuses WHERE user_id=$1`, userID,
	).Scan(&st.UserID, &st.Emoji, &st.Text, &st.UpdatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("get user status failed: %v", err)
		}
		return p.Store.GetUserStatus(userID)
	}
	return st, true
}

func (p *PostgresStore) AddProactiveLog(entry ProactiveLog) ProactiveLog {
	entry.ID = NewID()
	entry.CreatedAt = time.Now().UTC()

	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO proactive_logs(id, user_id, bot_id, conversation_id, trigger_type, message, sent_by_ws, sent_by_apns, created_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		entry.ID, entry.UserID, entry.BotID, entry.ConversationID, entry.TriggerType, entry.Message, entry.SentByWS, entry.SentByAPNs, entry.CreatedAt,
	)
	if err != nil {
		log.Printf("add proactive log failed: %v", err)
		fallback := p.Store.AddProactiveLog(entry)
		p.bumpProactiveMetric()
		return fallback
	}

	p.bumpProactiveMetric()
	return entry
}

func (p *PostgresStore) ListProactiveLogs(userID string, limit int) []ProactiveLog {
	if limit <= 0 {
		limit = 20
	}
	ctx, cancel := p.withTimeout()
	defer cancel()

	base := `SELECT id, user_id, conversation_id, bot_id, trigger_type, message, sent_by_ws, sent_by_apns, created_at
		FROM proactive_logs`
	args := []any{}
	if strings.TrimSpace(userID) != "" {
		base += ` WHERE user_id=$1`
		args = append(args, userID)
		base += ` ORDER BY created_at DESC LIMIT $2`
		args = append(args, limit)
	} else {
		base += ` ORDER BY created_at DESC LIMIT $1`
		args = append(args, limit)
	}

	rows, err := p.db.QueryContext(ctx, base, args...)
	if err != nil {
		log.Printf("list proactive logs failed: %v", err)
		return p.Store.ListProactiveLogs(userID, limit)
	}
	defer rows.Close()

	out := make([]ProactiveLog, 0, limit)
	for rows.Next() {
		var logEntry ProactiveLog
		if err := rows.Scan(&logEntry.ID, &logEntry.UserID, &logEntry.ConversationID, &logEntry.BotID, &logEntry.TriggerType, &logEntry.Message, &logEntry.SentByWS, &logEntry.SentByAPNs, &logEntry.CreatedAt); err != nil {
			continue
		}
		out = append(out, logEntry)
	}
	if len(out) == 0 {
		return p.Store.ListProactiveLogs(userID, limit)
	}
	return out
}

func (p *PostgresStore) SaveDirective(d Directive) Directive {
	d.ID = NewID()
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	if d.Status == "" {
		d.Status = "active"
	}
	ctx, cancel := p.withTimeout()
	defer cancel()
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO bot_directives(id,user_id,bot_id,content,category,source,status,original_message,created_at,updated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`,
		d.ID, d.UserID, d.BotID, d.Content, d.Category, d.Source, d.Status, nullIfEmpty(d.OriginalMessage), now,
	)
	if err != nil {
		log.Printf("save directive failed: %v", err)
	}
	return d
}

func (p *PostgresStore) ListDirectives(userID, botID, status string) []Directive {
	ctx, cancel := p.withTimeout()
	defer cancel()
	q := `SELECT id,bot_id,content,category,source,status,COALESCE(original_message,''),created_at,updated_at
	      FROM bot_directives WHERE user_id=$1 AND bot_id=$2`
	args := []any{userID, botID}
	if status != "" {
		q += ` AND status=$3`
		args = append(args, status)
	}
	q += ` ORDER BY created_at`
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		log.Printf("list directives failed: %v", err)
		return p.Store.ListDirectives(userID, botID, status)
	}
	defer rows.Close()
	var out []Directive
	for rows.Next() {
		var d Directive
		d.UserID = userID
		if err := rows.Scan(&d.ID, &d.BotID, &d.Content, &d.Category, &d.Source, &d.Status, &d.OriginalMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}

func (p *PostgresStore) RevokeDirective(userID, botID, directiveID string) bool {
	ctx, cancel := p.withTimeout()
	defer cancel()
	res, err := p.db.ExecContext(ctx,
		`UPDATE bot_directives SET status='revoked',updated_at=$1 WHERE id=$2 AND user_id=$3 AND bot_id=$4`,
		time.Now().UTC(), directiveID, userID, botID,
	)
	if err != nil {
		log.Printf("revoke directive failed: %v", err)
		return p.Store.RevokeDirective(userID, botID, directiveID)
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (p *PostgresStore) bumpProactiveMetric() {
	if p.redis != nil {
		if err := p.redis.Incr(context.Background(), p.metricKey(metricKeyProactive)).Err(); err != nil {
			log.Printf("redis incr proactive metric failed: %v", err)
		}
	}
}

func scanBotRow(rows *sql.Rows) (Bot, bool) {
	var bot Bot
	var avatarURL, avatarBase64 sql.NullString
	var traitsRaw, selfCognitionRaw []byte
	if err := rows.Scan(
		&bot.ID,
		&bot.UserID,
		&bot.Name,
		&avatarURL,
		&avatarBase64,
		&bot.Summary,
		&bot.Relationship,
		&bot.Role,
		&selfCognitionRaw,
		&bot.PersonaPrompt,
		&bot.PersonaVersion,
		&bot.SpeakingStyle,
		&bot.Background,
		&traitsRaw,
		&bot.Gender,
		&bot.ChatBackgroundStyle,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	); err != nil {
		return Bot{}, false
	}
	bot.Avatar = avatarURL.String
	bot.AvatarBase64 = avatarBase64.String
	_ = json.Unmarshal(traitsRaw, &bot.Traits)
	_ = json.Unmarshal(selfCognitionRaw, &bot.SelfCognition)
	return bot, true
}

func scanBotSingleRow(row *sql.Row) (Bot, bool) {
	var bot Bot
	var avatarURL, avatarBase64 sql.NullString
	var traitsRaw, selfCognitionRaw []byte
	if err := row.Scan(
		&bot.ID,
		&bot.UserID,
		&bot.Name,
		&avatarURL,
		&avatarBase64,
		&bot.Summary,
		&bot.Relationship,
		&bot.Role,
		&selfCognitionRaw,
		&bot.PersonaPrompt,
		&bot.PersonaVersion,
		&bot.SpeakingStyle,
		&bot.Background,
		&traitsRaw,
		&bot.Gender,
		&bot.ChatBackgroundStyle,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	); err != nil {
		return Bot{}, false
	}
	bot.Avatar = avatarURL.String
	bot.AvatarBase64 = avatarBase64.String
	_ = json.Unmarshal(traitsRaw, &bot.Traits)
	_ = json.Unmarshal(selfCognitionRaw, &bot.SelfCognition)
	return bot, true
}

func scanConversationRow(rows *sql.Rows) (Conversation, bool) {
	var conv Conversation
	if err := rows.Scan(
		&conv.ID,
		&conv.UserID,
		&conv.BotID,
		&conv.BotName,
		&conv.BotAvatar,
		&conv.BotAvatarBase64,
		&conv.LastMessage,
		&conv.LastMessageAt,
		&conv.UnreadCount,
		&conv.IsProactiveMessage,
	); err != nil {
		return Conversation{}, false
	}
	return conv, true
}

func scanConversationSingleRow(row *sql.Row) (Conversation, bool) {
	var conv Conversation
	if err := row.Scan(
		&conv.ID,
		&conv.UserID,
		&conv.BotID,
		&conv.BotName,
		&conv.BotAvatar,
		&conv.BotAvatarBase64,
		&conv.LastMessage,
		&conv.LastMessageAt,
		&conv.UnreadCount,
		&conv.IsProactiveMessage,
	); err != nil {
		return Conversation{}, false
	}
	return conv, true
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (p *PostgresStore) ListAllUserIDs() []string {
	ctx, cancel := p.withTimeout()
	defer cancel()
	rows, err := p.db.QueryContext(ctx, `SELECT id FROM users ORDER BY created_at`)
	if err != nil {
		log.Printf("list all user ids failed: %v", err)
		return p.Store.ListAllUserIDs()
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return p.Store.ListAllUserIDs()
	}
	return ids
}

func (p *PostgresStore) String() string {
	return fmt.Sprintf("PostgresStore(redis=%v)", p.redis != nil)
}
