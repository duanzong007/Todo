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
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrSessionNotFound   = errors.New("session not found")
)

type CreateUserInput struct {
	Username     string
	DisplayName  string
	PasswordHash string
	Role         domain.UserRole
}

type CreateSessionInput struct {
	UserID    uuid.UUID
	TokenHash string
	UserAgent string
	IPAddress string
	ExpiresAt time.Time
}

type AuthRepository struct {
	db *pgxpool.Pool
}

func NewAuthRepository(db *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) CreateUser(ctx context.Context, input CreateUserInput) (domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	role := input.Role
	if role == "" {
		role = domain.UserRoleMember
	}

	var userCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM app_users`).Scan(&userCount); err != nil {
		return domain.User{}, fmt.Errorf("count users: %w", err)
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO app_users (username, display_name, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, display_name, password_hash, role, is_active, last_login_at, created_at, updated_at
	`, input.Username, input.DisplayName, input.PasswordHash, role)

	user, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrUserAlreadyExists
		}
		return domain.User{}, err
	}

	if userCount == 0 {
		if _, err := tx.Exec(ctx, `UPDATE ingestion_sources SET user_id = $1 WHERE user_id IS NULL`, user.ID); err != nil {
			return domain.User{}, fmt.Errorf("claim orphan ingestion_sources: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE tasks SET user_id = $1 WHERE user_id IS NULL`, user.ID); err != nil {
			return domain.User{}, fmt.Errorf("claim orphan tasks: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit create user: %w", err)
	}

	return user, nil
}

func (r *AuthRepository) FindUserByUsername(ctx context.Context, username string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, username, display_name, password_hash, role, is_active, last_login_at, created_at, updated_at
		FROM app_users
		WHERE username = $1
	`, username)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
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

func (r *AuthRepository) GetUserBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
		WITH active_session AS (
			UPDATE user_sessions
			SET last_seen_at = $2
			WHERE token_hash = $1
				AND revoked_at IS NULL
				AND expires_at > $2
			RETURNING user_id
		)
		SELECT u.id, u.username, u.display_name, u.password_hash, u.role, u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM active_session s
		JOIN app_users u ON u.id = s.user_id
		WHERE u.is_active = TRUE
	`, tokenHash, now)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrSessionNotFound
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
	var user domain.User
	var lastLoginAt pgtype.Timestamptz

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, err
	}

	if lastLoginAt.Valid {
		value := lastLoginAt.Time
		user.LastLoginAt = &value
	}
	return user, nil
}
