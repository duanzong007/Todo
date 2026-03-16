ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS importance SMALLINT;

UPDATE tasks
SET importance = 3
WHERE importance IS NULL;

ALTER TABLE tasks
    ALTER COLUMN importance SET DEFAULT 3;

ALTER TABLE tasks
    ALTER COLUMN importance SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tasks_importance_range_check') THEN
        ALTER TABLE tasks
            ADD CONSTRAINT tasks_importance_range_check
            CHECK (importance BETWEEN 1 AND 5);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_tasks_user_active_schedule_importance_date
    ON tasks (user_id, importance DESC, scheduled_for ASC, created_at ASC)
    WHERE status = 'active' AND task_type = 'schedule' AND user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_active_deadline_importance_date
    ON tasks (user_id, importance DESC, deadline ASC, created_at ASC)
    WHERE status = 'active' AND task_type = 'ddl' AND user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_user_active_todo_importance_created_at
    ON tasks (user_id, importance DESC, created_at ASC)
    WHERE status = 'active' AND task_type = 'todo' AND user_id IS NOT NULL;
