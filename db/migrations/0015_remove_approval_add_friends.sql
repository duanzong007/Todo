UPDATE app_users
SET is_active = TRUE
WHERE is_active = FALSE;

DROP INDEX IF EXISTS idx_app_users_approval_status_created_at;

ALTER TABLE app_users
    DROP CONSTRAINT IF EXISTS app_users_approval_state_check,
    DROP CONSTRAINT IF EXISTS app_users_approved_after_create_check,
    DROP CONSTRAINT IF EXISTS app_users_approved_by_fkey;

ALTER TABLE app_users
    DROP COLUMN IF EXISTS approval_status,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS approved_by;

DROP TYPE IF EXISTS user_approval_status;

CREATE TABLE IF NOT EXISTS user_friends (
    requester_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    addressee_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (requester_id, addressee_id),
    CONSTRAINT user_friends_not_self_check CHECK (requester_id <> addressee_id),
    CONSTRAINT user_friends_status_check CHECK (status IN ('pending', 'accepted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_friends_pair_unique
    ON user_friends (
        LEAST(requester_id, addressee_id),
        GREATEST(requester_id, addressee_id)
    );

CREATE INDEX IF NOT EXISTS idx_user_friends_addressee_status_created_at
    ON user_friends (addressee_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_app_users_email_lower_active
    ON app_users (lower(email))
    WHERE is_active = TRUE AND btrim(email) <> '';

DROP TRIGGER IF EXISTS user_friends_set_updated_at ON user_friends;

CREATE TRIGGER user_friends_set_updated_at
BEFORE UPDATE ON user_friends
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
