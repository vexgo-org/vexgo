package config

import (
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

// buildSSO derives cfg.SSO from the already-resolved flat fields. Layering
// and defaults are handled by viper: the claim names and scope list carry
// their defaults from the keyDefaults table by the time this runs.
func (cfg *Config) buildSSO() {
	cfg.SSO = SSOConfig{
		GitHub: SSOProviderConfig{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
		},
		Google: SSOProviderConfig{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
		},
		OIDC: OIDCConfig{
			Enabled:   cfg.OIDCEnabled,
			IssuerURL: cfg.OIDCIssuerURL,
			SSOProviderConfig: SSOProviderConfig{
				ClientID:     cfg.OIDCClientID,
				ClientSecret: cfg.OIDCClientSecret,
			},
			AuthURL:       cfg.OIDCAuthURL,
			TokenURL:      cfg.OIDCTokenURL,
			UserInfoURL:   cfg.OIDCUserInfoURL,
			Scopes:        strings.Fields(cfg.OIDCScopes),
			EmailClaim:    cfg.OIDCEmailClaim,
			NameClaim:     cfg.OIDCNameClaim,
			GroupClaim:    cfg.OIDCGroupClaim,
			AllowedGroups: parseCommaSeparatedFromString(cfg.OIDCAllowedGroups),
			AutoRedirect:  cfg.OIDCAutoRedirect,
			VerifyEmail:   cfg.OIDCVerifyEmail,
		},
		AllowLocalLogin: cfg.AllowLocalLogin,
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
