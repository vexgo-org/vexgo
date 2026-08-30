// Package sso implements OAuth2 / OIDC login (GitHub, Google, OIDC) and
// SSO account binding.
package sso

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────
// State cache (CSRF protection, no cookie needed)
// ─────────────────────────────────────────────

const (
	stateLength = 16
	stateExpire = 5 * time.Minute
)

// StateStore persists one-time OAuth state values until a deadline. It is
// satisfied by the cache backends (internal/cache) without sso depending on
// them; the in-process default below keeps single-instance deployments
// working without configuration, while a distributed store lets multiple
// instances share one state.
type StateStore interface {
	// Set stores value under key until the ttl elapses.
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// GetDel atomically retrieves and removes the value stored under key,
	// enforcing one-time use.
	GetDel(ctx context.Context, key string) (value string, ok bool, err error)
}

// stateValue binds a state to the client IP and the requested callback
// method. Client IPs never contain the separator.
type stateValue struct {
	ip     string
	method string
}

const stateValueSep = "|"

func (v stateValue) encode() string {
	return v.ip + stateValueSep + v.method
}

func decodeStateValue(raw string) (stateValue, bool) {
	ip, method, found := strings.Cut(raw, stateValueSep)
	if !found {
		return stateValue{}, false
	}
	return stateValue{ip: ip, method: method}, true
}

// memoryStateStore keeps state values in the process. Expired entries are
// swept on Set so abandoned flows cannot grow the map without bound.
type memoryStateStore struct {
	mu      sync.Mutex
	entries map[string]memoryStateEntry
}

type memoryStateEntry struct {
	value   string
	expires time.Time
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{entries: make(map[string]memoryStateEntry)}
}

func (m *memoryStateStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for k, entry := range m.entries {
		if now.After(entry.expires) {
			delete(m.entries, k)
		}
	}
	m.entries[key] = memoryStateEntry{value: value, expires: now.Add(ttl)}
	return nil
}

// GetDel retrieves and removes the stored value in one step.
func (m *memoryStateStore) GetDel(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[key]
	delete(m.entries, key) // one-time use
	if !ok || time.Now().After(entry.expires) {
		return "", false, nil
	}
	return entry.value, true, nil
}

