CREATE EXTENSION IF NOT EXISTS citext;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('member', 'admin');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS app_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username CITEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role user_role NOT NULL DEFAULT 'member',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT app_users_username_check CHECK (char_length(btrim(username::text)) >= 3),
    CONSTRAINT app_users_display_name_check CHECK (char_length(btrim(display_name)) >= 1)
);

CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_sessions_token_hash_check CHECK (char_length(token_hash) > 0)
);

DROP TRIGGER IF EXISTS app_users_set_updated_at ON app_users;

CREATE TRIGGER app_users_set_updated_at
BEFORE UPDATE ON app_users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE ingestion_sources
    ADD COLUMN IF NOT EXISTS user_id UUID;

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS user_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_user_id_fkey') THEN
        ALTER TABLE ingestion_sources
            ADD CONSTRAINT ingestion_sources_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES app_users(id) ON DELETE CASCADE;
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_user_id_fkey') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES app_users(id) ON DELETE CASCADE;
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_user_id_required') THEN
        ALTER TABLE ingestion_sources
            ADD CONSTRAINT ingestion_sources_user_id_required
            CHECK (user_id IS NOT NULL) NOT VALID;
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_user_id_required') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_user_id_required
            CHECK (user_id IS NOT NULL) NOT VALID;
    END IF;
END
$$;

DROP INDEX IF EXISTS idx_tasks_ics_uid_schedule_unique;

CREATE INDEX IF NOT EXISTS idx_ingestion_sources_user_created_at
    ON ingestion_sources (user_id, created_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_active_schedule_date
    ON tasks (user_id, scheduled_for)
    WHERE status = 'active' AND task_type = 'schedule' AND user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_active_deadline
    ON tasks (user_id, deadline)
    WHERE status = 'active' AND task_type = 'ddl' AND user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_active_todo_created_at
    ON tasks (user_id, created_at DESC)
    WHERE status = 'active' AND task_type = 'todo' AND user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_user_ics_uid_schedule_unique
    ON tasks (user_id, (metadata ->> 'ics_uid'), scheduled_for)
    WHERE task_type = 'schedule' AND metadata ? 'ics_uid' AND user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id_created_at
    ON user_sessions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_sessions_active_expires_at
    ON user_sessions (expires_at)
    WHERE revoked_at IS NULL;
