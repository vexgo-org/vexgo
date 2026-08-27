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

// buildConfigOrFail calls buildConfig and fails the test on error.
func buildConfigOrFail(t *testing.T, addr string, port int, dataDir, configFile string) *Config {
	t.Helper()
	cfg, err := buildConfig(addr, port, dataDir, configFile)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestPriorityDefaultWhenNothingSet(t *testing.T) {
	cfg := buildConfigOrFail(t, "", 0, "", "")
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

	cfg := buildConfigOrFail(t, "", 0, "", "")
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
	cfg := buildConfigOrFail(t, "", 0, "", path)

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
	cfg := buildConfigOrFail(t, "", 0, "", path)

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
	cfg := buildConfigOrFail(t, "", 0, "", path)

	if !cfg.S3Enabled {
		t.Fatal("config file s3_enabled: true should override env S3_ENABLED=false")
	}
}

func TestPriorityFlagsOverrideConfigFileAndEnv(t *testing.T) {
	t.Setenv("ADDR", "10.0.0.1")
	path := writeConfig(t, "addr: 192.168.1.1\n")

	cfg := buildConfigOrFail(t, "172.16.0.1", 9090, "/custom/data", path)
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
	cfg := buildConfigOrFail(t, "", 0, "", path)
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
	cfg := buildConfigOrFail(t, "", 0, "", path)
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
	cfg := buildConfigOrFail(t, "", 0, "", "")
	cfg.LoadSSOFromEnv()
	cfg.LoadSSOFromConfig()

	if cfg.SSO.GitHub.ClientID != "env-id" {
		t.Fatalf("env-only config should populate SSO, got %s", cfg.SSO.GitHub.ClientID)
	}
}

// =============================================================================
// ParseFlags tests — cover alias pairs, version, help, unknown flags, defaults.
// =============================================================================

func TestParseFlagsLongSpellings(t *testing.T) {
	path := writeConfig(t, "")
	_, cfg := ParseFlags("dev", []string{
		"--config", path,
		"--addr", "10.0.0.1",
		"--port", "8080",
		"--data", "/tmp/data",
	})

	if cfg.Addr != "10.0.0.1" {
		t.Fatalf("--addr: expected 10.0.0.1, got %s", cfg.Addr)
	}
	if cfg.Port != 8080 {
		t.Fatalf("--port: expected 8080, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/data" {
		t.Fatalf("--data: expected /tmp/data, got %s", cfg.DataDir)
	}
}

func TestParseFlagsShortSpellings(t *testing.T) {
	path := writeConfig(t, "")
	_, cfg := ParseFlags("dev", []string{
		"-c", path,
		"-a", "10.0.0.1",
		"-p", "9090",
		"-d", "/custom/data",
	})

	if cfg.Addr != "10.0.0.1" {
		t.Fatalf("-a: expected 10.0.0.1, got %s", cfg.Addr)
	}
	if cfg.Port != 9090 {
		t.Fatalf("-p: expected 9090, got %d", cfg.Port)
	}
	if cfg.DataDir != "/custom/data" {
		t.Fatalf("-d: expected /custom/data, got %s", cfg.DataDir)
	}
}

func TestParseFlagsAliasPairsWriteSameConfig(t *testing.T) {
	// Long spelling followed by short spelling: the last one wins.
	_, cfg := ParseFlags("dev", []string{
		"--addr", "ignored",
		"-a", "10.1.1.1",
		"--port", "1111",
		"-p", "2222",
	})

	if cfg.Addr != "10.1.1.1" {
		t.Fatalf("addr should be 10.1.1.1 (last wins), got %s", cfg.Addr)
	}
	if cfg.Port != 2222 {
		t.Fatalf("port should be 2222 (last wins), got %d", cfg.Port)
	}
}

func TestParseFlagsNoArgsDefaults(t *testing.T) {
	_, cfg := ParseFlags("dev", nil)

	if cfg.Addr != defaultAddr {
		t.Fatalf("default addr, got %s", cfg.Addr)
	}
	if cfg.Port != defaultPort {
		t.Fatalf("default port, got %d", cfg.Port)
	}
	if cfg.DataDir != defaultDataDir {
		t.Fatalf("default data dir, got %s", cfg.DataDir)
	}
}

func TestParseFlagsVersion(t *testing.T) {
	action, _ := ParseFlags("1.2.3", []string{"--version"})
	if action != ActionVersion {
		t.Fatalf("--version should return ActionVersion, got %v", action)
	}
}

func TestParseFlagsVersionShort(t *testing.T) {
	action, _ := ParseFlags("1.2.3", []string{"-V"})
	if action != ActionVersion {
		t.Fatalf("-V should return ActionVersion, got %v", action)
	}
}

func TestParseFlagsVersionDefaultIsDev(t *testing.T) {
	// The default version injected by main.go is "dev".
	// Verify that passing "dev" as the version argument works with both
	// --version and -V.
	t.Run("long", func(t *testing.T) {
		action, _ := ParseFlags("dev", []string{"--version"})
		if action != ActionVersion {
			t.Errorf("expected ActionVersion, got %v", action)
		}
	})
	t.Run("short", func(t *testing.T) {
		action, _ := ParseFlags("dev", []string{"-V"})
		if action != ActionVersion {
			t.Errorf("expected ActionVersion, got %v", action)
		}
	})
}

func TestParseFlagsHelp(t *testing.T) {
	action, _ := ParseFlags("dev", []string{"--help"})
	if action != ActionHelp {
		t.Fatalf("--help should return ActionHelp, got %v", action)
	}
}

func TestParseFlagsHelpShort(t *testing.T) {
	action, _ := ParseFlags("dev", []string{"-h"})
	if action != ActionHelp {
		t.Fatalf("-h should return ActionHelp, got %v", action)
	}
}

func TestParseFlagsUnknownFlag(t *testing.T) {
	action, cfg := ParseFlags("dev", []string{"--nope"})
	if action != ActionRun {
		t.Fatalf("unknown flag should return ActionRun, got %v", action)
	}
	if cfg != nil {
		t.Fatal("unknown flag should return nil config to signal error")
	}
}

func TestParseFlagsMissingConfigFile(t *testing.T) {
	action, cfg := ParseFlags("dev", []string{
		"-c", filepath.Join(t.TempDir(), "nonexistent.yaml"),
	})
	if action != ActionRun {
		t.Fatalf("missing config file should return ActionRun, got %v", action)
	}
	if cfg != nil {
		t.Fatal("missing config file should return nil config to signal error")
	}
}

func TestParseFlagsInvalidConfigFile(t *testing.T) {
	path := writeConfig(t, "addr: [unclosed\n")
	action, cfg := ParseFlags("dev", []string{"-c", path})
	if action != ActionRun {
		t.Fatalf("invalid config file should return ActionRun, got %v", action)
	}
	if cfg != nil {
		t.Fatal("invalid config file should return nil config to signal error")
	}
}

func TestParseFlagsFlagOverConfigFile(t *testing.T) {
	t.Setenv("ADDR", "env-addr")
	path := writeConfig(t, "addr: file-addr\nport: 5000\n")

	_, cfg := ParseFlags("dev", []string{
		"-c", path,
		"-a", "flag-addr",
	})

	if cfg.Addr != "flag-addr" {
		t.Fatalf("flag should override config file and env, got %s", cfg.Addr)
	}
	if cfg.Port != 5000 {
		t.Fatalf("config file port should be applied, got %d", cfg.Port)
	}
}

func TestBaseURLTopLevelConfig(t *testing.T) {
	t.Setenv("BASE_URL", "https://env.example.com")

	cfg, err := buildConfig("", 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://env.example.com" {
		t.Fatalf("BASE_URL env should populate top-level BaseURL, got %q", cfg.BaseURL)
	}
	path := writeConfig(t, "base_url: https://file.example.com\n")
	cfg, err = buildConfig("", 0, "", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://file.example.com" {
		t.Fatalf("config file base_url should override env, got %q", cfg.BaseURL)
	}
}
