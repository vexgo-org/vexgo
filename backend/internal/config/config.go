// Package config parses server configuration from command-line flags, a YAML
// config file, environment variables, and built-in defaults, in that order of
// priority. It also computes runtime secrets (JWT, SSO) from these sources.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
)

// Action describes what the program should do after parsing flags.
type Action int

const (
	ActionRun     Action = iota // Start the server normally.
	ActionHelp                  // Print usage and exit 0.
	ActionVersion               // Print version and exit 0.
)

// Default values used in PrintUsage and as fallback in newConfigFromEnv.
const (
	defaultAddr    = "0.0.0.0"
	defaultPort    = 3001
	defaultDataDir = "./data"
)

// Config holds the server configuration from command line arguments and/or config file
type Config struct {
	Addr     string // Address to listen on (e.g., "0.0.0.0" or "127.0.0.1")
	Port     int    // Port to listen on
	DataDir  string // Data directory for storing sqlite database and media files
	LogLevel string `yaml:"log_level"` // Logging level: "debug", "info", "warn", "error"

	// BaseURL is the public origin of this instance (e.g., https://vexgo.example.com).
	// Used to build absolute links: SSO/OAuth callbacks and emailed
	// verification / password-reset / email-change links. Empty means the
	// request origin is used, which is unsafe behind untrusted clients.
	BaseURL string `yaml:"base_url"`

	// Database configuration
	DBType     string `yaml:"db_type"`     // Database type: "sqlite", "mysql", or "postgres"
	DBHost     string `yaml:"db_host"`     // Database host
	DBPort     int    `yaml:"db_port"`     // Database port
	DBUser     string `yaml:"db_user"`     // Database user
	DBPassword string `yaml:"db_password"` // Database password
	DBName     string `yaml:"db_name"`     // Database name
	DBSSLMode  string `yaml:"db_ssl_mode"` // PostgreSQL SSL mode (for postgres)

	// OIDC configuration
	OIDCEnabled       bool   `yaml:"oidc_enabled"`        // Enable OIDC login
	OIDCIssuerURL     string `yaml:"oidc_issuer_url"`     // Issuer URL for OIDC discovery
	OIDCClientID      string `yaml:"oidc_client_id"`      // OIDC client ID
	OIDCClientSecret  string `yaml:"oidc_client_secret"`  // OIDC client secret
	OIDCAuthURL       string `yaml:"oidc_auth_url"`       // Authorization endpoint (optional, for manual override)
	OIDCTokenURL      string `yaml:"oidc_token_url"`      // Token endpoint (optional, for manual override)
	OIDCUserInfoURL   string `yaml:"oidc_userinfo_url"`   // UserInfo endpoint (optional, for manual override)
	OIDCScopes        string `yaml:"oidc_scopes"`         // Space-separated scopes (default: "openid profile email")
	OIDCEmailClaim    string `yaml:"oidc_email_claim"`    // Claim name for email (default: "email")
	OIDCNameClaim     string `yaml:"oidc_name_claim"`     // Claim name for username (default: "name")
	OIDCGroupClaim    string `yaml:"oidc_group_claim"`    // Claim name for groups (default: "groups")
	OIDCAllowedGroups string `yaml:"oidc_allowed_groups"` // Comma-separated allowed groups (empty = allow all)
	OIDCAutoRedirect  bool   `yaml:"oidc_auto_redirect"`  // Auto-redirect to OIDC provider (skip login page)
	OIDCVerifyEmail   bool   `yaml:"oidc_verify_email"`   // Require email_verified=true in token

	// GitHub OAuth configuration
	GitHubClientID     string `yaml:"github_client_id"`     // GitHub OAuth App Client ID
	GitHubClientSecret string `yaml:"github_client_secret"` // GitHub OAuth App Client Secret

	// Google OAuth configuration
	GoogleClientID     string `yaml:"google_client_id"`     // Google OAuth 2.0 Client ID
	GoogleClientSecret string `yaml:"google_client_secret"` // Google OAuth 2.0 Client Secret

	// Global options
	AllowLocalLogin bool `yaml:"allow_local_login"` // Allow password-based login (default: true)

	// Trusted proxies configuration
	TrustedProxies     []string `yaml:"trusted_proxies"`      // List of trusted proxy IPs/CIDRs (empty = trust none)
	BehindReverseProxy bool     `yaml:"behind_reverse_proxy"` // Whether the server is behind a reverse proxy (default: false)

	// Runtime secrets (populated by ParseFlags)
	JWTSecret   []byte    `yaml:"-"`
	FrontendURL string    `yaml:"frontend_url"`
	SSO         SSOConfig `yaml:"-"`

	// S3 configuration
	S3Enabled                  bool   `yaml:"s3_enabled"`                      // Enable S3 storage
	S3Endpoint                 string `yaml:"s3_endpoint"`                     // S3 endpoint URL
	S3Region                   string `yaml:"s3_region"`                       // AWS region
	S3Bucket                   string `yaml:"s3_bucket"`                       // S3 bucket name
	S3AccessKey                string `yaml:"s3_access_key"`                   // S3 access key ID
	S3SecretKey                string `yaml:"s3_secret_key"`                   // S3 secret access key
	S3ForcePath                bool   `yaml:"s3_force_path"`                   // Force path-style URLs
	S3CustomDomain             string `yaml:"s3_custom_domain"`                // Optional custom domain for S3 URLs
	S3DisableBucketInCustomURL bool   `yaml:"s3_disable_bucket_in_custom_url"` // Disable including bucket in custom domain URLs (default: false, meaning include bucket by default)

	// fileSet records which fields were explicitly set in the config file.
	// This lets bool fields distinguish "explicitly false" from "unset",
	// so an explicit `false` in the file can override an environment `true`.
	fileSet map[string]bool
}

