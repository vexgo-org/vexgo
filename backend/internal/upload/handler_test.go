package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestGenerateFilename_LimitsExtensionToAllowlist ensures the client-supplied
// extension survives only when it is an allowlisted (non-executable) media
// type; anything else — separators, HTML/SVG documents, colons, overlong
// tails — yields a bare UUID name served as application/octet-stream.
func TestGenerateFilename_LimitsExtensionToAllowlist(t *testing.T) {
	cases := []struct {
		name     string
		original string
		wantExt  string
	}{
		{"plain jpg", "photo.jpg", ".jpg"},
		{"uppercase normalized", "PHOTO.JPG", ".jpg"},
		{"multi-dot takes last", "archive.tar.png", ".png"},
		{"disallowed gz stripped", "archive.tar.gz", ""},
		{"no extension", "noext", ""},
		{"dot only", "x.", ""},
		{"html stripped", "evil.html", ""},
		{"htm stripped", "evil.htm", ""},
		{"svg stripped", "evil.svg", ""},
		{"xhtml stripped", "evil.xhtml", ""},
		{"js stripped", "evil.js", ""},
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
		if _, err := storage.Upload(context.Background(), strings.NewReader("evil"), name, ""); err == nil {
			t.Errorf("Upload(%q): expected error, got nil", name)
		}
		// Delete cannot escape either: filepath.Base neutralizes traversal
		// segments (missing files stay "not an error"), the ".." name is
		// rejected outright, and os.Root confines the removal; the decoy must
		// survive all of them.
		for _, url := range []string{"/uploads/" + name, name} {
			err := storage.Delete(context.Background(), url)
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

// failingStorage always fails Upload with the error it was given, so the
// handler's error path can be exercised without a real storage backend.
type failingStorage struct{ err error }

func (s failingStorage) Upload(context.Context, io.Reader, string, string) (string, error) {
	return "", s.err
}

func (s failingStorage) Delete(context.Context, string) error { return nil }

// TestUploadFile_DoesNotLeakStorageErrors pins the contract that a persistence
// failure is logged server-side and answered with a generic 500: the storage
// error may carry paths or backend addresses and must never reach the client.
func TestUploadFile_DoesNotLeakStorageErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const internal = "s3://bucket/../../../etc/shadow: dial tcp 10.0.0.5:9000"
	h := NewHandler(Deps{
		JWTSecret: []byte("test-jwt-secret-for-upload-tests!"),
		Storage:   failingStorage{err: errors.New(internal)},
	})

	r := gin.New()
	r.POST("/upload", func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, uint(7))
		h.UploadFile(c)
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "photo.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("not really a png")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", w.Code, w.Body.String())
	}
	response := w.Body.String()
	for _, leak := range []string{"s3://", "10.0.0.5", "dial tcp", "etc/shadow"} {
		if strings.Contains(response, leak) {
			t.Errorf("internal storage detail leaked to client: %q present in body %s", leak, response)
		}
	}
	if !strings.Contains(response, "Failed to upload") {
		t.Errorf("expected a generic failure message, got body %s", response)
	}
}