// generateState creates a random one-time state for CSRF protection and
// stores it keyed by provider, bound to the client IP and requested method.
func (s *Service) generateState(ctx context.Context, provider, ip, method string) (string, error) {
	b := make([]byte, stateLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate sso state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(b)[:stateLength]

	value := stateValue{ip: ip, method: method}.encode()
	if err := s.states.Set(ctx, provider+"_"+state, value, stateExpire); err != nil {
		return "", fmt.Errorf("store sso state: %w", err)
	}
	return state, nil
}

// verifyState consumes and validates a state previously issued by
// generateState: it must exist (one-time use), match the client IP and not be
// expired. The stored method is returned on success. Store errors fail
// closed, keeping CSRF protection intact during a backend outage.
func (s *Service) verifyState(ctx context.Context, provider, ip, state string) (method string, ok bool) {
	value, found, err := s.states.GetDel(ctx, provider+"_"+state)
	if err != nil {
		slog.Error("sso state store unavailable, rejecting callback", "err", err)
		return "", false
	}
	if !found {
		return "", false
	}
	v, ok := decodeStateValue(value)
	if !ok || v.ip != ip {
		return "", false
	}
	return v.method, true
}

// Deps holds the dependencies required by the sso domain.
type Deps struct {
	DB        *gorm.DB
	SSO       *config.SSOConfig
	JWTSecret []byte
	// Mailer is consulted when deciding whether a local account may be linked
	// to an SSO identity by email: with SMTP disabled there is no verification
	// flow, so unverified local accounts stay linkable (see linkableByEmail).
	Mailer *mailer.Service

	// BaseURL is the public origin of this instance (cfg.BaseURL). When set,
	// OAuth callback URLs are built from it instead of the request host.
	BaseURL string

	// StateStore persists one-time OAuth state values. nil keeps them
	// in-process; a distributed store shares one state across instances.
	StateStore StateStore
}

// Service contains the business logic of the sso domain.
type Service struct {
	repo      Repository
	sso       *config.SSOConfig
	jwtSecret []byte
	mailer    *mailer.Service
	baseURL   string
	states    StateStore
}

// NewService creates an sso service with the given dependencies.
func NewService(deps Deps) *Service {
	states := deps.StateStore
	if states == nil {
		states = newMemoryStateStore()
	}
	return &Service{repo: NewRepository(deps.DB), sso: deps.SSO, jwtSecret: deps.JWTSecret, mailer: deps.Mailer, baseURL: deps.BaseURL, states: states}
}

// linkableByEmail reports whether a local account may be linked to an SSO
// identity whose email the provider verified. The local account must be
// verified itself, unless SMTP is disabled and no verification flow exists.
func (s *Service) linkableByEmail(ctx context.Context, u *model.User) (bool, error) {
	if u.EmailVerified {
		return true, nil
	}
	if s.mailer == nil {
		return false, nil
	}
	enabled, err := s.mailer.Enabled(ctx)
	if err != nil {
		// The verification state of the account cannot be confirmed; stay
		// strict rather than linking on a maybe.
		return false, nil
	}
	return !enabled, nil
}

// Providers returns the list of enabled providers and whether local login is
// allowed.
func (s *Service) Providers() ([]string, bool) {
	enabled := make([]string, 0, 3)
	if s.sso.GitHub.ClientID != "" {
		enabled = append(enabled, "github")
	}
	if s.sso.Google.ClientID != "" {
		enabled = append(enabled, "google")
	}
	if s.sso.OIDC.Enabled && s.sso.OIDC.ClientID != "" {
		enabled = append(enabled, "oidc")
	}
	return enabled, s.sso.AllowLocalLogin
}

// LoginRedirect starts the OAuth2 authorization flow. It returns the
// authorization URL, or a non-2xx status and an error message (exact original
// response) when the request is invalid or the provider is not configured.
func (s *Service) LoginRedirect(c *gin.Context, provider, method string) (authURL string, status int, message string) {
	if !isValidMethod(method) {
		return "", http.StatusBadRequest, "invalid method, use sso_get_token or get_sso_id"
	}

	redirectURI := s.callbackURI(c, provider)
	state, err := s.generateState(c.Request.Context(), provider, c.ClientIP(), method)
	if err != nil {
		// The error is translated to a generic user-facing message here, so
		// log it: it is otherwise invisible (typically a state store outage).
		slog.Warn("sso login redirect failed", "provider", provider, "err", err)
		return "", http.StatusInternalServerError, "failed to start login flow"
	}

	switch provider {
	case "github":
		if s.sso.GitHub.ClientID == "" {
			return "", http.StatusNotImplemented, "GitHub SSO not configured"
		}
		authURL = s.githubOAuth2Config(redirectURI).AuthCodeURL(state)
	case "google":
		if s.sso.Google.ClientID == "" {
			return "", http.StatusNotImplemented, "Google SSO not configured"
		}
		authURL = s.googleOAuth2Config(redirectURI).AuthCodeURL(state)
	case "oidc":
		if !s.sso.OIDC.Enabled || s.sso.OIDC.ClientID == "" {
			return "", http.StatusNotImplemented, "OIDC SSO not configured"
		}
		oidcCfg, err := s.oidcOAuth2Config(c.Request.Context(), redirectURI)
		if err != nil {
			slog.Warn("oidc discovery failed", "err", err)
			return "", http.StatusInternalServerError, "SSO provider is temporarily unavailable"
		}
		authURL = oidcCfg.AuthCodeURL(state)
	default:
		return "", http.StatusBadRequest, "unsupported provider: " + provider
	}

	return authURL, 0, ""
}

// Callback handles the OAuth2 callback for all providers. It returns the
// postMessage payload (either the provider-scoped sso_id for binding or the
// issued JWT), or an error message when the exchange fails.
func (s *Service) Callback(c *gin.Context, provider, state, code string) (payload map[string]string, message string) {
	method, ok := s.verifyState(c.Request.Context(), provider, c.ClientIP(), state)
	if !ok {
		return nil, "invalid or expired state parameter"
	}
	if !isValidMethod(method) {
		return nil, "invalid method in state"
	}
	if code == "" {
		return nil, "no authorization code provided"
	}

	redirectURI := s.callbackURI(c, provider)
	info, err := s.exchange(c, provider, code, redirectURI)
	if err != nil {
		// Details may contain provider response bodies and internal URLs;
		// log them server-side and return a generic message to the client.
		slog.Warn("sso exchange failed", "provider", provider, "err", err)
		return nil, "login with the provider failed, please try again"
	}

	// get_sso_id: just return the provider-scoped ID so the frontend can bind it
	if method == "get_sso_id" {
		return map[string]string{
			"sso_id": provider + ":" + info.providerID,
		}, ""
	}

	// sso_get_token: find or create local user, then issue JWT
	user, err := s.FindOrCreateUser(c.Request.Context(), provider, info)
	if err != nil {
		slog.Warn("sso account resolution failed", "provider", provider, "err", err)
		return nil, "failed to resolve the account for this login"
	}
	token, err := auth.IssueJWT(user, s.jwtSecret)
	if err != nil {
		return nil, "failed to issue token"
	}
	return map[string]string{"token": token}, ""
}

// ─────────────────────────────────────────────
// OAuth2 configs (built per-request to allow dynamic redirect URI)
// ─────────────────────────────────────────────

// githubOAuth2Config builds the oauth2.Config for GitHub OAuth.
func (s *Service) githubOAuth2Config(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.sso.GitHub.ClientID,
		ClientSecret: s.sso.GitHub.ClientSecret,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"read:user", "user:email"},
		RedirectURL:  redirectURI,
	}
}