// fileConfig mirrors Config for YAML unmarshalling, using pointer bools so an
// explicit `false` in the file can be told apart from "not present".
type fileConfig struct {
	Addr      string `yaml:"addr"`
	Port      int    `yaml:"port"`
	DataDir   string `yaml:"data_dir"`
	JWTSecret string `yaml:"jwt_secret"`
	LogLevel  string `yaml:"log_level"`
	BaseURL   string `yaml:"base_url"`

	DBType     string `yaml:"db_type"`
	DBHost     string `yaml:"db_host"`
	DBPort     int    `yaml:"db_port"`
	DBUser     string `yaml:"db_user"`
	DBPassword string `yaml:"db_password"`
	DBName     string `yaml:"db_name"`
	DBSSLMode  string `yaml:"db_ssl_mode"`

	OIDCEnabled       *bool  `yaml:"oidc_enabled"`
	OIDCIssuerURL     string `yaml:"oidc_issuer_url"`
	OIDCClientID      string `yaml:"oidc_client_id"`
	OIDCClientSecret  string `yaml:"oidc_client_secret"`
	OIDCAuthURL       string `yaml:"oidc_auth_url"`
	OIDCTokenURL      string `yaml:"oidc_token_url"`
	OIDCUserInfoURL   string `yaml:"oidc_userinfo_url"`
	OIDCScopes        string `yaml:"oidc_scopes"`
	OIDCEmailClaim    string `yaml:"oidc_email_claim"`
	OIDCNameClaim     string `yaml:"oidc_name_claim"`
	OIDCGroupClaim    string `yaml:"oidc_group_claim"`
	OIDCAllowedGroups string `yaml:"oidc_allowed_groups"`
	OIDCAutoRedirect  *bool  `yaml:"oidc_auto_redirect"`
	OIDCVerifyEmail   *bool  `yaml:"oidc_verify_email"`

	GitHubClientID     string `yaml:"github_client_id"`
	GitHubClientSecret string `yaml:"github_client_secret"`

	GoogleClientID     string `yaml:"google_client_id"`
	GoogleClientSecret string `yaml:"google_client_secret"`

	AllowLocalLogin *bool `yaml:"allow_local_login"`

	TrustedProxies     []string `yaml:"trusted_proxies"`
	BehindReverseProxy *bool    `yaml:"behind_reverse_proxy"`

	S3Enabled                  *bool  `yaml:"s3_enabled"`
	S3Endpoint                 string `yaml:"s3_endpoint"`
	S3Region                   string `yaml:"s3_region"`
	S3Bucket                   string `yaml:"s3_bucket"`
	S3AccessKey                string `yaml:"s3_access_key"`
	S3SecretKey                string `yaml:"s3_secret_key"`
	S3ForcePath                *bool  `yaml:"s3_force_path"`
	S3CustomDomain             string `yaml:"s3_custom_domain"`
	S3DisableBucketInCustomURL *bool  `yaml:"s3_disable_bucket_in_custom_url"`
}

