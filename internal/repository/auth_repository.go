package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"todo/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrSessionNotFound  = errors.New("session not found")
	ErrFriendNotFound   = errors.New("friend request not found")
	ErrCannotFriendSelf = errors.New("cannot add yourself as friend")
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
	isActive := true
	if userCount == 0 {
		role = domain.UserRoleAdmin
	}

	user, err = insertSSOUser(ctx, tx, input, role, isActive)
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
		SELECT id, sso_display_name, email, role, is_active, last_login_at, created_at, updated_at
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

func insertSSOUser(ctx context.Context, tx authTx, input SSOUserInput, role domain.UserRole, isActive bool) (domain.User, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO app_users (sso_display_name, role, is_active, auth_provider, external_subject, sso_username, email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, sso_display_name, email, role, is_active, last_login_at, created_at, updated_at
	`, input.DisplayName, role, isActive, input.Provider, input.ExternalSubject, input.Username, input.Email)
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
		SELECT u.id, u.sso_display_name, u.email, u.role, u.is_active, u.last_login_at, u.created_at, u.updated_at, s.expires_at
		FROM active_session s
		JOIN app_users u ON u.id = s.user_id
		WHERE u.is_active = TRUE
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

func (r *AuthRepository) ListFriends(ctx context.Context, userID uuid.UUID) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT friend.id, friend.sso_display_name, friend.email, friend.role, friend.is_active, friend.last_login_at, friend.created_at, friend.updated_at
		FROM user_friends friendship
		JOIN app_users friend ON friend.id = CASE
			WHEN friendship.requester_id = $1 THEN friendship.addressee_id
			ELSE friendship.requester_id
		END
		WHERE (friendship.requester_id = $1 OR friendship.addressee_id = $1)
			AND friendship.status = 'accepted'
			AND friend.is_active = TRUE
		ORDER BY lower(friend.sso_display_name), lower(friend.email)
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
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
		return nil, fmt.Errorf("iterate friends: %w", err)
	}
	return users, nil
}

func (r *AuthRepository) ListIncomingFriendRequests(ctx context.Context, userID uuid.UUID) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT requester.id, requester.sso_display_name, requester.email, requester.role, requester.is_active, requester.last_login_at, requester.created_at, requester.updated_at
		FROM user_friends friendship
		JOIN app_users requester ON requester.id = friendship.requester_id
		WHERE friendship.addressee_id = $1
			AND friendship.status = 'pending'
			AND requester.is_active = TRUE
		ORDER BY friendship.created_at DESC, lower(requester.sso_display_name)
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list incoming friend requests: %w", err)
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
		return nil, fmt.Errorf("iterate incoming friend requests: %w", err)
	}
	return users, nil
}

func (r *AuthRepository) RequestFriendByEmail(ctx context.Context, requesterID uuid.UUID, email string) (domain.User, error) {
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" {
		return domain.User{}, ErrUserNotFound
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin friend request: %w", err)
	}
	defer tx.Rollback(ctx)

	target, err := scanUser(tx.QueryRow(ctx, `
		SELECT id, sso_display_name, email, role, is_active, last_login_at, created_at, updated_at
		FROM app_users
		WHERE lower(email) = $1
			AND is_active = TRUE
	`, normalizedEmail))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	if target.ID == requesterID {
		return domain.User{}, ErrCannotFriendSelf
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_friends
		SET status = 'accepted'
		WHERE requester_id = $1
			AND addressee_id = $2
			AND status = 'pending'
	`, target.ID, requesterID); err != nil {
		return domain.User{}, fmt.Errorf("accept reciprocal friend request: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_friends (requester_id, addressee_id, status)
		VALUES ($1, $2, 'pending')
		ON CONFLICT DO NOTHING
	`, requesterID, target.ID); err != nil {
		return domain.User{}, fmt.Errorf("insert friend request: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit friend request: %w", err)
	}

	return target, nil
}

func (r *AuthRepository) AcceptFriendRequest(ctx context.Context, userID, requesterID uuid.UUID) (domain.User, error) {
	return r.updateFriendRequest(ctx, userID, requesterID, true)
}

func (r *AuthRepository) RejectFriendRequest(ctx context.Context, userID, requesterID uuid.UUID) (domain.User, error) {
	return r.updateFriendRequest(ctx, userID, requesterID, false)
}

func (r *AuthRepository) updateFriendRequest(ctx context.Context, userID, requesterID uuid.UUID, accept bool) (domain.User, error) {
	target, err := scanUser(r.db.QueryRow(ctx, `
		SELECT requester.id, requester.sso_display_name, requester.email, requester.role, requester.is_active, requester.last_login_at, requester.created_at, requester.updated_at
		FROM user_friends friendship
		JOIN app_users requester ON requester.id = friendship.requester_id
		WHERE friendship.requester_id = $1
			AND friendship.addressee_id = $2
			AND friendship.status = 'pending'
	`, requesterID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrFriendNotFound
		}
		return domain.User{}, err
	}

	if accept {
		_, err = r.db.Exec(ctx, `
			UPDATE user_friends
			SET status = 'accepted'
			WHERE requester_id = $1
				AND addressee_id = $2
				AND status = 'pending'
		`, requesterID, userID)
	} else {
		_, err = r.db.Exec(ctx, `
			DELETE FROM user_friends
			WHERE requester_id = $1
				AND addressee_id = $2
				AND status = 'pending'
		`, requesterID, userID)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("update friend request: %w", err)
	}

	return target, nil
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
	destinations := []any{
		&user.ID,
		&user.DisplayName,
		&user.Email,
		&user.Role,
		&user.IsActive,
		&lastLoginAt,
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
	return user, nil
}
