// backend/config/sso.go
package config

import (
	"os"
	"strconv"
	"strings"
)

// SSOProviderConfig holds OAuth2 credentials for a single provider.
type SSOProviderConfig struct {
	ClientID     string
	ClientSecret string
}

// OIDCConfig holds the full configuration for OIDC-based SSO.
// Supports standard OIDC discovery (IssuerURL) as well as manual endpoint
// override (AuthURL / TokenURL) for non-standard providers.
type OIDCConfig struct {
	SSOProviderConfig

	// ── Required ─────────────────────────────────────────────────────────────
	Enabled   bool   // OIDC_ENABLED
	IssuerURL string // OIDC_ISSUER_URL  — used for OIDC discovery (.well-known/openid-configuration)

	// ── Manual endpoint override (only needed when discovery is unavailable) ─
	AuthURL     string // OIDC_AUTH_URL
	TokenURL    string // OIDC_TOKEN_URL
	UserInfoURL string // OIDC_USERINFO_URL

	// ── Scopes ───────────────────────────────────────────────────────────────
	// Default: "openid profile email"
	// Some providers need extra scopes, e.g. "openid profile email groups"
	Scopes []string // OIDC_SCOPES  (space-separated)

	// ── Claims ───────────────────────────────────────────────────────────────
	// Override if your provider uses non-standard claim names.
	EmailClaim string // OIDC_EMAIL_CLAIM  (default: "email")
	NameClaim  string // OIDC_NAME_CLAIM   (default: "name")
	GroupClaim string // OIDC_GROUP_CLAIM  (default: "groups")

	// ── Access control ───────────────────────────────────────────────────────
	// Comma-separated list of groups allowed to log in.
	// Empty = allow all users.
	AllowedGroups []string // OIDC_ALLOWED_GROUPS  (comma-separated)

	// ── UX ───────────────────────────────────────────────────────────────────
	AutoRedirect bool // OIDC_AUTO_REDIRECT  — skip login page, go straight to provider
	VerifyEmail  bool // OIDC_VERIFY_EMAIL   — require email_verified=true in token
}

// SSOConfig is the full SSO configuration (GitHub / Google / OIDC).
// The zero value disables every provider.
type SSOConfig struct {
	// Simple OAuth2 providers
	GitHub SSOProviderConfig
	Google SSOProviderConfig

	// Full OIDC (Keycloak / Authentik / Authelia / Okta / Casdoor / university SSO …)
	OIDC OIDCConfig

	// Global option: set false to force SSO-only (disable password login)
	AllowLocalLogin bool // ALLOW_LOCAL_LOGIN (default: true)
}

// LoadSSOFromEnv populates cfg.SSO from environment variables.
// This is the baseline; config file values override via LoadFromConfig.
func (cfg *Config) LoadSSOFromEnv() {
	cfg.SSO = SSOConfig{
		GitHub: SSOProviderConfig{
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		},
		Google: SSOProviderConfig{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		},
		OIDC: OIDCConfig{
			Enabled:   parseBool("OIDC_ENABLED", false),
			IssuerURL: os.Getenv("OIDC_ISSUER_URL"),
			SSOProviderConfig: SSOProviderConfig{
				ClientID:     os.Getenv("OIDC_CLIENT_ID"),
				ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
			},
			AuthURL:       os.Getenv("OIDC_AUTH_URL"),
			TokenURL:      os.Getenv("OIDC_TOKEN_URL"),
			UserInfoURL:   os.Getenv("OIDC_USERINFO_URL"),
			Scopes:        parseScopes("OIDC_SCOPES", []string{"openid", "profile", "email"}),
			EmailClaim:    envDefault("OIDC_EMAIL_CLAIM", "email"),
			NameClaim:     envDefault("OIDC_NAME_CLAIM", "name"),
			GroupClaim:    envDefault("OIDC_GROUP_CLAIM", "groups"),
			AllowedGroups: parseCommaSeparated("OIDC_ALLOWED_GROUPS"),
			AutoRedirect:  parseBool("OIDC_AUTO_REDIRECT", false),
			VerifyEmail:   parseBool("OIDC_VERIFY_EMAIL", false),
		},
		AllowLocalLogin: parseBool("ALLOW_LOCAL_LOGIN", true),
	}
}