// ParseFlags parses command-line arguments and returns the action to take
// and the server configuration. version is the build version string, injected
// via ldflags.
//
// When args are invalid it prints an error and usage to stderr and returns
// ActionRun with a nil *Config — the caller should exit with code 2.
func ParseFlags(version string, args []string) (Action, *Config) {
	fs := flag.NewFlagSet("vexgo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		configFile  string
		addr        string
		port        int
		dataDir     string
		showVersion bool
	)

	// Register both long and short forms for every option, bound to the
	// same variable so either spelling writes the same value.
	fs.StringVar(&configFile, "config", "", "Path to configuration file (YAML format)")
	fs.StringVar(&configFile, "c", "", "Alias for -config")

	fs.StringVar(&addr, "addr", "", "Address to listen on")
	fs.StringVar(&addr, "a", "", "Alias for -addr")

	fs.IntVar(&port, "port", 0, "Port to listen on")
	fs.IntVar(&port, "p", 0, "Alias for -port")

	fs.StringVar(&dataDir, "data", "", "Data directory for storing SQLite database and media files")
	fs.StringVar(&dataDir, "d", "", "Alias for -data")

	fs.BoolVar(&showVersion, "version", false, "Print version and exit")
	fs.BoolVar(&showVersion, "V", false, "Alias for -version")

	// Suppress the automatic usage callback — we handle display ourselves
	// so the help message is printed exactly once.
	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ActionHelp, nil
		}
		fmt.Fprintf(os.Stderr, "vexgo: error: %v\n", err)
		PrintUsage()
		return ActionRun, nil
	}

	if showVersion {
		fmt.Printf("vexgo %s\n", version)
		return ActionVersion, nil
	}

	// .env is only needed when actually building the server configuration;
	// help and version should not read it.
	loadDotEnv()

	cfg, err := buildConfig(addr, port, dataDir, configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vexgo: error: %v\n", err)
		return ActionRun, nil
	}
	return ActionRun, cfg
}

// buildConfig merges the three configuration sources with explicit priority:
// command line flags > config file > environment variables > defaults.
func buildConfig(addr string, port int, dataDir, configFile string) (*Config, error) {
	// 1. Lowest priority: defaults, then environment variables
	cfg := newConfigFromEnv()

	// 2. Config file overrides environment variables (but not command line flags)
	if configFile != "" {
		file := &fileConfig{}
		if err := loadConfigFile(configFile, file); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configFile, err)
		}
		slog.Info("loaded configuration", "configFile", configFile)
		applyFileConfig(cfg, file)
	}

	// 3. Highest priority: command line flags
	if addr != "" {
		cfg.Addr = addr
	}
	if port != 0 {
		cfg.Port = port
	}
	if dataDir != "" {
		cfg.DataDir = dataDir
	}

	return cfg, nil
}

// loadDotEnv loads environment variables from a .env file (best-effort).
// It is called by ParseFlags only on the run path, just before config
// construction, so help and version exit without reading the file.
func loadDotEnv() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Info("no .env file found, will use environment variables from the system")
	}
}

