package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/config"
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

// runServerCmd builds a fresh root command per call (cobra accumulates flag state
// across Execute calls), captures its output, runs the given args, and
// returns stdout, and the execution error.
func runRootCmd(t *testing.T, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmd("test-version")
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return &out, err
}

// runServerCmd builds a fresh `vexgo server` command per call (cobra accumulates flag state
// across Execute calls), captures its output, runs the given args, and
// returns stdout, the resolved config, and the execution error.
func runServerCmd(t *testing.T, args ...string) (*bytes.Buffer, *config.Config, error) {
	t.Helper()
	var out bytes.Buffer
	root, state := newServerCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return &out, state.cfg, err
}

// runDevCmd builds a fresh `vexgo dev` command per call, captures its output,
// runs the given args, and returns stdout, the resolved config, and the
// execution error.
func runDevCmd(t *testing.T, args ...string) (*bytes.Buffer, *config.Config, error) {
	t.Helper()
	var out bytes.Buffer
	root, state := newDevCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return &out, state.cfg, err
}

func TestLongSpellings(t *testing.T) {
	path := writeConfig(t, "")
	_, cfg, err := runServerCmd(t, "--config", path, "--addr", "10.0.0.1", "--port", "8080", "--data", "/tmp/data")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
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

func TestShortSpellings(t *testing.T) {
	path := writeConfig(t, "")
	_, cfg, err := runServerCmd(t, "-c", path, "-a", "10.0.0.1", "-p", "9090", "-d", "/custom/data")
	if err != nil {
		t.Fatal(err)
	}
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

func TestEqualsSyntax(t *testing.T) {
	path := writeConfig(t, "")
	_, cfg, err := runServerCmd(t, "--config="+path, "--addr=10.0.0.1", "--port=8080", "--data=/tmp/data")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("--flag=value syntax should parse successfully")
	}
	if cfg.Addr != "10.0.0.1" || cfg.Port != 8080 || cfg.DataDir != "/tmp/data" {
		t.Fatalf("equals syntax should apply all values: %+v", cfg)
	}
}

func TestAliasPairsLastWins(t *testing.T) {
	// Long spelling followed by short spelling: the last one wins.
	_, cfg, err := runServerCmd(t, "--addr", "ignored", "-a", "10.1.1.1", "--port", "1111", "-p", "2222")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "10.1.1.1" {
		t.Fatalf("addr should be 10.1.1.1 (last wins), got %s", cfg.Addr)
	}
	if cfg.Port != 2222 {
		t.Fatalf("port should be 2222 (last wins), got %d", cfg.Port)
	}
}

func TestNoArgsDefaults(t *testing.T) {
	_, cfg, err := runServerCmd(t)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "0.0.0.0" || cfg.Port != 3001 || cfg.DataDir != "./data" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestVersionLong(t *testing.T) {
	out, err := runRootCmd(t, "--version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "vexgo test-version") {
		t.Fatalf("--version should print the version, got %q", out.String())
	}
}

func TestVersionShort(t *testing.T) {
	out, err := runRootCmd(t, "-V")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "vexgo test-version") {
		t.Fatalf("-V should print the version, got %q", out.String())
	}
}

func TestServerHelpLong(t *testing.T) {
	out, cfg, err := runServerCmd(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("--help should not resolve a config")
	}
	for _, want := range []string{
		"-c, --config",
		"-a, --addr",
		"-p, --port",
		"-d, --data",
		`(default "0.0.0.0")`,
		"(default 3001)",
		`(default "./data")`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help output should contain %q, got:\n%s", want, out.String())
		}
	}
}

func TestHelpShort(t *testing.T) {
	out, cfg, err := runServerCmd(t, "-h")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("-h should not resolve a config")
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("-h should print usage, got %q", out.String())
	}
}

func TestUnknownFlagIsAnError(t *testing.T) {
	_, cfg, err := runServerCmd(t, "--nope")
	if err == nil {
		t.Fatal("unknown flag should be an error")
	}
	if cfg != nil {
		t.Fatal("unknown flag should not resolve a config")
	}
}

func TestMissingConfigFileIsAnError(t *testing.T) {
	_, cfg, err := runServerCmd(t, "-c", filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("missing config file should be an error")
	}
	if cfg != nil {
		t.Fatal("missing config file should not resolve a config")
	}
}

func TestInvalidConfigFileIsAnError(t *testing.T) {
	path := writeConfig(t, "addr: [unclosed\n")
	_, cfg, err := runServerCmd(t, "-c", path)
	if err == nil {
		t.Fatal("invalid config file should be an error")
	}
	if cfg != nil {
		t.Fatal("invalid config file should not resolve a config")
	}
}

func TestFlagOverridesConfigFileAndEnv(t *testing.T) {
	t.Setenv("ADDR", "env-addr")
	path := writeConfig(t, "addr: file-addr\nport: 5000\n")

	_, cfg, err := runServerCmd(t, "-c", path, "-a", "flag-addr")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "flag-addr" {
		t.Fatalf("flag should override config file and env, got %s", cfg.Addr)
	}
	if cfg.Port != 5000 {
		t.Fatalf("config file port should be applied, got %d", cfg.Port)
	}
}

