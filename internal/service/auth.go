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

	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrFriendNotFound     = errors.New("friend request not found")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$`)

type AuthResult struct {
	User      domain.User
	Token     string
	ExpiresAt time.Time
}

type SessionAuthResult struct {
	User      domain.User
	ExpiresAt time.Time
}

type AuthService struct {
	repo              *repository.AuthRepository
	sessionCookieName string
	sessionTTL        time.Duration
	ssoClient         *SSOClient
}

func NewAuthService(repo *repository.AuthRepository, sessionCookieName string, sessionTTL time.Duration, ssoClient *SSOClient) *AuthService {
	return &AuthService{
		repo:              repo,
		sessionCookieName: sessionCookieName,
		sessionTTL:        sessionTTL,
		ssoClient:         ssoClient,
	}
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (SessionAuthResult, error) {
	if strings.TrimSpace(token) == "" {
		return SessionAuthResult{}, ErrInvalidSession
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.sessionTTL)
	user, sessionExpiresAt, err := s.repo.GetUserBySessionTokenHash(ctx, hashToken(token), now, expiresAt)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return SessionAuthResult{}, ErrInvalidSession
		}
		return SessionAuthResult{}, err
	}
	return SessionAuthResult{
		User:      user,
		ExpiresAt: sessionExpiresAt,
	}, nil
}

func (s *AuthService) SSOConfigured() bool {
	return s.ssoClient != nil
}

func (s *AuthService) SSOAuthCodeURL(state, nonce, redirectURL string) (string, error) {
	if s.ssoClient == nil {
		return "", ErrSSONotConfigured
	}
	return s.ssoClient.AuthCodeURL(state, nonce, redirectURL), nil
}

func (s *AuthService) LoginWithSSO(ctx context.Context, code, nonce, redirectURL, userAgent, ipAddress string) (AuthResult, error) {
	if s.ssoClient == nil {
		return AuthResult{}, ErrSSONotConfigured
	}

	input, err := s.ssoClient.ExchangeCode(ctx, code, nonce, redirectURL)
	if err != nil {
		return AuthResult{}, err
	}

	if !s.ssoClient.AutoRegister() {
		user, err := s.repo.FindUserBySSO(ctx, input.Provider, input.ExternalSubject)
		if err != nil {
			return AuthResult{}, err
		}
		return s.issueSession(ctx, user, userAgent, ipAddress)
	}

	user, _, err := s.repo.FindOrCreateUserBySSO(ctx, input)
	if err != nil {
		return AuthResult{}, err
	}
	return s.issueSession(ctx, user, userAgent, ipAddress)
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

func (s *AuthService) ListShareableUsers(ctx context.Context, actor domain.User) ([]domain.User, error) {
	if !actor.CanUseSystem() {
		return nil, ErrPermissionDenied
	}
	return s.repo.ListFriends(ctx, actor.ID)
}

func (s *AuthService) ListIncomingFriendRequests(ctx context.Context, actor domain.User) ([]domain.User, error) {
	if !actor.CanUseSystem() {
		return nil, ErrPermissionDenied
	}
	return s.repo.ListIncomingFriendRequests(ctx, actor.ID)
}

func (s *AuthService) RequestFriendByEmail(ctx context.Context, actor domain.User, email string) (domain.User, error) {
	if !actor.CanUseSystem() {
		return domain.User{}, ErrPermissionDenied
	}
	return s.repo.RequestFriendByEmail(ctx, actor.ID, email)
}

func (s *AuthService) AcceptFriendRequest(ctx context.Context, actor domain.User, userID string) (domain.User, error) {
	return s.updateFriendRequest(ctx, actor, userID, true)
}

func (s *AuthService) RejectFriendRequest(ctx context.Context, actor domain.User, userID string) (domain.User, error) {
	return s.updateFriendRequest(ctx, actor, userID, false)
}

func (s *AuthService) updateFriendRequest(ctx context.Context, actor domain.User, userID string, accept bool) (domain.User, error) {
	if !actor.CanUseSystem() {
		return domain.User{}, ErrPermissionDenied
	}
	parsedUserID, err := parseUUID(userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("invalid user id: %w", err)
	}
	var user domain.User
	if accept {
		user, err = s.repo.AcceptFriendRequest(ctx, actor.ID, parsedUserID)
	} else {
		user, err = s.repo.RejectFriendRequest(ctx, actor.ID, parsedUserID)
	}
	if err != nil {
		if errors.Is(err, repository.ErrFriendNotFound) {
			return domain.User{}, ErrFriendNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *AuthService) issueSession(ctx context.Context, user domain.User, userAgent, ipAddress string) (AuthResult, error) {
	if !user.IsActive {
		return AuthResult{}, ErrInvalidCredentials
	}

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

func parseUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, err
	}
	return parsed, nil
}