// googleOAuth2Config builds the oauth2.Config for Google OAuth.
func (s *Service) googleOAuth2Config(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.sso.Google.ClientID,
		ClientSecret: s.sso.Google.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"openid", "profile", "email"},
		RedirectURL:  redirectURI,
	}
}

// oidcOAuth2Config builds the oauth2.Config for OIDC.
// When IssuerURL is set it uses OIDC discovery to obtain endpoints automatically;
// AuthURL / TokenURL are used as a manual fallback for non-standard providers.
func (s *Service) oidcOAuth2Config(ctx context.Context, redirectURI string) (*oauth2.Config, error) {
	cfg := s.sso.OIDC

	var endpoint oauth2.Endpoint
	if cfg.IssuerURL != "" {
		provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("OIDC discovery failed for %s: %w", cfg.IssuerURL, err)
		}
		endpoint = provider.Endpoint()
	} else if cfg.AuthURL != "" && cfg.TokenURL != "" {
		endpoint = oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		}
	} else {
		return nil, errors.New("OIDC: set either OIDC_ISSUER_URL (recommended) or both OIDC_AUTH_URL and OIDC_TOKEN_URL")
	}

	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     endpoint,
		Scopes:       cfg.Scopes,
		RedirectURL:  redirectURI,
	}, nil
}

