DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum
        WHERE enumtypid = 'task_event_type'::regtype
          AND enumlabel = 'restored'
    ) THEN
        ALTER TYPE task_event_type ADD VALUE 'restored';
    END IF;
END
$$;
