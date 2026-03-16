DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_username_lowercase_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_username_lowercase_check
            CHECK (username::text = lower(username::text));
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_username_format_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_username_format_check
            CHECK (username::text ~ '^[a-z0-9][a-z0-9._-]{2,31}$');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_display_name_length_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_display_name_length_check
            CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 32);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_password_hash_present_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_password_hash_present_check
            CHECK (char_length(btrim(password_hash)) > 0);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_last_login_after_create_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_last_login_after_create_check
            CHECK (last_login_at IS NULL OR last_login_at >= created_at);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_updated_after_create_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_updated_after_create_check
            CHECK (updated_at >= created_at);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'user_sessions_expires_after_create_check') THEN
        ALTER TABLE user_sessions
            ADD CONSTRAINT user_sessions_expires_after_create_check
            CHECK (expires_at > created_at);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'user_sessions_revoked_after_create_check') THEN
        ALTER TABLE user_sessions
            ADD CONSTRAINT user_sessions_revoked_after_create_check
            CHECK (revoked_at IS NULL OR revoked_at >= created_at);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_raw_content_check') THEN
        ALTER TABLE ingestion_sources
            ADD CONSTRAINT ingestion_sources_raw_content_check
            CHECK (char_length(btrim(raw_content)) > 0);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_checksum_check') THEN
        ALTER TABLE ingestion_sources
            ADD CONSTRAINT ingestion_sources_checksum_check
            CHECK (char_length(btrim(checksum)) > 0);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_metadata_object_check') THEN
        ALTER TABLE ingestion_sources
            ADD CONSTRAINT ingestion_sources_metadata_object_check
            CHECK (jsonb_typeof(metadata) = 'object');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ingestion_sources_id_user_id_unique') THEN
        ALTER TABLE ingestion_sources
            ADD CONSTRAINT ingestion_sources_id_user_id_unique
            UNIQUE (id, user_id);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_metadata_object_check') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_metadata_object_check
            CHECK (jsonb_typeof(metadata) = 'object');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_updated_after_create_check') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_updated_after_create_check
            CHECK (updated_at >= created_at);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_completed_after_create_check') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_completed_after_create_check
            CHECK (completed_at IS NULL OR completed_at >= created_at);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_done_supported_types_check') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_done_supported_types_check
            CHECK (status <> 'done' OR task_type IN ('todo', 'ddl'));
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_postpone_supported_types_check') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_postpone_supported_types_check
            CHECK (postponed_count = 0 OR task_type IN ('schedule', 'ddl'));
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_source_matches_user_fkey') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_source_matches_user_fkey
            FOREIGN KEY (source_id, user_id)
            REFERENCES ingestion_sources(id, user_id)
            ON UPDATE CASCADE
            ON DELETE RESTRICT;
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'task_events_payload_object_check') THEN
        ALTER TABLE task_events
            ADD CONSTRAINT task_events_payload_object_check
            CHECK (jsonb_typeof(payload) = 'object');
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_ingestion_sources_user_source_type_checksum
    ON ingestion_sources (user_id, source_type, checksum, created_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_source_id
    ON tasks (user_id, source_id)
    WHERE user_id IS NOT NULL AND source_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_task_events_created_at_desc
    ON task_events (created_at DESC);