// newConfigFromEnv returns a Config populated from environment variables,
// falling back to defaults when a variable is unset or invalid.
func newConfigFromEnv() *Config {
	return &Config{
		Addr:     envString("ADDR", defaultAddr),
		Port:     envInt("PORT", defaultPort),
		DataDir:  envString("DATA_DIR", defaultDataDir),
		LogLevel: envString("LOG_LEVEL", "info"),
		BaseURL:  envString("BASE_URL", ""),

		DBType:     envString("DB_TYPE", ""),
		DBHost:     envString("DB_HOST", ""),
		DBPort:     envInt("DB_PORT", 0),
		DBUser:     envString("DB_USER", ""),
		DBPassword: envString("DB_PASSWORD", ""),
		DBName:     envString("DB_NAME", ""),
		DBSSLMode:  envString("DB_SSL_MODE", ""),

		OIDCEnabled:       envBool("OIDC_ENABLED", false),
		OIDCIssuerURL:     envString("OIDC_ISSUER_URL", ""),
		OIDCClientID:      envString("OIDC_CLIENT_ID", ""),
		OIDCClientSecret:  envString("OIDC_CLIENT_SECRET", ""),
		OIDCAuthURL:       envString("OIDC_AUTH_URL", ""),
		OIDCTokenURL:      envString("OIDC_TOKEN_URL", ""),
		OIDCUserInfoURL:   envString("OIDC_USERINFO_URL", ""),
		OIDCScopes:        envString("OIDC_SCOPES", ""),
		OIDCEmailClaim:    envString("OIDC_EMAIL_CLAIM", ""),
		OIDCNameClaim:     envString("OIDC_NAME_CLAIM", ""),
		OIDCGroupClaim:    envString("OIDC_GROUP_CLAIM", ""),
		OIDCAllowedGroups: envString("OIDC_ALLOWED_GROUPS", ""),
		OIDCAutoRedirect:  envBool("OIDC_AUTO_REDIRECT", false),
		OIDCVerifyEmail:   envBool("OIDC_VERIFY_EMAIL", false),

		GitHubClientID:     envString("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: envString("GITHUB_CLIENT_SECRET", ""),

		GoogleClientID:     envString("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: envString("GOOGLE_CLIENT_SECRET", ""),

		AllowLocalLogin: envBool("ALLOW_LOCAL_LOGIN", true),

		TrustedProxies:     parseTrustedProxies(envString("TRUSTED_PROXIES", "")),
		BehindReverseProxy: envBool("BEHIND_REVERSE_PROXY", false),

		S3Enabled:                  envBool("S3_ENABLED", false),
		S3Endpoint:                 envString("S3_ENDPOINT", ""),
		S3Region:                   envString("S3_REGION", ""),
		S3Bucket:                   envString("S3_BUCKET", ""),
		S3AccessKey:                envString("S3_ACCESS_KEY", ""),
		S3SecretKey:                envString("S3_SECRET_KEY", ""),
		S3ForcePath:                envBool("S3_FORCE_PATH", false),
		S3CustomDomain:             envString("S3_CUSTOM_DOMAIN", ""),
		S3DisableBucketInCustomURL: envBool("S3_DISABLE_BUCKET_IN_CUSTOM_URL", false),

		fileSet: make(map[string]bool),
	}
}

// envString returns the environment variable value or defaultValue when unset.
func envString(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// envInt returns the parsed integer environment variable or defaultValue.
func envInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}

// envBool returns the parsed boolean environment variable or defaultValue.
func envBool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}

// loadConfigFile parses a YAML configuration file into file.
func loadConfigFile(filename string, file *fileConfig) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		// Strip the *fs.PathError wrapper so buildConfig can render a clean
		// message ("no such file or directory", "permission denied", ...)
		// without duplicating the filename.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return pathErr.Err
		}
		return err
	}
	if err := yaml.Unmarshal(data, file); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}
	return nil
}

