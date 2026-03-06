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
