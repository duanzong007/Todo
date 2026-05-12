package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"todo/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionNotFound = errors.New("session not found")
	ErrUserNotPending  = errors.New("user is not pending approval")
)

type CreateSessionInput struct {
	UserID    uuid.UUID
	TokenHash string
	UserAgent string
	IPAddress string
	ExpiresAt time.Time
}

type SSOUserInput struct {
	Provider        string
	ExternalSubject string
	Username        string
	DisplayName     string
	Email           string
	AutoApprove     bool
}

type AuthRepository struct {
	db *pgxpool.Pool
}

func NewAuthRepository(db *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindUserBySSO(ctx context.Context, provider, externalSubject string) (domain.User, error) {
	user, err := findUserBySSO(ctx, r.db, SSOUserInput{
		Provider:        provider,
		ExternalSubject: externalSubject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *AuthRepository) FindOrCreateUserBySSO(ctx context.Context, input SSOUserInput) (domain.User, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.User{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	user, err := findUserBySSO(ctx, tx, input)
	if err == nil {
		if err := updateSSOUserProfile(ctx, tx, input); err != nil {
			return domain.User{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.User{}, false, fmt.Errorf("commit sso login: %w", err)
		}
		return user, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, false, err
	}

	var userCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM app_users`).Scan(&userCount); err != nil {
		return domain.User{}, false, fmt.Errorf("count users: %w", err)
	}

	role := domain.UserRoleMember
	isActive := input.AutoApprove
	approvalStatus := domain.UserApprovalPending
	var approvedAt any
	if input.AutoApprove {
		approvalStatus = domain.UserApprovalApproved
		approvedAt = time.Now().UTC()
	}
	if userCount == 0 {
		role = domain.UserRoleAdmin
		isActive = true
		approvalStatus = domain.UserApprovalApproved
		approvedAt = time.Now().UTC()
	}

	user, err = insertSSOUser(ctx, tx, input, role, approvalStatus, isActive, approvedAt)
	if err != nil {
		return domain.User{}, false, err
	}

	if userCount == 0 {
		if _, err := tx.Exec(ctx, `UPDATE ingestion_sources SET user_id = $1 WHERE user_id IS NULL`, user.ID); err != nil {
			return domain.User{}, false, fmt.Errorf("claim orphan ingestion_sources: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE tasks SET user_id = $1 WHERE user_id IS NULL`, user.ID); err != nil {
			return domain.User{}, false, fmt.Errorf("claim orphan tasks: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, false, fmt.Errorf("commit create sso user: %w", err)
	}

	return user, true, nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, input CreateSessionInput) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_sessions (user_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, input.UserID, input.TokenHash, input.UserAgent, input.IPAddress, input.ExpiresAt); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE app_users
		SET last_login_at = NOW()
		WHERE id = $1
	`, input.UserID); err != nil {
		return fmt.Errorf("update last_login_at: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create session: %w", err)
	}

	return nil
}

type authTx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func findUserBySSO(ctx context.Context, tx authTx, input SSOUserInput) (domain.User, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, sso_display_name, role, approval_status, is_active, last_login_at, approved_at, approved_by, created_at, updated_at
		FROM app_users
		WHERE auth_provider = $1
			AND external_subject = $2
	`, input.Provider, input.ExternalSubject)
	return scanUser(row)
}

func updateSSOUserProfile(ctx context.Context, tx authTx, input SSOUserInput) error {
	_, err := tx.Exec(ctx, `
		UPDATE app_users
		SET
			sso_username = $3,
			email = $4,
			sso_display_name = $5
		WHERE auth_provider = $1
			AND external_subject = $2
	`, input.Provider, input.ExternalSubject, input.Username, input.Email, input.DisplayName)
	if err != nil {
		return fmt.Errorf("update sso user profile: %w", err)
	}
	return nil
}

func insertSSOUser(ctx context.Context, tx authTx, input SSOUserInput, role domain.UserRole, approvalStatus domain.UserApprovalStatus, isActive bool, approvedAt any) (domain.User, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO app_users (sso_display_name, role, is_active, approval_status, approved_at, auth_provider, external_subject, sso_username, email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, sso_display_name, role, approval_status, is_active, last_login_at, approved_at, approved_by, created_at, updated_at
	`, input.DisplayName, role, isActive, approvalStatus, approvedAt, input.Provider, input.ExternalSubject, input.Username, input.Email)
	return scanUser(row)
}

func (r *AuthRepository) GetUserBySessionTokenHash(ctx context.Context, tokenHash string, now, refreshedExpiresAt time.Time) (domain.User, time.Time, error) {
	row := r.db.QueryRow(ctx, `
		WITH active_session AS (
			UPDATE user_sessions
			SET
				last_seen_at = $2,
				expires_at = $3
			WHERE token_hash = $1
				AND revoked_at IS NULL
				AND expires_at > $2
			RETURNING user_id, expires_at
		)
		SELECT u.id, u.sso_display_name, u.role, u.approval_status, u.is_active, u.last_login_at, u.approved_at, u.approved_by, u.created_at, u.updated_at, s.expires_at
		FROM active_session s
		JOIN app_users u ON u.id = s.user_id
		WHERE u.is_active = TRUE
			AND u.approval_status = 'approved'
	`, tokenHash, now, refreshedExpiresAt)

	var expiresAt time.Time
	user, err := scanUserWithExtra(row, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, time.Time{}, ErrSessionNotFound
		}
		return domain.User{}, time.Time{}, err
	}
	return user, expiresAt, nil
}

func (r *AuthRepository) ListPendingUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, sso_display_name, role, approval_status, is_active, last_login_at, approved_at, approved_by, created_at, updated_at
		FROM app_users
		WHERE approval_status = 'pending'
		ORDER BY created_at, sso_display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending users: %w", err)
	}
	return users, nil
}

func (r *AuthRepository) ListApprovedUsersExcept(ctx context.Context, excludeUserID uuid.UUID) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, sso_display_name, role, approval_status, is_active, last_login_at, approved_at, approved_by, created_at, updated_at
		FROM app_users
		WHERE id <> $1
			AND is_active = TRUE
			AND approval_status = 'approved'
		ORDER BY lower(sso_display_name)
	`, excludeUserID)
	if err != nil {
		return nil, fmt.Errorf("list approved users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approved users: %w", err)
	}
	return users, nil
}

func (r *AuthRepository) ApproveUser(ctx context.Context, adminID, userID uuid.UUID) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE app_users
		SET
			approval_status = 'approved',
			is_active = TRUE,
			approved_at = NOW(),
			approved_by = $1
		WHERE id = $2
			AND approval_status = 'pending'
		RETURNING id, sso_display_name, role, approval_status, is_active, last_login_at, approved_at, approved_by, created_at, updated_at
	`, adminID, userID)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if queryErr := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_users WHERE id = $1)`, userID).Scan(&exists); queryErr != nil {
				return domain.User{}, fmt.Errorf("check user existence: %w", queryErr)
			}
			if !exists {
				return domain.User{}, ErrUserNotFound
			}
			return domain.User{}, ErrUserNotPending
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *AuthRepository) DeletePendingUser(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
		DELETE FROM app_users
		WHERE id = $1
			AND approval_status = 'pending'
		RETURNING id, sso_display_name, role, approval_status, is_active, last_login_at, approved_at, approved_by, created_at, updated_at
	`, userID)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if queryErr := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app_users WHERE id = $1)`, userID).Scan(&exists); queryErr != nil {
				return domain.User{}, fmt.Errorf("check user existence: %w", queryErr)
			}
			if !exists {
				return domain.User{}, ErrUserNotFound
			}
			return domain.User{}, ErrUserNotPending
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *AuthRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	commandTag, err := r.db.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE token_hash = $1
	`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func scanUser(row interface{ Scan(dest ...any) error }) (domain.User, error) {
	return scanUserWithExtra(row)
}

func scanUserWithExtra(row interface{ Scan(dest ...any) error }, extraDest ...any) (domain.User, error) {
	var user domain.User
	var lastLoginAt pgtype.Timestamptz
	var approvedAt pgtype.Timestamptz
	var approvedBy pgtype.UUID
	destinations := []any{
		&user.ID,
		&user.DisplayName,
		&user.Role,
		&user.ApprovalStatus,
		&user.IsActive,
		&lastLoginAt,
		&approvedAt,
		&approvedBy,
		&user.CreatedAt,
		&user.UpdatedAt,
	}
	destinations = append(destinations, extraDest...)

	err := row.Scan(destinations...)
	if err != nil {
		return domain.User{}, err
	}

	if lastLoginAt.Valid {
		value := lastLoginAt.Time
		user.LastLoginAt = &value
	}
	if approvedAt.Valid {
		value := approvedAt.Time
		user.ApprovedAt = &value
	}
	if approvedBy.Valid {
		value := uuid.UUID(approvedBy.Bytes)
		user.ApprovedBy = &value
	}
	return user, nil
}
