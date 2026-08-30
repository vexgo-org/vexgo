package upload

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestGenerateFilename_SanitizesExtension ensures the client-supplied
// extension survives only as a short alphanumeric suffix; anything else
// (separators, HTML, colons, overlong tails) yields a bare UUID name.
func TestGenerateFilename_SanitizesExtension(t *testing.T) {
	cases := []struct {
		name     string
		original string
		wantExt  string
	}{
		{"plain jpg", "photo.jpg", ".jpg"},
		{"uppercase normalized", "PHOTO.JPG", ".jpg"},
		{"multi-dot takes last", "archive.tar.gz", ".gz"},
		{"no extension", "noext", ""},
		{"dot only", "x.", ""},
		{"html injection", "x.<script>", ""},
		{"windows ADS colon", "x.jpg:b", ""},
		{"backslash traversal", `x.\evil.jpg`, ".jpg"},
		{"overlong extension", "x." + strings.Repeat("a", 50), ""},
		{"dashes rejected", "x.tar-gz", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateFilename(tc.original)

			if tc.wantExt == "" {
				if isUUID(got) {
					return
				}
				t.Errorf("expected bare UUID name, got %q", got)
				return
			}
			if !strings.HasSuffix(got, tc.wantExt) {
				t.Fatalf("expected suffix %q, got %q", tc.wantExt, got)
			}
			base := strings.TrimSuffix(got, tc.wantExt)
			if !isUUID(base) {
				t.Errorf("expected UUID base name, got %q", base)
			}
		})
	}
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// TestLocalStorage_ContainsHostileFilenames ensures os.Root confinement:
// hostile filenames can never create or delete files outside the media
// directory. Upload rejects separator-carrying names outright; the ".." name
// has no separator and is what the os.Root layer refuses.
func TestLocalStorage_ContainsHostileFilenames(t *testing.T) {
	dataDir := t.TempDir()
	storage := NewLocalStorage(dataDir)

	// A decoy outside the media tree must survive every hostile operation.
	if err := os.WriteFile(filepath.Join(dataDir, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("seed decoy: %v", err)
	}

	for _, name := range []string{"../evil.txt", "media/../../evil.txt", "/tmp/evil.txt", ".."} {
		if _, err := storage.Upload(strings.NewReader("evil"), name, ""); err == nil {
			t.Errorf("Upload(%q): expected error, got nil", name)
		}
		// Delete cannot escape either: filepath.Base neutralizes traversal
		// segments (missing files stay "not an error"), the ".." name is
		// rejected outright, and os.Root confines the removal; the decoy must
		// survive all of them.
		for _, url := range []string{"/uploads/" + name, name} {
			err := storage.Delete(url)
			if name == ".." {
				if err == nil {
					t.Errorf("Delete(%q): expected error, got nil", url)
				}
				continue
			}
			if err != nil {
				t.Errorf("Delete(%q): unexpected error %v", url, err)
			}
		}
	}

	if _, err := os.Stat(filepath.Join(dataDir, "secret.txt")); err != nil {
		t.Errorf("decoy file was disturbed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "evil.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("evil.txt must not exist outside media")
	}
}
