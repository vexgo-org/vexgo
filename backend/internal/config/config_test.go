package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// writeConfig writes a YAML config file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// loadOrFail resolves a configuration with a fresh viper instance, writing
// the given YAML to a temporary config file when non-empty. Empty content
// means no config file at all.
func loadOrFail(t *testing.T, fileContent string) *Config {
	t.Helper()
	path := ""
	if fileContent != "" {
		path = writeConfig(t, fileContent)
	}
	cfg, err := Load(viper.New(), path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestPriorityDefaultWhenNothingSet(t *testing.T) {
	cfg := loadOrFail(t, "")
	if cfg.Addr != "0.0.0.0" || cfg.Port != 3001 || cfg.DataDir != "./data" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if !cfg.AllowLocalLogin {
		t.Fatalf("AllowLocalLogin should default to true, got %v", cfg.AllowLocalLogin)
	}
	if cfg.S3Enabled {
		t.Fatal("S3Enabled should default to false")
	}
}

func TestPriorityEnvOverridesDefault(t *testing.T) {
	t.Setenv("ADDR", "10.0.0.1")
	t.Setenv("PORT", "8080")
	t.Setenv("ALLOW_LOCAL_LOGIN", "false")
	t.Setenv("S3_ENABLED", "true")

	cfg := loadOrFail(t, "")
	if cfg.Addr != "10.0.0.1" || cfg.Port != 8080 {
		t.Fatalf("env should override defaults: %+v", cfg)
	}
	if cfg.AllowLocalLogin {
		t.Fatal("ALLOW_LOCAL_LOGIN=false should disable local login")
	}
	if !cfg.S3Enabled {
		t.Fatal("S3_ENABLED=true should enable S3")
	}
}

func TestPriorityConfigFileOverridesEnv(t *testing.T) {
	t.Setenv("ADDR", "10.0.0.1")
	t.Setenv("DB_HOST", "env-host")
	t.Setenv("S3_ENABLED", "true")

	cfg := loadOrFail(t, "addr: 192.168.1.1\ndb_host: file-host\ns3_enabled: false\n")

	if cfg.Addr != "192.168.1.1" {
		t.Fatalf("config file should override env for addr, got %s", cfg.Addr)
	}
	if cfg.DBHost != "file-host" {
		t.Fatalf("config file should override env for db_host, got %s", cfg.DBHost)
	}
}

// The original bug: env S3_ENABLED=true made the config file's
// s3_enabled: false impossible to apply.
func TestPriorityConfigFileBoolFalseOverridesEnvTrue(t *testing.T) {
	t.Setenv("S3_ENABLED", "true")
	t.Setenv("ALLOW_LOCAL_LOGIN", "true")

	cfg := loadOrFail(t, "s3_enabled: false\nallow_local_login: false\n")

	if cfg.S3Enabled {
		t.Fatal("config file s3_enabled: false should override env S3_ENABLED=true")
	}
	if cfg.AllowLocalLogin {
		t.Fatal("config file allow_local_login: false should override env ALLOW_LOCAL_LOGIN=true")
	}
}

func TestPriorityConfigFileBoolTrueOverridesEnvFalse(t *testing.T) {
	t.Setenv("S3_ENABLED", "false")

	cfg := loadOrFail(t, "s3_enabled: true\n")

	if !cfg.S3Enabled {
		t.Fatal("config file s3_enabled: true should override env S3_ENABLED=false")
	}
}

func TestBaseURLTopLevelConfig(t *testing.T) {
	t.Setenv("BASE_URL", "https://env.example.com")

	cfg := loadOrFail(t, "")
	if cfg.BaseURL != "https://env.example.com" {
		t.Fatalf("BASE_URL env should populate top-level BaseURL, got %q", cfg.BaseURL)
	}
	cfg = loadOrFail(t, "base_url: https://file.example.com\n")
	if cfg.BaseURL != "https://file.example.com" {
		t.Fatalf("config file base_url should override env, got %q", cfg.BaseURL)
	}
}

// =============================================================================
// SSO derivation — buildSSO over the resolved flat fields.
// =============================================================================

func TestSSODefaultsApplyWithoutAnySource(t *testing.T) {
	cfg := loadOrFail(t, "")

	if !cfg.SSO.AllowLocalLogin {
		t.Fatal("SSO.AllowLocalLogin should default to true")
	}
	if cfg.SSO.OIDC.Enabled {
		t.Fatal("OIDC should be disabled by default")
	}
	if cfg.SSO.OIDC.EmailClaim != "email" {
		t.Fatalf("email claim default, got %q", cfg.SSO.OIDC.EmailClaim)
	}
	if cfg.SSO.OIDC.NameClaim != "name" {
		t.Fatalf("name claim default, got %q", cfg.SSO.OIDC.NameClaim)
	}
	if cfg.SSO.OIDC.GroupClaim != "groups" {
		t.Fatalf("group claim default, got %q", cfg.SSO.OIDC.GroupClaim)
	}
	want := []string{"openid", "profile", "email"}
	if !reflect.DeepEqual(cfg.SSO.OIDC.Scopes, want) {
		t.Fatalf("scopes default: want %v, got %v", want, cfg.SSO.OIDC.Scopes)
	}
}

func TestLoadFromConfigBoolFalse(t *testing.T) {
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_AUTO_REDIRECT", "true")

	cfg := loadOrFail(t, "oidc_enabled: false\noidc_auto_redirect: false\n")

	if cfg.SSO.OIDC.Enabled {
		t.Fatal("config file oidc_enabled: false should disable OIDC in SSO")
	}
	if cfg.SSO.OIDC.AutoRedirect {
		t.Fatal("config file oidc_auto_redirect: false should disable auto redirect in SSO")
	}
}

func TestLoadFromConfigStringOverrides(t *testing.T) {
	t.Setenv("GITHUB_CLIENT_ID", "env-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "env-secret")

	cfg := loadOrFail(t, "github_client_id: file-id\n")

	if cfg.SSO.GitHub.ClientID != "file-id" {
		t.Fatalf("config file should override env for github client id, got %s", cfg.SSO.GitHub.ClientID)
	}
	// Not set in the file: the env value survives.
	if cfg.SSO.GitHub.ClientSecret != "env-secret" {
		t.Fatalf("env value should be preserved when file does not set it, got %s", cfg.SSO.GitHub.ClientSecret)
	}
}

func TestEnvWithoutConfigFileStillPopulatesSSO(t *testing.T) {
	t.Setenv("GITHUB_CLIENT_ID", "env-id")
	cfg := loadOrFail(t, "")

	if cfg.SSO.GitHub.ClientID != "env-id" {
		t.Fatalf("env-only config should populate SSO, got %s", cfg.SSO.GitHub.ClientID)
	}
}

func TestOIDCAllowedGroupsFromEnvAndFile(t *testing.T) {
	t.Setenv("OIDC_ALLOWED_GROUPS", "admins, devs")

	cfg := loadOrFail(t, "")
	want := []string{"admins", "devs"}
	if !reflect.DeepEqual(cfg.SSO.OIDC.AllowedGroups, want) {
		t.Fatalf("env allowed groups: want %v, got %v", want, cfg.SSO.OIDC.AllowedGroups)
	}

	cfg = loadOrFail(t, "oidc_allowed_groups: admins,ops\n")
	want = []string{"admins", "ops"}
	if !reflect.DeepEqual(cfg.SSO.OIDC.AllowedGroups, want) {
		t.Fatalf("file allowed groups: want %v, got %v", want, cfg.SSO.OIDC.AllowedGroups)
	}
}

// =============================================================================
// Trusted proxies.
// =============================================================================

func TestTrustedProxiesFromEnvAreTrimmed(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "1.1.1.1, 2.2.2.2 ,3.3.3.3")

	cfg := loadOrFail(t, "")
	want := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	if !reflect.DeepEqual(cfg.TrustedProxies, want) {
		t.Fatalf("trusted proxies: want %v, got %v", want, cfg.TrustedProxies)
	}
}

func TestTrustedProxiesFromFile(t *testing.T) {
	cfg := loadOrFail(t, "trusted_proxies: [\"192.168.1.100\", \"10.0.0.0/8\"]\n")
	want := []string{"192.168.1.100", "10.0.0.0/8"}
	if !reflect.DeepEqual(cfg.TrustedProxies, want) {
		t.Fatalf("trusted proxies: want %v, got %v", want, cfg.TrustedProxies)
	}
}

// =============================================================================
// JWT secret resolution.
// =============================================================================

func TestJWTSecretFileOverridesEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "env-secret")

	cfg := loadOrFail(t, "jwt_secret: file-secret\n")
	if string(cfg.JWTSecret) != "file-secret" {
		t.Fatalf("config file jwt_secret should override env, got %q", string(cfg.JWTSecret))
	}
}

func TestJWTSecretFromEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "env-secret")

	cfg := loadOrFail(t, "")
	if string(cfg.JWTSecret) != "env-secret" {
		t.Fatalf("JWT_SECRET env should populate JWTSecret, got %q", string(cfg.JWTSecret))
	}
}

// A jwt_secret value that is present but not a string (a YAML type mistake)
// must fail the load instead of silently degrading to "", which would rotate
// the secret on every restart.
func TestJWTSecretNonStringIsAnError(t *testing.T) {
	for name, content := range map[string]string{
		"nested map": "jwt_secret:\n  nested: map\n",
		"number":     "jwt_secret: 12345\n",
		"list":       "jwt_secret: [abc]\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(viper.New(), writeConfig(t, content))
			if err == nil {
				t.Fatal("non-string jwt_secret should be an error")
			}
			if !strings.Contains(err.Error(), "jwt_secret") {
				t.Fatalf("error should name the offending key, got %v", err)
			}
		})
	}
}

func TestJWTSecretAbsentAndEmptyKeepFallback(t *testing.T) {
	for name, content := range map[string]string{
		"key absent":     "",
		"null value":     "jwt_secret:\n",
		"explicit empty": "jwt_secret: \"\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := Load(viper.New(), writeConfig(t, content))
			if err != nil {
				t.Fatalf("jwt_secret %q should not error, got %v", content, err)
			}
			if err := cfg.ComputeJWTSecret(); err != nil {
				t.Fatal(err)
			}
			if len(cfg.JWTSecret) == 0 {
				t.Fatal("fallback should generate a non-empty secret")
			}
		})
	}
}