// callbackURI builds the absolute callback URL for the given provider.
// When BASE_URL is set (e.g. https://vexgo.yzlab.de), it takes priority over
// auto-detection from the request host. This is needed when running behind a
// reverse proxy or when the public domain differs from the listen address.
func (s *Service) callbackURI(c *gin.Context, provider string) string {
	if base := s.baseURL; base != "" {
		return fmt.Sprintf("%s/api/sso/%s/callback", strings.TrimRight(base, "/"), provider)
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	u := url.URL{
		Scheme: scheme,
		Host:   c.Request.Host,
	}
	return u.JoinPath("api", "sso", provider, "callback").String()
}

// ─────────────────────────────────────────────
// Per-provider exchange
// ─────────────────────────────────────────────

type ssoUserInfo struct {
	providerID string
	username   string
	email      string
	avatar     string
	// emailVerified reports whether the provider confirmed ownership of
	// info.email. It is only true on explicit confirmation (GitHub /user/emails
	// verified flag, Google/OIDC email_verified claim); providers that stay
	// silent count as unverified so email-based account linking stays safe.
	emailVerified bool
}

// exchange delegates the OAuth2 code exchange to the matching provider
// implementation and returns the normalized user info.
func (s *Service) exchange(c *gin.Context, provider, code, redirectURI string) (*ssoUserInfo, error) {
	switch provider {
	case "github":
		return s.exchangeGitHub(c, code, redirectURI)
	case "google":
		return s.exchangeGoogle(c, code, redirectURI)
	case "oidc":
		return s.exchangeOIDC(c, code, redirectURI)
	default:
		return nil, errors.New("unsupported provider: " + provider)
	}
}

// exchangeGitHub exchanges the code with GitHub and fetches the user profile,
// falling back to the emails endpoint when the primary email is not exposed.
func (s *Service) exchangeGitHub(c *gin.Context, code, redirectURI string) (*ssoUserInfo, error) {
	tok, err := s.githubOAuth2Config(redirectURI).Exchange(c.Request.Context(), code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	body, err := apiGet("https://api.github.com/user", tok.AccessToken, "token")
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	info := &ssoUserInfo{}
	if id, ok := data["id"].(float64); ok {
		info.providerID = fmt.Sprintf("%.0f", id)
	}
	info.username, _ = data["login"].(string)
	if name, _ := data["name"].(string); name != "" {
		info.username = name
	}
	info.email, _ = data["email"].(string)
	info.avatar, _ = data["avatar_url"].(string)

	// GitHub may omit email in /user when it is set to private. The /user
	// endpoint alone does not prove ownership, so confirmation comes from
	// /user/emails: either the address we got is flagged verified there, or
	// the account's verified primary email replaces it.
	if emails := fetchGitHubEmails(tok.AccessToken); emails != nil {
		if isVerifiedGitHubEmail(emails, info.email) {
			info.emailVerified = true
		} else if primary := verifiedGitHubPrimaryEmail(emails); primary != "" {
			info.email = primary
			info.emailVerified = true
		}
	}
	if info.email == "" {
		info.email = fetchGitHubPrimaryEmail(tok.AccessToken)
		info.emailVerified = info.email != ""
	}
	if info.providerID == "" {
		return nil, errors.New("cannot get user ID from GitHub")
	}
	return info, nil
}

// exchangeGoogle exchanges the code with Google and fetches the userinfo.
func (s *Service) exchangeGoogle(c *gin.Context, code, redirectURI string) (*ssoUserInfo, error) {
	tok, err := s.googleOAuth2Config(redirectURI).Exchange(c.Request.Context(), code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	body, err := apiGet("https://www.googleapis.com/oauth2/v2/userinfo", tok.AccessToken, "bearer")
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	info := &ssoUserInfo{}
	info.providerID, _ = data["id"].(string)
	info.username, _ = data["name"].(string)
	info.email, _ = data["email"].(string)
	info.avatar, _ = data["picture"].(string)
	info.emailVerified, _ = data["email_verified"].(bool)

	if info.providerID == "" {
		return nil, errors.New("cannot get user ID from Google")
	}
	return info, nil
}

// exchangeOIDC exchanges the code with the OIDC provider, preferring id_token
// claims over a userinfo call, and enforces the verify-email and
// allowed-groups policies when configured.
func (s *Service) exchangeOIDC(c *gin.Context, code, redirectURI string) (*ssoUserInfo, error) {
	oidcCfg := s.sso.OIDC

	oauth2Cfg, err := s.oidcOAuth2Config(c.Request.Context(), redirectURI)
	if err != nil {
		return nil, err
	}
	tok, err := oauth2Cfg.Exchange(c.Request.Context(), code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	var claims map[string]any

	// Prefer id_token claims (avoids an extra round-trip to userinfo)
	if rawIDToken, ok := tok.Extra("id_token").(string); ok {
		if c, err := parseOIDCIDTokenClaims(rawIDToken); err == nil {
			claims = c
		}
	}

	// Fallback: hit userinfo endpoint
	if claims == nil {
		if oidcCfg.UserInfoURL == "" {
			return nil, errors.New("OIDC: id_token missing and OIDC_USERINFO_URL not configured")
		}
		body, err := apiGet(oidcCfg.UserInfoURL, tok.AccessToken, "bearer")
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &claims); err != nil {
			return nil, err
		}
	}

	info := s.claimsToUserInfo(claims)
	if info.providerID == "" {
		return nil, errors.New("cannot get user ID from OIDC provider")
	}

	// ── OIDC_VERIFY_EMAIL ────────────────────────────────────────────────────
	if oidcCfg.VerifyEmail {
		if verified, _ := claims["email_verified"].(bool); !verified {
			return nil, errors.New("email address has not been verified by the OIDC provider")
		}
	}

	// ── OIDC_ALLOWED_GROUPS ──────────────────────────────────────────────────
	if len(oidcCfg.AllowedGroups) > 0 {
		if !isInAllowedGroups(claims, oidcCfg.GroupClaim, oidcCfg.AllowedGroups) {
			return nil, errors.New("you are not in an allowed group")
		}
	}

	return info, nil
}

// isInAllowedGroups checks whether the groups claim contains at least one of the allowed groups.
func isInAllowedGroups(claims map[string]any, groupClaim string, allowed []string) bool {
	raw, ok := claims[groupClaim]
	if !ok {
		return false
	}
	// groups claim is typically []any (JSON array of strings)
	groups, ok := raw.([]any)
	if !ok {
		return false
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, g := range allowed {
		allowedSet[g] = struct{}{}
	}
	for _, g := range groups {
		if s, ok := g.(string); ok {
			if _, found := allowedSet[s]; found {
				return true
			}
		}
	}
	return false
}

// parseOIDCIDTokenClaims decodes the payload of the id_token without signature verification.
// Signature verification is skipped here because the token was obtained directly via
// a back-channel code exchange (not supplied by the user), so it is already trusted.
func parseOIDCIDTokenClaims(rawIDToken string) (map[string]any, error) {
	parts := strings.Split(rawIDToken, ".")
	if len(parts) < 2 {
		return nil, errors.New("malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// claimsToUserInfo maps OIDC claims to ssoUserInfo, honoring the configured
// claim names for email and display name.
func (s *Service) claimsToUserInfo(claims map[string]any) *ssoUserInfo {
	cfg := s.sso.OIDC
	info := &ssoUserInfo{}

	// subject / id
	for _, key := range []string{"sub", "id"} {
		if v, _ := claims[key].(string); v != "" {
			info.providerID = v
			break
		}
	}

	// name — respect OIDC_NAME_CLAIM, fall back to preferred_username
	if name, _ := claims[cfg.NameClaim].(string); name != "" {
		info.username = name
	}
	if info.username == "" {
		info.username, _ = claims["preferred_username"].(string)
	}

	// email — respect OIDC_EMAIL_CLAIM
	info.email, _ = claims[cfg.EmailClaim].(string)
	info.avatar, _ = claims["picture"].(string)
	if verified, ok := claims["email_verified"].(bool); ok {
		info.emailVerified = verified
	}
	return info
}

// ─────────────────────────────────────────────
// User find-or-create
// ─────────────────────────────────────────────

// FindOrCreateUser resolves an SSO identity to a local user: first by exact
// SSO binding, then by email match, and finally by auto-registering a new
// guest user. The binding is persisted so future logins skip the fallbacks.
func (s *Service) FindOrCreateUser(ctx context.Context, provider string, info *ssoUserInfo) (*model.User, error) {
	// 1. Exact SSO binding match
	if binding, err := s.repo.FindSSOBinding(ctx, provider, info.providerID); err == nil {
		user, err := s.repo.FindUserByID(ctx, binding.UserID)
		if err != nil {
			return nil, errors.New("user account not found")
		}
		// Update last login time
		user.LastLoginAt = time.Now()
		if err := s.repo.SaveUser(ctx, user); err != nil {
			return nil, err
		}
		return user, nil
	}

	// 2. Email match → link to existing account. Linking is only allowed when
	// the provider confirmed the email (email_verified) AND the local account
	// is verified itself — unless the instance has no verification flow at
	// all (SMTP disabled), matching the fail-open stance of local login.
	// Without these checks an attacker controlling an unverified SSO identity
	// carrying a victim's email could log in as the victim.
	var user *model.User
	emailTaken := false
	if info.email != "" && info.emailVerified {
		u, err := s.repo.FindUserByEmail(ctx, info.email)
		switch {
		case err == nil:
			emailTaken = true
			linkable, err := s.linkableByEmail(ctx, u)
			if err != nil {
				return nil, err
			}
			if linkable {
				user = u
				// Update last login time
				user.LastLoginAt = time.Now()
				if err := s.repo.SaveUser(ctx, user); err != nil {
					return nil, err
				}
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return nil, err
		}
	}

	// 3. Auto-register new user
	if user == nil {
		if emailTaken {
			return nil, errors.New("an account with this email already exists; verify your email first or log in with a password")
		}
		username := s.generateUsername(ctx, info.username, info.email)
		u := model.User{
			Username:        username,
			Email:           info.email,
			Role:            model.RoleGuest,
			PasswordVersion: 0,
			// No password set — this user can only log in via SSO
		}
		if err := s.repo.CreateUser(ctx, &u); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		user = &u
	}

	// Persist binding so future logins skip steps 2-3
	if err := s.repo.CreateBinding(ctx, &model.SSOBinding{
		UserID:     user.ID,
		Provider:   provider,
		ProviderID: info.providerID,
		Email:      info.email,
		Name:       info.username,
		Avatar:     info.avatar,
	}); err != nil {
		return nil, err
	}

	return user, nil
}

// generateUsername derives a unique username from the provider name or email.
func (s *Service) generateUsername(ctx context.Context, name, email string) string {
	base := name
	if base == "" {
		if idx := strings.Index(email, "@"); idx > 0 {
			base = email[:idx]
		}
	}
	if base == "" {
		base = "user"
	}
	var sb strings.Builder
	for _, ch := range base {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			sb.WriteRune(ch)
		}
	}
	if sb.Len() == 0 {
		sb.WriteString("user")
	}
	candidate := sb.String()

	for suffix := 0; ; suffix++ {
		username := candidate
		if suffix > 0 {
			username = fmt.Sprintf("%s%d", candidate, suffix)
		}
		count, _ := s.repo.CountUsersByUsername(ctx, username)
		if count == 0 {
			return username
		}
	}
}

// ─────────────────────────────────────────────
// Utilities
// ─────────────────────────────────────────────

func apiGet(url, accessToken, scheme string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(scheme) == "token" {
		req.Header.Set("Authorization", "token "+accessToken) // GitHub style
	} else {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// fetchGitHubEmails returns the account's email entries from GitHub's
// /user/emails endpoint, or nil when unavailable.
func fetchGitHubEmails(accessToken string) []map[string]any {
	body, err := apiGet("https://api.github.com/user/emails", accessToken, "token")
	if err != nil {
		return nil
	}
	var emails []map[string]any
	if err := json.Unmarshal(body, &emails); err != nil {
		return nil
	}
	return emails
}

// isVerifiedGitHubEmail reports whether the given address is flagged verified
// in a /user/emails payload.
func isVerifiedGitHubEmail(emails []map[string]any, email string) bool {
	for _, e := range emails {
		if addr, _ := e["email"].(string); addr != email {
			continue
		}
		if verified, _ := e["verified"].(bool); verified {
			return true
		}
	}
	return false
}

// verifiedGitHubPrimaryEmail returns the primary, verified email address from
// a /user/emails payload, or an empty string when unavailable.
func verifiedGitHubPrimaryEmail(emails []map[string]any) string {
	for _, e := range emails {
		if primary, _ := e["primary"].(bool); !primary {
			continue
		}
		if verified, _ := e["verified"].(bool); !verified {
			continue
		}
		if email, _ := e["email"].(string); email != "" {
			return email
		}
	}
	return ""
}

// fetchGitHubPrimaryEmail returns the primary, verified email address from
// GitHub's /user/emails endpoint, or an empty string when unavailable.
func fetchGitHubPrimaryEmail(accessToken string) string {
	return verifiedGitHubPrimaryEmail(fetchGitHubEmails(accessToken))
}

// isValidMethod reports whether the SSO flow method is supported.
func isValidMethod(method string) bool {
	return method == "sso_get_token" || method == "get_sso_id"
}
