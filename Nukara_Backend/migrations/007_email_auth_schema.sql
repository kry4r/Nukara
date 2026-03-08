-- migrations/007_email_auth_schema.sql
-- Replace phone/SMS auth schema with email-based auth while preserving user IDs and related business data.

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'phone'
    ) THEN
        ALTER TABLE users RENAME COLUMN phone TO email;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'email'
    ) THEN
        ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF to_regclass('public.sms_codes') IS NOT NULL AND to_regclass('public.email_codes') IS NULL THEN
        ALTER TABLE sms_codes RENAME TO email_codes;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS email_codes (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    purpose VARCHAR(20) NOT NULL,
    code VARCHAR(10) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'email_codes' AND column_name = 'phone'
    ) THEN
        ALTER TABLE email_codes RENAME COLUMN phone TO email;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'email_codes' AND column_name = 'email'
    ) THEN
        ALTER TABLE email_codes ALTER COLUMN email TYPE VARCHAR(255);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_email_codes_email_purpose_created_at
    ON email_codes(email, purpose, created_at DESC);
