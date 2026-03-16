DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
            AND table_name = 'tasks'
            AND column_name = 'deadline'
            AND data_type = 'date'
    ) THEN
        ALTER TABLE tasks
            ALTER COLUMN deadline TYPE TIMESTAMPTZ
            USING CASE
                WHEN deadline IS NULL THEN NULL
                ELSE make_timestamptz(
                    EXTRACT(YEAR FROM deadline)::int,
                    EXTRACT(MONTH FROM deadline)::int,
                    EXTRACT(DAY FROM deadline)::int,
                    23,
                    59,
                    0,
                    'Asia/Shanghai'
                )
            END;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_done_supported_types_check') THEN
        ALTER TABLE tasks
            DROP CONSTRAINT tasks_done_supported_types_check;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_tasks_user_done_completed_at_desc
    ON tasks (user_id, completed_at DESC, updated_at DESC)
    WHERE status = 'done' AND user_id IS NOT NULL;
