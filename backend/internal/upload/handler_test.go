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
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestGenerateFilename_LimitsExtensionToAllowlist ensures the client-supplied
// extension survives only when it is an allowlisted media type; anything else —
// separators, HTML documents, colons, overlong tails — yields a bare UUID name
// served as application/octet-stream.
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
		{"svg kept", "icon.svg", ".svg"},
		{"uppercase svg normalized", "ICON.SVG", ".svg"},
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

// TestLocalStorage_ContainsHostileFilenames ensures os.Root confinement: a
// hostile key can never create, read or delete a file outside the media
// directory. Storage takes a key, not a URL, so every escaping key is refused
// by cleanKey before touching the filesystem — absolute paths, backslashes and
// ".." segments alike. The decoy must survive all of it.
func TestLocalStorage_ContainsHostileFilenames(t *testing.T) {
	dataDir := t.TempDir()
	store := storage.NewLocalStorage(dataDir)

	rootDir := path.Join(dataDir, "media")
	if err := os.MkdirAll(rootDir, 0o750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// A decoy inside the media tree, next to where a traversal would land.
	if err := os.WriteFile(filepath.Join(rootDir, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("seed decoy: %v", err)
	}

	// -1 skips the size check so each case fails on the key, not on a size
	// mismatch that would mask the assertion.
	for _, key := range []string{
		"../evil.txt",
		"media/../../evil.txt",
		"/tmp/evil.txt",
		"/evil.txt",
		`..\evil.txt`,
		"..",
		".",
		"",
	} {
		if err := store.Put(
			context.Background(),
			key,
			strings.NewReader("evil"),
			-1,
			"",
		); !errors.Is(err, storage.ErrInvalidKey) {
			t.Errorf("Put(%q) = %v, want ErrInvalidKey", key, err)
		}
		if err := store.Delete(context.Background(), key); !errors.Is(err, storage.ErrInvalidKey) {
			t.Errorf("Delete(%q) = %v, want ErrInvalidKey", key, err)
		}
		if _, err := store.Open(context.Background(), key); !errors.Is(err, storage.ErrInvalidKey) {
			t.Errorf("Open(%q) = %v, want ErrInvalidKey", key, err)
		}
		if _, err := store.URL(context.Background(), key); !errors.Is(err, storage.ErrInvalidKey) {
			t.Errorf("URL(%q) = %v, want ErrInvalidKey", key, err)
		}
	}

	if _, err := os.Stat(filepath.Join(rootDir, "secret.txt")); err != nil {
		t.Errorf("decoy file was disturbed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "evil.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("evil.txt must not exist outside media")
	}

	// A valid key that simply is not there is not an error, so deleting an
	// already-deleted file stays idempotent.
	if err := store.Delete(context.Background(), "absent.txt"); err != nil {
		t.Errorf("Delete of a missing object = %v, want nil", err)
	}
}

// TestLocalStorage_PublishesObjectUnderItsKey pins that a successful Put
// leaves the bytes readable under the key the caller asked for. The object is
// staged under a temporary name first, so a Put that never publishes it would
// hand the caller a URL to a file that does not exist.
func TestLocalStorage_PublishesObjectUnderItsKey(t *testing.T) {
	dataDir := t.TempDir()
	store := storage.NewLocalStorage(dataDir)
	ctx := context.Background()
	const payload = "jpeg-data"

	if err := store.Put(ctx, "photo.png", strings.NewReader(payload), int64(len(payload)), ""); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	url, err := store.URL(ctx, "photo.png")
	if err != nil {
		t.Fatalf("URL error: %v", err)
	}
	if url != "/uploads/photo.png" {
		t.Errorf("URL = %q, want /uploads/photo.png", url)
	}

	rc, err := store.Open(ctx, "photo.png")
	if err != nil {
		t.Fatalf("Open after Put = %v, want the published object", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(got) != payload {
		t.Errorf("object content = %q, want %q", got, payload)
	}

	// No staging file may be left behind: it would be an unlistable orphan.
	entries, err := os.ReadDir(filepath.Join(dataDir, "media"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "photo.png" {
			t.Errorf("unexpected leftover in media dir: %s", entry.Name())
		}
	}

	// And a published object is deletable, unlike a stranded staging file.
	if err := store.Delete(ctx, "photo.png"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if _, err := store.Open(ctx, "photo.png"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Open after Delete = %v, want ErrNotFound", err)
	}
}

// TestLocalStorage_PutRejectsSizeMismatch covers the guard that keeps a
// truncated upload from being recorded as a successful one: the declared size
// and the stored size have to agree, and the staging file is cleaned up.
func TestLocalStorage_PutRejectsSizeMismatch(t *testing.T) {
	dataDir := t.TempDir()
	store := storage.NewLocalStorage(dataDir)

	err := store.Put(context.Background(), "short.png", strings.NewReader("tiny"), 42, "")
	if err == nil {
		t.Fatal("expected a size mismatch error, got nil")
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, "media"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("rejected upload left %d file(s) behind", len(entries))
	}
}

// failingStorage always fails Put with the error it was given, so the
// handler's error path can be exercised without a real storage backend.
type failingStorage struct{ err error }

func (s failingStorage) Put(context.Context, string, io.Reader, int64, string) error {
	return s.err
}

func (s failingStorage) Delete(context.Context, string) error { return nil }

func (s failingStorage) Open(context.Context, string) (io.ReadCloser, error) { return nil, s.err }

func (s failingStorage) URL(context.Context, string) (string, error) { return "", s.err }

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