// applyFileConfig applies config file values over cfg (which already contains
// environment variables). Only fields explicitly present in the file are
// applied, so an explicit `false` for a bool overrides an environment `true`.
func applyFileConfig(cfg *Config, f *fileConfig) {
	if f.Addr != "" {
		cfg.Addr = f.Addr
	}
	if f.Port != 0 {
		cfg.Port = f.Port
	}
	if f.DataDir != "" {
		cfg.DataDir = f.DataDir
	}
	if f.JWTSecret != "" {
		cfg.JWTSecret = []byte(f.JWTSecret)
	}
	if f.LogLevel != "" {
		cfg.LogLevel = f.LogLevel
	}
	if f.BaseURL != "" {
		cfg.BaseURL = f.BaseURL
	}

	// Database
	if f.DBType != "" {
		cfg.DBType = f.DBType
	}
	if f.DBHost != "" {
		cfg.DBHost = f.DBHost
	}
	if f.DBPort != 0 {
		cfg.DBPort = f.DBPort
	}
	if f.DBUser != "" {
		cfg.DBUser = f.DBUser
	}
	if f.DBPassword != "" {
		cfg.DBPassword = f.DBPassword
	}
	if f.DBName != "" {
		cfg.DBName = f.DBName
	}
	if f.DBSSLMode != "" {
		cfg.DBSSLMode = f.DBSSLMode
	}

	// OIDC
	if f.OIDCEnabled != nil {
		cfg.OIDCEnabled = *f.OIDCEnabled
		cfg.fileSet["oidc_enabled"] = true
	}
	if f.OIDCIssuerURL != "" {
		cfg.OIDCIssuerURL = f.OIDCIssuerURL
	}
	if f.OIDCClientID != "" {
		cfg.OIDCClientID = f.OIDCClientID
	}
	if f.OIDCClientSecret != "" {
		cfg.OIDCClientSecret = f.OIDCClientSecret
	}
	if f.OIDCAuthURL != "" {
		cfg.OIDCAuthURL = f.OIDCAuthURL
	}
	if f.OIDCTokenURL != "" {
		cfg.OIDCTokenURL = f.OIDCTokenURL
	}
	if f.OIDCUserInfoURL != "" {
		cfg.OIDCUserInfoURL = f.OIDCUserInfoURL
	}
	if f.OIDCScopes != "" {
		cfg.OIDCScopes = f.OIDCScopes
	}
	if f.OIDCEmailClaim != "" {
		cfg.OIDCEmailClaim = f.OIDCEmailClaim
	}
	if f.OIDCNameClaim != "" {
		cfg.OIDCNameClaim = f.OIDCNameClaim
	}
	if f.OIDCGroupClaim != "" {
		cfg.OIDCGroupClaim = f.OIDCGroupClaim
	}
	if f.OIDCAllowedGroups != "" {
		cfg.OIDCAllowedGroups = f.OIDCAllowedGroups
	}
	if f.OIDCAutoRedirect != nil {
		cfg.OIDCAutoRedirect = *f.OIDCAutoRedirect
		cfg.fileSet["oidc_auto_redirect"] = true
	}
	if f.OIDCVerifyEmail != nil {
		cfg.OIDCVerifyEmail = *f.OIDCVerifyEmail
		cfg.fileSet["oidc_verify_email"] = true
	}

	// GitHub OAuth
	if f.GitHubClientID != "" {
		cfg.GitHubClientID = f.GitHubClientID
	}
	if f.GitHubClientSecret != "" {
		cfg.GitHubClientSecret = f.GitHubClientSecret
	}

	// Google OAuth
	if f.GoogleClientID != "" {
		cfg.GoogleClientID = f.GoogleClientID
	}
	if f.GoogleClientSecret != "" {
		cfg.GoogleClientSecret = f.GoogleClientSecret
	}

	// Global options
	if f.AllowLocalLogin != nil {
		cfg.AllowLocalLogin = *f.AllowLocalLogin
		cfg.fileSet["allow_local_login"] = true
	}

	// Trusted proxies
	if f.TrustedProxies != nil {
		cfg.TrustedProxies = f.TrustedProxies
	}
	if f.BehindReverseProxy != nil {
		cfg.BehindReverseProxy = *f.BehindReverseProxy
		cfg.fileSet["behind_reverse_proxy"] = true
	}

	// S3
	if f.S3Enabled != nil {
		cfg.S3Enabled = *f.S3Enabled
		cfg.fileSet["s3_enabled"] = true
	}
	if f.S3Endpoint != "" {
		cfg.S3Endpoint = f.S3Endpoint
	}
	if f.S3Region != "" {
		cfg.S3Region = f.S3Region
	}
	if f.S3Bucket != "" {
		cfg.S3Bucket = f.S3Bucket
	}
	if f.S3AccessKey != "" {
		cfg.S3AccessKey = f.S3AccessKey
	}
	if f.S3SecretKey != "" {
		cfg.S3SecretKey = f.S3SecretKey
	}
	if f.S3ForcePath != nil {
		cfg.S3ForcePath = *f.S3ForcePath
		cfg.fileSet["s3_force_path"] = true
	}
	if f.S3CustomDomain != "" {
		cfg.S3CustomDomain = f.S3CustomDomain
	}
	if f.S3DisableBucketInCustomURL != nil {
		cfg.S3DisableBucketInCustomURL = *f.S3DisableBucketInCustomURL
		cfg.fileSet["s3_disable_bucket_in_custom_url"] = true
	}
}

// GetListenAddr returns the full listen address in the format "addr:port"
func (c *Config) GetListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}

// PrintUsage prints the command-line usage to stdout, showing both long
// and short spellings with their default values.
func PrintUsage() {
	fmt.Printf("Usage: vexgo [options]\n\nOptions:\n")
	fmt.Printf("  --config, -c <file>    Path to configuration file (YAML format)\n")
	fmt.Printf("  --addr, -a <addr>      Address to listen on (default: %s)\n", defaultAddr)
	fmt.Printf("  --port, -p <port>      Port to listen on (default: %d)\n", defaultPort)
	fmt.Printf("  --data, -d <dir>       Data directory (default: %s)\n", defaultDataDir)
	fmt.Printf("  --version, -V          Print version and exit\n")
	fmt.Printf("  --help, -h             Print this help and exit\n")
}

// parseTrustedProxies parses a comma-separated list of trusted proxies
func parseTrustedProxies(s string) []string {
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
