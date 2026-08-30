// Package config resolves server configuration from command-line flags, a
// YAML config file, environment variables, and built-in defaults, in that
// order of priority. Layering is delegated to viper; the cobra command that
// owns the flags lives in internal/cli. The package also computes runtime
// secrets (JWT, SSO) from the resolved values.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Built-in defaults, shared by the viper defaults table and the flag
// definitions in internal/cli, so help text and resolution cannot drift.
const (
	DefaultAddr    = "0.0.0.0"
	DefaultPort    = 3001
	DefaultDataDir = "./data"
)

// Config holds the resolved server configuration from command line arguments
// and/or config file.
type Config struct {
	Addr     string `mapstructure:"addr"`      // Address to listen on (e.g., "0.0.0.0" or "127.0.0.1")
	Port     int    `mapstructure:"port"`      // Port to listen on
	DataDir  string `mapstructure:"data_dir"`  // Data directory for storing sqlite database and media files
	LogLevel string `mapstructure:"log_level"` // Logging level: "debug", "info", "warn", "error"

	// CaptchaRateLimitPerMinute caps the requests per client IP per minute on
	// the unauthenticated captcha endpoints. 0 disables the limit.
	CaptchaRateLimitPerMinute int `mapstructure:"captcha_rate_limit_per_minute"`

	// AuthRateLimitPerMinute caps the requests per client IP per minute on the
	// unauthenticated auth endpoints (register, login, password reset and
	// verification resend) to slow down online brute-force and mail-bombing.
	// 0 disables the limit.
	AuthRateLimitPerMinute int `mapstructure:"auth_rate_limit_per_minute"`

	// BaseURL is the public origin of this instance (e.g., https://vexgo.example.com).
	// Used to build absolute links: SSO/OAuth callbacks and emailed
	// verification / password-reset / email-change links. Empty means the
	// request origin is used, which is unsafe behind untrusted clients.
	BaseURL string `mapstructure:"base_url"`

	// Database configuration
	DBType     string `mapstructure:"db_type"`     // Database type: "sqlite", "mysql", "mariadb", or "postgres"
	DBHost     string `mapstructure:"db_host"`     // Database host
	DBPort     int    `mapstructure:"db_port"`     // Database port
	DBUser     string `mapstructure:"db_user"`     // Database user
	DBPassword string `mapstructure:"db_password"` // Database password
	DBName     string `mapstructure:"db_name"`     // Database name
	DBSSLMode  string `mapstructure:"db_ssl_mode"` // PostgreSQL SSL mode (for postgres)

	// OIDC configuration
	OIDCEnabled       bool   `mapstructure:"oidc_enabled"`        // Enable OIDC login
	OIDCIssuerURL     string `mapstructure:"oidc_issuer_url"`     // Issuer URL for OIDC discovery
	OIDCClientID      string `mapstructure:"oidc_client_id"`      // OIDC client ID
	OIDCClientSecret  string `mapstructure:"oidc_client_secret"`  // OIDC client secret
	OIDCAuthURL       string `mapstructure:"oidc_auth_url"`       // Authorization endpoint (optional, for manual override)
	OIDCTokenURL      string `mapstructure:"oidc_token_url"`      // Token endpoint (optional, for manual override)
	OIDCUserInfoURL   string `mapstructure:"oidc_userinfo_url"`   // UserInfo endpoint (optional, for manual override)
	OIDCScopes        string `mapstructure:"oidc_scopes"`         // Space-separated scopes (default: "openid profile email")
	OIDCEmailClaim    string `mapstructure:"oidc_email_claim"`    // Claim name for email (default: "email")
	OIDCNameClaim     string `mapstructure:"oidc_name_claim"`     // Claim name for username (default: "name")
	OIDCGroupClaim    string `mapstructure:"oidc_group_claim"`    // Claim name for groups (default: "groups")
	OIDCAllowedGroups string `mapstructure:"oidc_allowed_groups"` // Comma-separated allowed groups (empty = allow all)
	OIDCAutoRedirect  bool   `mapstructure:"oidc_auto_redirect"`  // Auto-redirect to OIDC provider (skip login page)
	OIDCVerifyEmail   bool   `mapstructure:"oidc_verify_email"`   // Require email_verified=true in token

	// GitHub OAuth configuration
	GitHubClientID     string `mapstructure:"github_client_id"`     // GitHub OAuth App Client ID
	GitHubClientSecret string `mapstructure:"github_client_secret"` // GitHub OAuth App Client Secret

	// Google OAuth configuration
	GoogleClientID     string `mapstructure:"google_client_id"`     // Google OAuth 2.0 Client ID
	GoogleClientSecret string `mapstructure:"google_client_secret"` // Google OAuth 2.0 Client Secret

	// SettingsEncryptionKey is the passphrase used to encrypt secrets at rest
	// in the database (SMTP password, AI and comment-moderation API keys) with
	// AES-256-GCM. When empty, those secrets are stored in plaintext and a
	// warning is logged at startup.
	SettingsEncryptionKey string `mapstructure:"settings_encryption_key"`

	// Global options
	AllowLocalLogin bool `mapstructure:"allow_local_login"` // Allow password-based login (default: true)

	// Trusted proxies configuration
	TrustedProxies     []string `mapstructure:"trusted_proxies"`      // List of trusted proxy IPs/CIDRs (empty = trust none)
	BehindReverseProxy bool     `mapstructure:"behind_reverse_proxy"` // Whether the server is behind a reverse proxy (default: false)

	// Runtime secrets (JWTSecret is resolved like every other key; see
	// ComputeJWTSecret for the development fallback)
	JWTSecret   []byte    `mapstructure:"-"`
	FrontendURL string    `mapstructure:"frontend_url"`
	SSO         SSOConfig `mapstructure:"-"`

	// S3 configuration
	S3Enabled                  bool   `mapstructure:"s3_enabled"`                      // Enable S3 storage
	S3Endpoint                 string `mapstructure:"s3_endpoint"`                     // S3 endpoint URL
	S3Region                   string `mapstructure:"s3_region"`                       // AWS region
	S3Bucket                   string `mapstructure:"s3_bucket"`                       // S3 bucket name
	S3AccessKey                string `mapstructure:"s3_access_key"`                   // S3 access key ID
	S3SecretKey                string `mapstructure:"s3_secret_key"`                   // S3 secret access key
	S3ForcePath                bool   `mapstructure:"s3_force_path"`                   // Force path-style URLs
	S3CustomDomain             string `mapstructure:"s3_custom_domain"`                // Optional custom domain for S3 URLs
	S3DisableBucketInCustomURL bool   `mapstructure:"s3_disable_bucket_in_custom_url"` // Disable including bucket in custom domain URLs (default: false, meaning include bucket by default)
}

