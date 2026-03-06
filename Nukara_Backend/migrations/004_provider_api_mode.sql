ALTER TABLE providers
ADD COLUMN IF NOT EXISTS api_mode TEXT NOT NULL DEFAULT 'chat_completions';