// A flag explicitly passed as its zero value must override lower-priority
// sources — this is what viper's Changed-aware flag layer buys.
func TestExplicitEmptyDataOverridesEnv(t *testing.T) {
	t.Setenv("DATA_DIR", "/env/data")

	_, cfg, err := runServerCmd(t, "--data=")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "" {
		t.Fatalf("explicit --data= should override DATA_DIR, got %q", cfg.DataDir)
	}
}

func TestExplicitZeroPortOverridesEnv(t *testing.T) {
	t.Setenv("PORT", "8080")

	_, cfg, err := runServerCmd(t, "--port", "0")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 0 {
		t.Fatalf("explicit --port 0 should override PORT, got %d", cfg.Port)
	}
}

func TestAbsentFlagDoesNotOverrideEnv(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("DATA_DIR", "/env/data")

	_, cfg, err := runServerCmd(t, "-a", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "10.0.0.1" {
		t.Fatalf("explicit flag should apply, got %s", cfg.Addr)
	}
	if cfg.Port != 8080 {
		t.Fatalf("absent flag must not override env, got %d", cfg.Port)
	}
	if cfg.DataDir != "/env/data" {
		t.Fatalf("absent flag must not override env, got %s", cfg.DataDir)
	}
}

func TestInterspersedPositional(t *testing.T) {
	_, cfg, err := runServerCmd(t, "--port", "7000")
	if err != nil {
		t.Fatalf("interspersed positional arguments should be tolerated: %v", err)
	}
	if cfg.Port != 7000 {
		t.Fatalf("flags after a positional argument should still apply, got %d", cfg.Port)
	}
}

// =============================================================================
// loadDotEnv — missing file logs info, other failures log a warning.
// =============================================================================

// captureLogs redirects the default slog logger into a buffer for the
// duration of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLoadDotEnvMissingFileLogsInfo(t *testing.T) {
	t.Chdir(t.TempDir())
	buf := captureLogs(t)

	loadDotEnv()

	out := buf.String()
	if !strings.Contains(out, "INFO") || !strings.Contains(out, "no .env file found") {
		t.Fatalf("missing .env should log info, got %q", out)
	}
}

func TestLoadDotEnvUnreadableLogsWarning(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	// A directory named .env opens fine but fails on read (EISDIR), which
	// exercises the non-not-exist branch regardless of the user the tests
	// run as (a chmod 000 file would be readable under root).
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0o700); err != nil {
		t.Fatal(err)
	}
	buf := captureLogs(t)

	loadDotEnv()

	out := buf.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "failed to load .env") {
		t.Fatalf("unreadable .env should log a warning with the error, got %q", out)
	}
	if !strings.Contains(out, "err=") {
		t.Fatalf("warning should carry the underlying error, got %q", out)
	}
}

