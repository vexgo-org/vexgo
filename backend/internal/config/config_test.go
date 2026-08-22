package config

import (
	"os"
	"path/filepath"
	"testing"
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

func TestPriorityDefaultWhenNothingSet(t *testing.T) {
	cfg := buildConfig("", 0, "", "")
	if cfg.Addr != "0.0.0.0" || cfg.Port != 3001 || cfg.DataDir != "./data" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.AllowLocalLogin != true {
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

	cfg := buildConfig("", 0, "", "")
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

	path := writeConfig(t, "addr: 192.168.1.1\ndb_host: file-host\n")
	cfg := buildConfig("", 0, "", path)

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

	path := writeConfig(t, "s3_enabled: false\nallow_local_login: false\n")
	cfg := buildConfig("", 0, "", path)

	if cfg.S3Enabled {
		t.Fatal("config file s3_enabled: false should override env S3_ENABLED=true")
	}
	if cfg.AllowLocalLogin {
		t.Fatal("config file allow_local_login: false should override env ALLOW_LOCAL_LOGIN=true")
	}
	if !cfg.fileSet["s3_enabled"] || !cfg.fileSet["allow_local_login"] {
		t.Fatal("explicitly set file fields should be recorded in fileSet")
	}
}

func TestPriorityConfigFileBoolTrueOverridesEnvFalse(t *testing.T) {
	t.Setenv("S3_ENABLED", "false")

	path := writeConfig(t, "s3_enabled: true\n")
	cfg := buildConfig("", 0, "", path)

	if !cfg.S3Enabled {
		t.Fatal("config file s3_enabled: true should override env S3_ENABLED=false")
	}
}

func TestPriorityFlagsOverrideConfigFileAndEnv(t *testing.T) {
	t.Setenv("ADDR", "10.0.0.1")
	path := writeConfig(t, "addr: 192.168.1.1\n")

	cfg := buildConfig("172.16.0.1", 9090, "/custom/data", path)
	if cfg.Addr != "172.16.0.1" {
		t.Fatalf("flag should override config file for addr, got %s", cfg.Addr)
	}
	if cfg.Port != 9090 {
		t.Fatalf("flag should override for port, got %d", cfg.Port)
	}
	if cfg.DataDir != "/custom/data" {
		t.Fatalf("flag should override for data dir, got %s", cfg.DataDir)
	}
}

func TestLoadFromConfigBoolFalse(t *testing.T) {
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_AUTO_REDIRECT", "true")

	path := writeConfig(t, "oidc_enabled: false\noidc_auto_redirect: false\n")
	cfg := buildConfig("", 0, "", path)
	cfg.LoadSSOFromEnv()
	cfg.LoadSSOFromConfig()

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

	path := writeConfig(t, "github_client_id: file-id\n")
	cfg := buildConfig("", 0, "", path)
	cfg.LoadSSOFromEnv()
	cfg.LoadSSOFromConfig()

	if cfg.SSO.GitHub.ClientID != "file-id" {
		t.Fatalf("config file should override env for github client id, got %s", cfg.SSO.GitHub.ClientID)
	}
	// Not set in the file: falls back to the env-merged cfg value.
	if cfg.SSO.GitHub.ClientSecret != "env-secret" {
		t.Fatalf("env value should be preserved when file does not set it, got %s", cfg.SSO.GitHub.ClientSecret)
	}
}

func TestEnvWithoutConfigFileStillPopulatesSSO(t *testing.T) {
	t.Setenv("GITHUB_CLIENT_ID", "env-id")
	cfg := buildConfig("", 0, "", "")
	cfg.LoadSSOFromEnv()
	cfg.LoadSSOFromConfig()

	if cfg.SSO.GitHub.ClientID != "env-id" {
		t.Fatalf("env-only config should populate SSO, got %s", cfg.SSO.GitHub.ClientID)
	}
}
