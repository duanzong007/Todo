DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'tasks_source_id_fkey'
          AND conrelid = 'tasks'::regclass
    ) THEN
        ALTER TABLE tasks
            DROP CONSTRAINT tasks_source_id_fkey;
    END IF;
END
$$;

UPDATE task_shares AS share_row
SET shared_by = owner_task.user_id
FROM tasks AS owner_task
WHERE owner_task.id = share_row.task_id
  AND share_row.shared_by <> owner_task.user_id;

CREATE OR REPLACE FUNCTION enforce_task_share_owner()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    owner_id UUID;
BEGIN
    SELECT user_id
    INTO owner_id
    FROM tasks
    WHERE id = NEW.task_id;

    IF owner_id IS NULL THEN
        RAISE EXCEPTION 'task % does not exist for sharing', NEW.task_id;
    END IF;

    IF NEW.shared_by <> owner_id THEN
        RAISE EXCEPTION 'shared_by must match task owner for task %', NEW.task_id;
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS task_shares_enforce_owner ON task_shares;

CREATE TRIGGER task_shares_enforce_owner
BEFORE INSERT OR UPDATE OF task_id, shared_by
ON task_shares
FOR EACH ROW
EXECUTE FUNCTION enforce_task_share_owner();

CREATE INDEX IF NOT EXISTS idx_task_shares_user_task_id
    ON task_shares (user_id, task_id);
