package public

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A themes directory that cannot be created would otherwise surface much later
// as a baffling "file not found" during a theme install or preview, so the
// failure has to be reported where it happens.
func TestNewRenderer_ReportsUncreatableThemesDir(t *testing.T) {
	// A regular file where the data directory belongs makes MkdirAll fail.
	blocker := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	// The failure must not stop the renderer from being constructed.
	renderer := NewRenderer(nil, "http://localhost", blocker)
	if renderer == nil {
		t.Fatal("expected a renderer even when the themes directory cannot be created")
	}
	if !strings.Contains(logs.String(), "themes directory") {
		t.Errorf("expected the failed themes directory creation to be logged, got %q", logs.String())
	}
}