// LoadSSOFromConfig applies config file values over the env-loaded SSO.
// Only fields explicitly set in the config file are applied.
func (cfg *Config) LoadSSOFromConfig() {
	// GitHub OAuth
	if cfg.fileSet["github_client_id"] || cfg.GitHubClientID != "" {
		cfg.SSO.GitHub.ClientID = cfg.GitHubClientID
	}
	if cfg.fileSet["github_client_secret"] || cfg.GitHubClientSecret != "" {
		cfg.SSO.GitHub.ClientSecret = cfg.GitHubClientSecret
	}

	// Google OAuth
	if cfg.fileSet["google_client_id"] || cfg.GoogleClientID != "" {
		cfg.SSO.Google.ClientID = cfg.GoogleClientID
	}
	if cfg.fileSet["google_client_secret"] || cfg.GoogleClientSecret != "" {
		cfg.SSO.Google.ClientSecret = cfg.GoogleClientSecret
	}

	// OIDC Enabled
	if cfg.fileSet["oidc_enabled"] {
		cfg.SSO.OIDC.Enabled = cfg.OIDCEnabled
	}

	// OIDC Client credentials
	if cfg.OIDCClientID != "" {
		cfg.SSO.OIDC.ClientID = cfg.OIDCClientID
	}
	if cfg.OIDCClientSecret != "" {
		cfg.SSO.OIDC.ClientSecret = cfg.OIDCClientSecret
	}

	// OIDC endpoints
	if cfg.OIDCIssuerURL != "" {
		cfg.SSO.OIDC.IssuerURL = cfg.OIDCIssuerURL
	}
	if cfg.OIDCAuthURL != "" {
		cfg.SSO.OIDC.AuthURL = cfg.OIDCAuthURL
	}
	if cfg.OIDCTokenURL != "" {
		cfg.SSO.OIDC.TokenURL = cfg.OIDCTokenURL
	}
	if cfg.OIDCUserInfoURL != "" {
		cfg.SSO.OIDC.UserInfoURL = cfg.OIDCUserInfoURL
	}

	// OIDC Scopes
	if cfg.OIDCScopes != "" {
		cfg.SSO.OIDC.Scopes = strings.Fields(cfg.OIDCScopes)
	}

	// OIDC Claim names
	if cfg.OIDCEmailClaim != "" {
		cfg.SSO.OIDC.EmailClaim = cfg.OIDCEmailClaim
	}
	if cfg.OIDCNameClaim != "" {
		cfg.SSO.OIDC.NameClaim = cfg.OIDCNameClaim
	}
	if cfg.OIDCGroupClaim != "" {
		cfg.SSO.OIDC.GroupClaim = cfg.OIDCGroupClaim
	}

	// OIDC Allowed groups
	if cfg.OIDCAllowedGroups != "" {
		cfg.SSO.OIDC.AllowedGroups = parseCommaSeparatedFromString(cfg.OIDCAllowedGroups)
	}

	// OIDC UX options
	if cfg.fileSet["oidc_auto_redirect"] {
		cfg.SSO.OIDC.AutoRedirect = cfg.OIDCAutoRedirect
	}
	if cfg.fileSet["oidc_verify_email"] {
		cfg.SSO.OIDC.VerifyEmail = cfg.OIDCVerifyEmail
	}

	// Global options
	if cfg.fileSet["allow_local_login"] {
		cfg.SSO.AllowLocalLogin = cfg.AllowLocalLogin
	}
}

// parseCommaSeparatedFromString parses a comma-separated list from a string.
func parseCommaSeparatedFromString(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ── helpers ───────────────────────────────────────────────────────────────────

func envDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// parseBool parses a boolean environment variable, falling back to defaultVal
// when the variable is unset or not a valid boolean.
func parseBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

// parseScopes parses a space-separated scope string.
// Falls back to defaultScopes when the env var is unset.
func parseScopes(key string, defaultScopes []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return defaultScopes
	}
	return strings.Fields(v)
}

// parseCommaSeparated parses a comma-separated list, trimming whitespace.
// Returns nil (not an empty slice) when the env var is unset, so callers
// can use `len(AllowedGroups) == 0` to mean "allow all".
func parseCommaSeparated(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	return result
}