func TestComputeJWTSecretGeneratesFallback(t *testing.T) {
	cfg := &Config{}
	if err := cfg.ComputeJWTSecret(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.JWTSecret) == 0 {
		t.Fatal("fallback should generate a non-empty secret")
	}
	if cfg.FrontendURL != "http://localhost:5173" {
		t.Fatalf("FrontendURL should fall back to the dev server origin, got %q", cfg.FrontendURL)
	}
}

func TestComputeJWTSecretKeepsExistingValues(t *testing.T) {
	cfg := &Config{JWTSecret: []byte("already-set"), FrontendURL: "https://vexgo.example.com"}
	if err := cfg.ComputeJWTSecret(); err != nil {
		t.Fatal(err)
	}
	if string(cfg.JWTSecret) != "already-set" {
		t.Fatalf("existing secret should be preserved, got %q", string(cfg.JWTSecret))
	}
	if cfg.FrontendURL != "https://vexgo.example.com" {
		t.Fatalf("existing FrontendURL should be preserved, got %q", cfg.FrontendURL)
	}
}

// =============================================================================
// Error paths.
// =============================================================================

func TestMissingConfigFileIsAnError(t *testing.T) {
	_, err := Load(viper.New(), filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("missing config file should be an error")
	}
	if !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("error should carry the clean path error, got %v", err)
	}
}

func TestInvalidConfigFileIsAnError(t *testing.T) {
	_, err := Load(viper.New(), writeConfig(t, "addr: [unclosed\n"))
	if err == nil {
		t.Fatal("invalid config file should be an error")
	}
}

func TestGetListenAddr(t *testing.T) {
	cfg := &Config{Addr: "127.0.0.1", Port: 8080}
	if got := cfg.GetListenAddr(); got != "127.0.0.1:8080" {
		t.Fatalf("GetListenAddr: got %q", got)
	}
}
