ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS sso_display_name TEXT NOT NULL DEFAULT '';

UPDATE app_users
SET sso_display_name = COALESCE(
    NULLIF(btrim(sso_display_name), ''),
    NULLIF(btrim(display_name), ''),
    NULLIF(btrim(sso_username), ''),
    'SSO User'
)
WHERE btrim(sso_display_name) = '';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_sso_display_name_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_sso_display_name_check
            CHECK (char_length(btrim(sso_display_name)) BETWEEN 1 AND 64);
    END IF;
END
$$;

ALTER TABLE app_users
    DROP CONSTRAINT IF EXISTS app_users_username_key,
    DROP CONSTRAINT IF EXISTS app_users_username_check,
    DROP CONSTRAINT IF EXISTS app_users_username_lowercase_check,
    DROP CONSTRAINT IF EXISTS app_users_username_format_check,
    DROP CONSTRAINT IF EXISTS app_users_display_name_check,
    DROP CONSTRAINT IF EXISTS app_users_display_name_length_check,
    DROP CONSTRAINT IF EXISTS app_users_password_hash_present_check;

ALTER TABLE app_users
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS password_hash;
