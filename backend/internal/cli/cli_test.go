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

// runCmd builds a fresh root command per call (cobra accumulates flag state
// across Execute calls), captures its output, runs the given args, and
// returns stdout, the resolved config, and the execution error.
func runCmd(t *testing.T, args ...string) (*bytes.Buffer, *config.Config, error) {
	t.Helper()
	var out bytes.Buffer
	root, state := newRootCmd("test-version")
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return &out, state.cfg, err
}

func TestLongSpellings(t *testing.T) {
	path := writeConfig(t, "")
	_, cfg, err := runCmd(t, "--config", path, "--addr", "10.0.0.1", "--port", "8080", "--data", "/tmp/data")
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
	_, cfg, err := runCmd(t, "-c", path, "-a", "10.0.0.1", "-p", "9090", "-d", "/custom/data")
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
	_, cfg, err := runCmd(t, "--config="+path, "--addr=10.0.0.1", "--port=8080", "--data=/tmp/data")
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
	_, cfg, err := runCmd(t, "--addr", "ignored", "-a", "10.1.1.1", "--port", "1111", "-p", "2222")
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
	_, cfg, err := runCmd(t)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "0.0.0.0" || cfg.Port != 3001 || cfg.DataDir != "./data" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestVersionLong(t *testing.T) {
	out, cfg, err := runCmd(t, "--version")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("--version should not resolve a config")
	}
	if !strings.Contains(out.String(), "vexgo test-version") {
		t.Fatalf("--version should print the version, got %q", out.String())
	}
}

func TestVersionShort(t *testing.T) {
	out, cfg, err := runCmd(t, "-V")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("-V should not resolve a config")
	}
	if !strings.Contains(out.String(), "vexgo test-version") {
		t.Fatalf("-V should print the version, got %q", out.String())
	}
}

func TestHelpLong(t *testing.T) {
	out, cfg, err := runCmd(t, "--help")
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
		"-V, --version",
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
	out, cfg, err := runCmd(t, "-h")
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
	_, cfg, err := runCmd(t, "--nope")
	if err == nil {
		t.Fatal("unknown flag should be an error")
	}
	if cfg != nil {
		t.Fatal("unknown flag should not resolve a config")
	}
}

func TestMissingConfigFileIsAnError(t *testing.T) {
	_, cfg, err := runCmd(t, "-c", filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("missing config file should be an error")
	}
	if cfg != nil {
		t.Fatal("missing config file should not resolve a config")
	}
}

func TestInvalidConfigFileIsAnError(t *testing.T) {
	path := writeConfig(t, "addr: [unclosed\n")
	_, cfg, err := runCmd(t, "-c", path)
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

	_, cfg, err := runCmd(t, "-c", path, "-a", "flag-addr")
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

	_, cfg, err := runCmd(t, "--data=")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "" {
		t.Fatalf("explicit --data= should override DATA_DIR, got %q", cfg.DataDir)
	}
}

func TestExplicitZeroPortOverridesEnv(t *testing.T) {
	t.Setenv("PORT", "8080")

	_, cfg, err := runCmd(t, "--port", "0")
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

	_, cfg, err := runCmd(t, "-a", "10.0.0.1")
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

func TestDashTerminator(t *testing.T) {
	t.Setenv("PORT", "8080")

	// Everything after -- is positional; --port 9999 must not be parsed,
	// so the env value survives.
	_, cfg, err := runCmd(t, "--", "--port", "9999")
	if err != nil {
		t.Fatalf("-- terminator should not be an error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("args after -- must not be parsed as flags; env PORT should survive, got %d", cfg.Port)
	}
}

func TestInterspersedPositional(t *testing.T) {
	_, cfg, err := runCmd(t, "serve", "--port", "7000")
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
