package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidSession       = errors.New("invalid session")
	ErrRegistrationDisabled = errors.New("registration disabled")
	ErrUsernameTaken        = errors.New("username already exists")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$`)

type AuthResult struct {
	User      domain.User
	Token     string
	ExpiresAt time.Time
}

type AuthService struct {
	repo              *repository.AuthRepository
	sessionCookieName string
	sessionTTL        time.Duration
	allowRegistration bool
}

func NewAuthService(repo *repository.AuthRepository, sessionCookieName string, sessionTTL time.Duration, allowRegistration bool) *AuthService {
	return &AuthService{
		repo:              repo,
		sessionCookieName: sessionCookieName,
		sessionTTL:        sessionTTL,
		allowRegistration: allowRegistration,
	}
}

func (s *AuthService) Register(ctx context.Context, username, displayName, password, userAgent, ipAddress string) (AuthResult, error) {
	if !s.allowRegistration {
		return AuthResult{}, ErrRegistrationDisabled
	}

	normalizedUsername, err := validateUsername(username)
	if err != nil {
		return AuthResult{}, err
	}
	normalizedDisplayName, err := validateDisplayName(displayName)
	if err != nil {
		return AuthResult{}, err
	}
	if err := validatePassword(password); err != nil {
		return AuthResult{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate password hash: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, repository.CreateUserInput{
		Username:     normalizedUsername,
		DisplayName:  normalizedDisplayName,
		PasswordHash: string(passwordHash),
		Role:         domain.UserRoleMember,
	})
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return AuthResult{}, ErrUsernameTaken
		}
		return AuthResult{}, err
	}

	return s.issueSession(ctx, user, userAgent, ipAddress)
}

func (s *AuthService) Login(ctx context.Context, username, password, userAgent, ipAddress string) (AuthResult, error) {
	normalizedUsername, err := validateUsername(username)
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	user, err := s.repo.FindUserByUsername(ctx, normalizedUsername)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}
	if !user.IsActive {
		return AuthResult{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	return s.issueSession(ctx, user, userAgent, ipAddress)
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if strings.TrimSpace(token) == "" {
		return domain.User{}, ErrInvalidSession
	}

	user, err := s.repo.GetUserBySessionTokenHash(ctx, hashToken(token), time.Now().UTC())
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return domain.User{}, ErrInvalidSession
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}

	err := s.repo.RevokeSession(ctx, hashToken(token))
	if err != nil && !errors.Is(err, repository.ErrSessionNotFound) {
		return err
	}
	return nil
}

func (s *AuthService) SessionCookieName() string {
	return s.sessionCookieName
}

func (s *AuthService) AllowRegistration() bool {
	return s.allowRegistration
}

func (s *AuthService) issueSession(ctx context.Context, user domain.User, userAgent, ipAddress string) (AuthResult, error) {
	token, err := generateSessionToken()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate session token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	if err := s.repo.CreateSession(ctx, repository.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: hashToken(token),
		UserAgent: strings.TrimSpace(userAgent),
		IPAddress: strings.TrimSpace(ipAddress),
		ExpiresAt: expiresAt,
	}); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:      user,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func generateSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validateUsername(value string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if !usernamePattern.MatchString(normalized) {
		return "", errors.New("用户名长度需为 3-32，只能包含字母、数字、点、下划线和中划线")
	}
	return normalized, nil
}

func validateDisplayName(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if len([]rune(normalized)) < 1 || len([]rune(normalized)) > 32 {
		return "", errors.New("显示名称长度需为 1-32")
	}
	return normalized, nil
}

func validatePassword(value string) error {
	if len(value) < 8 {
		return errors.New("密码至少需要 8 位")
	}
	if len(value) > 72 {
		return errors.New("密码最长支持 72 位")
	}
	if strings.TrimSpace(value) == "" {
		return errors.New("密码不能为空白字符")
	}
	return nil
}
