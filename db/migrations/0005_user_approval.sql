DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_approval_status') THEN
        CREATE TYPE user_approval_status AS ENUM ('pending', 'approved');
    END IF;
END
$$;

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS approval_status user_approval_status;

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;

ALTER TABLE app_users
    ADD COLUMN IF NOT EXISTS approved_by UUID;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_approved_by_fkey') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_approved_by_fkey
            FOREIGN KEY (approved_by) REFERENCES app_users(id) ON DELETE SET NULL;
    END IF;
END
$$;

UPDATE app_users
SET
    approval_status = 'approved',
    approved_at = COALESCE(approved_at, last_login_at, created_at)
WHERE approval_status IS NULL;

ALTER TABLE app_users
    ALTER COLUMN approval_status SET NOT NULL;

ALTER TABLE app_users
    ALTER COLUMN approval_status SET DEFAULT 'pending';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_approval_state_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_approval_state_check
            CHECK (
                (approval_status = 'pending' AND approved_at IS NULL AND approved_by IS NULL) OR
                (approval_status = 'approved' AND approved_at IS NOT NULL)
            );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'app_users_approved_after_create_check') THEN
        ALTER TABLE app_users
            ADD CONSTRAINT app_users_approved_after_create_check
            CHECK (approved_at IS NULL OR approved_at >= created_at);
    END IF;
END
$$;

WITH promoted AS (
    SELECT id
    FROM app_users
    WHERE approval_status = 'approved'
    ORDER BY created_at
    LIMIT 1
)
UPDATE app_users
SET role = 'admin'
WHERE id IN (SELECT id FROM promoted)
  AND NOT EXISTS (SELECT 1 FROM app_users WHERE role = 'admin');

CREATE INDEX IF NOT EXISTS idx_app_users_approval_status_created_at
    ON app_users (approval_status, created_at);
