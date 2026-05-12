package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"todo/internal/repository"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	ErrSSONotConfigured = errors.New("sso not configured")
	ErrInvalidSSOLogin  = errors.New("invalid sso login")
)

var invalidUsernameRunePattern = regexp.MustCompile(`[^a-z0-9._-]+`)

type SSOConfig struct {
	ProviderName string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AutoRegister bool
	AutoApprove  bool
}

type SSOClient struct {
	providerName string
	autoRegister bool
	autoApprove  bool
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

type ssoClaims struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
}

func NewSSOClient(ctx context.Context, cfg SSOConfig) (*SSOClient, error) {
	if strings.TrimSpace(cfg.IssuerURL) == "" &&
		strings.TrimSpace(cfg.ClientID) == "" &&
		strings.TrimSpace(cfg.ClientSecret) == "" &&
		strings.TrimSpace(cfg.RedirectURL) == "" {
		return nil, nil
	}
	if strings.TrimSpace(cfg.IssuerURL) == "" {
		return nil, fmt.Errorf("SSO_ISSUER_URL must not be empty")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, fmt.Errorf("SSO_CLIENT_ID must not be empty")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("SSO_CLIENT_SECRET must not be empty")
	}
	if strings.TrimSpace(cfg.RedirectURL) == "" {
		return nil, fmt.Errorf("SSO_REDIRECT_URL must not be empty")
	}

	provider, err := oidc.NewProvider(ctx, strings.TrimSpace(cfg.IssuerURL))
	if err != nil {
		return nil, fmt.Errorf("discover sso provider: %w", err)
	}

	scopes := normalizeScopes(cfg.Scopes)
	oauth2Config := oauth2.Config{
		ClientID:     strings.TrimSpace(cfg.ClientID),
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		Endpoint:     provider.Endpoint(),
		RedirectURL:  strings.TrimSpace(cfg.RedirectURL),
		Scopes:       scopes,
	}

	providerName := strings.TrimSpace(cfg.ProviderName)
	if providerName == "" {
		providerName = "soid"
	}

	return &SSOClient{
		providerName: providerName,
		autoRegister: cfg.AutoRegister,
		autoApprove:  cfg.AutoApprove,
		oauth2Config: oauth2Config,
		verifier:     provider.Verifier(&oidc.Config{ClientID: strings.TrimSpace(cfg.ClientID)}),
	}, nil
}

func (c *SSOClient) AuthCodeURL(state, nonce, redirectURL string) string {
	config := c.oauthConfigForRedirect(redirectURL)
	return config.AuthCodeURL(
		state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", nonce),
	)
}

func (c *SSOClient) ExchangeCode(ctx context.Context, code, nonce, redirectURL string) (repository.SSOUserInput, error) {
	if strings.TrimSpace(code) == "" {
		return repository.SSOUserInput{}, fmt.Errorf("%w: missing code", ErrInvalidSSOLogin)
	}
	if strings.TrimSpace(nonce) == "" {
		return repository.SSOUserInput{}, fmt.Errorf("%w: missing nonce", ErrInvalidSSOLogin)
	}

	config := c.oauthConfigForRedirect(redirectURL)
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return repository.SSOUserInput{}, fmt.Errorf("%w: exchange code: %v", ErrInvalidSSOLogin, err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return repository.SSOUserInput{}, fmt.Errorf("%w: missing id_token", ErrInvalidSSOLogin)
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return repository.SSOUserInput{}, fmt.Errorf("%w: verify id_token: %v", ErrInvalidSSOLogin, err)
	}
	if idToken.Nonce != nonce {
		return repository.SSOUserInput{}, fmt.Errorf("%w: nonce mismatch", ErrInvalidSSOLogin)
	}

	var claims ssoClaims
	if err := idToken.Claims(&claims); err != nil {
		return repository.SSOUserInput{}, fmt.Errorf("%w: decode claims: %v", ErrInvalidSSOLogin, err)
	}

	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return repository.SSOUserInput{}, fmt.Errorf("%w: missing subject", ErrInvalidSSOLogin)
	}

	username := buildSSOUsername(claims)
	displayName := buildSSODisplayName(claims, username)

	return repository.SSOUserInput{
		Provider:        c.providerName,
		ExternalSubject: subject,
		Username:        username,
		DisplayName:     displayName,
		Email:           normalizeEmail(claims.Email),
		AutoApprove:     c.autoApprove,
	}, nil
}

func (c *SSOClient) AutoRegister() bool {
	return c.autoRegister
}

func (c *SSOClient) oauthConfigForRedirect(redirectURL string) oauth2.Config {
	config := c.oauth2Config
	if normalized := strings.TrimSpace(redirectURL); normalized != "" {
		config.RedirectURL = normalized
	}
	return config
}

func normalizeScopes(scopes []string) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(scopes)+1)
	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		values = append(values, normalized)
	}
	if !seen[oidc.ScopeOpenID] {
		values = append([]string{oidc.ScopeOpenID}, values...)
	}
	return values
}

func buildSSOUsername(claims ssoClaims) string {
	candidates := []string{
		claims.PreferredUsername,
		emailLocalPart(claims.Email),
		claims.Name,
		claims.Subject,
	}
	for _, candidate := range candidates {
		username := normalizeSSOUsername(candidate)
		if _, err := validateUsername(username); err == nil {
			return username
		}
	}

	subject := invalidUsernameRunePattern.ReplaceAllString(strings.ToLower(claims.Subject), "")
	if len(subject) > 24 {
		subject = subject[:24]
	}
	return normalizeSSOUsername("user-" + subject)
}

func normalizeSSOUsername(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimSuffix(strings.TrimPrefix(normalized, "@"), ".")
	normalized = invalidUsernameRunePattern.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, ".-_")
	if normalized == "" || !isASCIILetterOrDigit(rune(normalized[0])) {
		normalized = "user-" + normalized
	}
	if len(normalized) > 32 {
		normalized = normalized[:32]
		normalized = strings.TrimRight(normalized, ".-_")
	}
	for len(normalized) < 3 {
		normalized += "0"
	}
	return normalized
}

func buildSSODisplayName(claims ssoClaims, username string) string {
	for _, candidate := range []string{claims.Name, claims.PreferredUsername, claims.Email, username} {
		normalized := strings.TrimSpace(candidate)
		if normalized == "" {
			continue
		}
		runes := []rune(normalized)
		if len(runes) > 32 {
			normalized = string(runes[:32])
		}
		if _, err := validateDisplayName(normalized); err == nil {
			return normalized
		}
	}
	return username
}

func emailLocalPart(value string) string {
	email := normalizeEmail(value)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 {
		return ""
	}
	return email[:at]
}

func normalizeEmail(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return ""
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return ""
	}
	return normalized
}

func isASCIILetterOrDigit(value rune) bool {
	return unicode.IsDigit(value) || ('a' <= value && value <= 'z')
}
