DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum
        WHERE enumlabel = 'rescheduled'
          AND enumtypid = 'task_event_type'::regtype
    ) THEN
        ALTER TYPE task_event_type ADD VALUE 'rescheduled';
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum
        WHERE enumlabel = 'shared'
          AND enumtypid = 'task_event_type'::regtype
    ) THEN
        ALTER TYPE task_event_type ADD VALUE 'shared';
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS task_shares (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    shared_by UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, user_id),
    CONSTRAINT task_shares_not_self_shared CHECK (user_id <> shared_by)
);

CREATE INDEX IF NOT EXISTS idx_task_shares_user_created_at
    ON task_shares (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_shares_task_id
    ON task_shares (task_id);
