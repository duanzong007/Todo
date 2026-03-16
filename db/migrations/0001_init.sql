CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'task_type') THEN
        CREATE TYPE task_type AS ENUM ('todo', 'schedule', 'ddl');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'task_status') THEN
        CREATE TYPE task_status AS ENUM ('active', 'done');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'source_type') THEN
        CREATE TYPE source_type AS ENUM ('manual_text', 'sms_paste', 'ics_import');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'task_event_type') THEN
        CREATE TYPE task_event_type AS ENUM ('created', 'imported', 'completed', 'postponed');
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TABLE IF NOT EXISTS ingestion_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type source_type NOT NULL,
    raw_content TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    checksum TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID REFERENCES ingestion_sources(id) ON DELETE SET NULL,
    title TEXT NOT NULL CHECK (char_length(btrim(title)) > 0),
    note TEXT NOT NULL DEFAULT '',
    task_type task_type NOT NULL,
    status task_status NOT NULL DEFAULT 'active',
    scheduled_for DATE,
    deadline DATE,
    completed_at TIMESTAMPTZ,
    postponed_count INTEGER NOT NULL DEFAULT 0 CHECK (postponed_count >= 0),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tasks_type_dates_check CHECK (
        (task_type = 'todo' AND scheduled_for IS NULL AND deadline IS NULL) OR
        (task_type = 'schedule' AND scheduled_for IS NOT NULL AND deadline IS NULL) OR
        (task_type = 'ddl' AND scheduled_for IS NULL AND deadline IS NOT NULL)
    ),
    CONSTRAINT tasks_status_check CHECK (
        (status = 'active' AND completed_at IS NULL) OR
        (status = 'done' AND completed_at IS NOT NULL)
    )
);

CREATE TABLE IF NOT EXISTS task_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    event_type task_event_type NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ingestion_sources_created_at
    ON ingestion_sources (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ingestion_sources_checksum
    ON ingestion_sources (checksum);

CREATE INDEX IF NOT EXISTS idx_tasks_source_id
    ON tasks (source_id);

CREATE INDEX IF NOT EXISTS idx_tasks_active_schedule_date
    ON tasks (scheduled_for)
    WHERE status = 'active' AND task_type = 'schedule';

CREATE INDEX IF NOT EXISTS idx_tasks_active_deadline
    ON tasks (deadline)
    WHERE status = 'active' AND task_type = 'ddl';

CREATE INDEX IF NOT EXISTS idx_tasks_active_todo_created_at
    ON tasks (created_at DESC)
    WHERE status = 'active' AND task_type = 'todo';

CREATE INDEX IF NOT EXISTS idx_tasks_active_type_created_at
    ON tasks (task_type, created_at DESC)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_tasks_metadata_gin
    ON tasks
    USING GIN (metadata);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_ics_uid_schedule_unique
    ON tasks ((metadata ->> 'ics_uid'), scheduled_for)
    WHERE task_type = 'schedule' AND metadata ? 'ics_uid';

CREATE INDEX IF NOT EXISTS idx_task_events_task_created_at
    ON task_events (task_id, created_at DESC);

DROP TRIGGER IF EXISTS tasks_set_updated_at ON tasks;

CREATE TRIGGER tasks_set_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