func TestLoadDotEnvValidFileSetsVariablesQuietly(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("VEXGO_DOTENV_TEST_VAR=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("VEXGO_DOTENV_TEST_VAR") })
	buf := captureLogs(t)

	loadDotEnv()

	if os.Getenv("VEXGO_DOTENV_TEST_VAR") != "1" {
		t.Fatal(".env variables should be set")
	}
	if buf.String() != "" {
		t.Fatalf("valid .env should not log anything, got %q", buf.String())
	}
}

func TestRootRejectsPositionalArgs(t *testing.T) {
	_, err := runRootCmd(t, "serve")
	if err == nil {
		t.Fatal("root should reject positional arguments")
	}
}

func TestUnknownCommandIsAnError(t *testing.T) {
	_, err := runRootCmd(t, "unknown")
	if err == nil {
		t.Fatal("unknown command should be an error")
	}
}

func TestServerRejectsPositionalArgs(t *testing.T) {
	_, _, err := runServerCmd(t, "serve")
	if err == nil {
		t.Fatal("server should reject positional arguments")
	}
}

func TestServerRejectsMultiplePositionalArgs(t *testing.T) {
	_, _, err := runServerCmd(t, "foo", "bar")
	if err == nil {
		t.Fatal("server should reject positional arguments")
	}
}

func TestServerRejectsArgsAfterDashTerminator(t *testing.T) {
	_, _, err := runServerCmd(t, "--", "--port", "9999")
	if err == nil {
		t.Fatal("arguments after -- should be rejected")
	}
}

func TestRootRejectsServerFlags(t *testing.T) {
	_, err := runRootCmd(t, "--port", "8080")
	if err == nil {
		t.Fatal("root should reject server flags")
	}
}

func TestServerUnknownFlagIsAnError(t *testing.T) {
	_, cfg, err := runServerCmd(t, "--nope")

	if err == nil {
		t.Fatal("unknown flag should be an error")
	}

	if cfg != nil {
		t.Fatal("config should not be resolved when flag parsing fails")
	}
}

func TestPortFlagMissingValueIsAnError(t *testing.T) {
	_, _, err := runServerCmd(t, "--port")
	if err == nil {
		t.Fatal("--port without a value should be an error")
	}
}

func TestConfigFlagMissingValueIsAnError(t *testing.T) {
	_, _, err := runServerCmd(t, "--config")
	if err == nil {
		t.Fatal("--config without a value should be an error")
	}
}

func TestInvalidPortIsAnError(t *testing.T) {
	_, _, err := runServerCmd(t, "--port", "not-a-number")
	if err == nil {
		t.Fatal("non-numeric port should be an error")
	}
}

func TestVersionRejectsPositionalArgs(t *testing.T) {
	_, err := runRootCmd(t, "--version", "foo")
	if err == nil {
		t.Fatal("version command should reject positional arguments")
	}
}

func TestServerDontUseConfigFileThemeDir(t *testing.T) {
	path := writeConfig(t, "theme_dir: from-config-file\n")
	_, cfg, err := runServerCmd(t, "-c", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
	if cfg.ThemeDir == "from-config-file" {
		t.Fatal("theme_dir in config file should not be used")
	}
}

// =============================================================================
// vexgo dev — the dev-only --theme-dir flag.
// =============================================================================

func TestDevCommandExists(t *testing.T) {
	out, _, err := runDevCmd(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("dev --help should print usage, got %q", out.String())
	}
	for _, want := range []string{"dev", "--theme-dir"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dev help should contain %q, got:\n%s", want, out.String())
		}
	}
}

func TestDevCommandRejectsPositionalArgs(t *testing.T) {
	_, _, err := runDevCmd(t, "serve")
	if err == nil {
		t.Fatal("dev should reject positional arguments")
	}
}

func TestDevThemeDirOverridesConfigFile(t *testing.T) {
	path := writeConfig(t, "theme_dir: from-config-file\n")
	_, cfg, err := runDevCmd(t, "-c", path, "--theme-dir", "../vexgo-default-theme/")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
	if cfg.ThemeDir != "../vexgo-default-theme/" {
		t.Fatalf("--theme-dir should override the config file, got %q", cfg.ThemeDir)
	}
}

func TestDevDontUseConfigFileThemeDir(t *testing.T) {
	path := writeConfig(t, "theme_dir: from-config-file\n")
	_, cfg, err := runDevCmd(t, "-c", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
	if cfg.ThemeDir == "from-config-file" {
		t.Fatal("theme_dir in config file should not be used")
	}
}

func TestDevThemeDirEmptyIsNoop(t *testing.T) {
	_, cfg, err := runDevCmd(t, "--theme-dir", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
	if cfg.ThemeDir != "" {
		t.Fatalf("explicit empty --theme-dir should leave ThemeDir empty, got %q", cfg.ThemeDir)
	}
}

func TestDevHelpDoesNotResolveConfig(t *testing.T) {
	_, cfg, err := runDevCmd(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("--help should not resolve a config")
	}
}

func TestServerRejectsThemeDirFlag(t *testing.T) {
	_, cfg, err := runServerCmd(t, "--theme-dir", "/tmp/theme")
	if err == nil {
		t.Fatal("server should reject --theme-dir")
	}
	if cfg != nil {
		t.Fatal("--theme-dir on server should not resolve a config")
	}
}

// =============================================================================
// Execute dispatch — server vs dev runState selection.
// =============================================================================

func TestExecuteDevResolvesThemeDir(t *testing.T) {
	cfg, err := Execute("test-version", []string{"dev", "--theme-dir", "/tmp/dev-theme"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
	if cfg.ThemeDir != "/tmp/dev-theme" {
		t.Fatalf("--theme-dir should flow through Execute, got %q", cfg.ThemeDir)
	}
}

func TestExecuteServerLeavesThemeDirEmpty(t *testing.T) {
	cfg, err := Execute("test-version", []string{"server", "--port", "7000"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected a resolved config")
	}
	if cfg.ThemeDir != "" {
		t.Fatalf("server should leave ThemeDir empty, got %q", cfg.ThemeDir)
	}
	if cfg.Port != 7000 {
		t.Fatalf("server flags should apply through Execute, got %d", cfg.Port)
	}
}

func TestExecuteServerRejectsThemeDirFlag(t *testing.T) {
	cfg, err := Execute("test-version", []string{"server", "--theme-dir", "/tmp/theme"})
	if err == nil {
		t.Fatal("server should reject --theme-dir through Execute")
	}
	if cfg != nil {
		t.Fatal("rejected flags should not resolve a config")
	}
}

func TestExecuteHelpReturnsNilConfig(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"server", "--help"},
		{"dev", "--help"},
	} {
		cfg, err := Execute("test-version", args)
		if err != nil {
			t.Fatalf("Execute(%v) should not error: %v", args, err)
		}
		if cfg != nil {
			t.Fatalf("Execute(%v) --help should return nil config", args)
		}
	}
}
