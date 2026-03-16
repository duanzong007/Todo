DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
            AND table_name = 'ingestion_sources'
            AND column_name = 'user_id'
            AND is_nullable = 'YES'
    ) AND NOT EXISTS (SELECT 1 FROM ingestion_sources WHERE user_id IS NULL) THEN
        IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_user_id_required') THEN
            ALTER TABLE ingestion_sources
                VALIDATE CONSTRAINT ingestion_sources_user_id_required;
        END IF;

        ALTER TABLE ingestion_sources
            ALTER COLUMN user_id SET NOT NULL;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
            AND table_name = 'tasks'
            AND column_name = 'user_id'
            AND is_nullable = 'YES'
    ) AND NOT EXISTS (SELECT 1 FROM tasks WHERE user_id IS NULL) THEN
        IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_user_id_required') THEN
            ALTER TABLE tasks
                VALIDATE CONSTRAINT tasks_user_id_required;
        END IF;

        ALTER TABLE tasks
            ALTER COLUMN user_id SET NOT NULL;
    END IF;
END
$$;