// keyDefaults maps every configuration key to its built-in default. The
// environment variable name for a key is strings.ToUpper(key) (for example
// data_dir → DATA_DIR). Only a few keys are backed by command-line flags;
// those bindings live in internal/cli.
var keyDefaults = map[string]any{
	"addr":      DefaultAddr,
	"port":      DefaultPort,
	"data_dir":  DefaultDataDir,
	"log_level": "info",
	"base_url":  "",

	"captcha_rate_limit_per_minute": 30,
	"auth_rate_limit_per_minute":    10,

	"db_type":     "",
	"db_host":     "",
	"db_port":     0,
	"db_user":     "",
	"db_password": "",
	"db_name":     "",
	"db_ssl_mode": "",

	"oidc_enabled":        false,
	"oidc_issuer_url":     "",
	"oidc_client_id":      "",
	"oidc_client_secret":  "",
	"oidc_auth_url":       "",
	"oidc_token_url":      "",
	"oidc_userinfo_url":   "",
	"oidc_scopes":         "openid profile email",
	"oidc_email_claim":    "email",
	"oidc_name_claim":     "name",
	"oidc_group_claim":    "groups",
	"oidc_allowed_groups": "",
	"oidc_auto_redirect":  false,
	"oidc_verify_email":   false,

	"github_client_id":     "",
	"github_client_secret": "",

	"google_client_id":     "",
	"google_client_secret": "",

	"settings_encryption_key": "",

	"allow_local_login": true,

	"trusted_proxies":      []string{},
	"behind_reverse_proxy": false,

	"s3_enabled":                      false,
	"s3_endpoint":                     "",
	"s3_region":                       "",
	"s3_bucket":                       "",
	"s3_access_key":                   "",
	"s3_secret_key":                   "",
	"s3_force_path":                   false,
	"s3_custom_domain":                "",
	"s3_disable_bucket_in_custom_url": false,

	"jwt_secret":   "",
	"frontend_url": "",
}

// Load resolves the configuration into a *Config using the given viper
// instance. Flag bindings must already be registered on v (see internal/cli).
//
// The precedence is: explicitly passed flags > config file > environment
// variables > built-in defaults. Viper natively orders flags above the config
// file and the config file above defaults; environment variables are layered
// into the defaults for keys the file leaves unset. This yields file-over-env
// semantics: an explicit `false` in the file must be able to override an
// environment `true`.
func Load(v *viper.Viper, configFile string) (*Config, error) {
	for key, def := range keyDefaults {
		v.SetDefault(key, def)
	}

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configFile, cleanPathError(err))
		}
		slog.Info("loaded configuration", "configFile", configFile)
	}

	// Environment variables fill the gaps the config file leaves open. For a
	// key present in the file the fill is skipped, so the file wins over the
	// environment; everything else falls back to the variable, or to the
	// built-in default when the variable is unset.
	for key := range keyDefaults {
		if v.InConfig(key) {
			continue
		}
		if env := os.Getenv(strings.ToUpper(key)); env != "" {
			v.SetDefault(key, env)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode configuration: %w", err)
	}
	// []byte cannot be mapstructure-decoded from a string, so the JWT secret
	// is read explicitly after the unmarshal. A value that is present but not
	// a string (e.g. a nested map from a YAML type mistake) would silently
	// degrade to "" and rotate the secret on every restart, so it is rejected
	// outright. Absent, null, and empty values keep the documented random
	// fallback in ComputeJWTSecret.
	if secret := v.Get("jwt_secret"); secret != nil {
		s, ok := secret.(string)
		if !ok {
			return nil, fmt.Errorf("jwt_secret: expected a string, got %T", secret)
		}
		cfg.JWTSecret = []byte(s)
	}
	// The comma-split decode hook does not trim whitespace around list items;
	// keep "1.2.3.4, 5.6.7.8" usable.
	for i, p := range cfg.TrustedProxies {
		cfg.TrustedProxies[i] = strings.TrimSpace(p)
	}
	cfg.buildSSO()
	return cfg, nil
}

// cleanPathError strips the *fs.PathError wrapper so the rendered message is
// a clean "no such file or directory" / "permission denied" without
// duplicating the filename, which the surrounding error text already carries.
func cleanPathError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// GetListenAddr returns the full listen address in the format "addr:port"
func (c *Config) GetListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}
