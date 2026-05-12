ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS auth_provider TEXT NOT NULL DEFAULT 'local';

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS external_subject TEXT;

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS sso_username TEXT NOT NULL DEFAULT '';

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS sso_display_name TEXT NOT NULL DEFAULT '';

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_auth_provider_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_auth_provider_check
            CHECK (char_length(btrim(auth_provider)) > 0);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_external_subject_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_external_subject_check
            CHECK (external_subject IS NULL OR char_length(btrim(external_subject)) > 0);
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_users_auth_provider_external_subject
    ON app_users (auth_provider, external_subject)
    WHERE external_subject IS NOT NULL;
