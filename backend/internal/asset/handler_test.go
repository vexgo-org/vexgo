package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newTestRouter returns a router serving the asset routes with the JWT
// middleware stubbed out: the acting user is taken from a request header, so
// one handler can act as several users inside a test. The handler is returned
// so a test can swap its repository to reach the error branches.
func newTestRouter(t *testing.T) (*gin.Engine, *Handler, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := newTestDB(t)
	dataDir := t.TempDir()
	h := NewHandler(Deps{
		DB:        db,
		JWTSecret: []byte("test-jwt-secret-for-asset-tests!"),
		Storage:   storage.NewLocalStorage(dataDir),
	})

	asUser := func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.GetHeader("X-Test-User"), 10, 64)
		c.Set(middleware.CtxUserIDKey, uint(id))
	}

	r := gin.New()
	r.POST("/asset/upload", asUser, h.Upload)
	r.POST("/asset/delete/:key", asUser, h.Delete)
	return r, h, db, dataDir
}

// multipartBody builds a single-part multipart body.
func multipartBody(t *testing.T, field, filename string, payload []byte) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

// doPost sends a request to the router as userID. The asset routes are mounted
// without the JWT middleware, so the test acts as the user it names.
func doPost(
	t *testing.T,
	r *gin.Engine,
	userID uint,
	path string,
	contentType string,
	body io.Reader,
) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Test-User", strconv.FormatUint(uint64(userID), 10))
	r.ServeHTTP(w, req)
	return w
}

func doUpload(
	t *testing.T,
	r *gin.Engine,
	userID uint,
	field string,
	filename string,
	payload []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartBody(t, field, filename, payload)
	return doPost(t, r, userID, "/asset/upload", contentType, body)
}

func doDelete(t *testing.T, r *gin.Engine, userID uint, key string) *httptest.ResponseRecorder {
	t.Helper()
	return doPost(t, r, userID, "/asset/delete/"+key, "", nil)
}

// TestUpload_RejectsOversizeFile covers the handler's share of the size guard.
// MaxBytesReader only bounds the whole request, so a body just over the
// per-file cap still parses; the handler has to refuse it and store nothing.
func TestUpload_RejectsOversizeFile(t *testing.T) {
	r, _, db, dataDir := newTestRouter(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	oversize := bytes.Repeat([]byte("a"), model.MaxAssetBytes+1)
	w := doUpload(t, r, user.ID, "file", "huge.jpg", oversize)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	// Exactly one JSON object: the handler must stop at the size guard instead
	// of falling through and writing a second response.
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("response is not a single JSON object: %v (body: %s)", err, w.Body.String())
	}
	if response["error"] != "File is too large" {
		t.Errorf("error = %v, want %q", response["error"], "File is too large")
	}
	if names := storedFiles(t, dataDir); len(names) != 0 {
		t.Errorf("oversize upload left %v in storage", names)
	}
}

func TestUpload_RequiresFilePart(t *testing.T) {
	r, _, db, _ := newTestRouter(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	// A part under a different field name is as good as no part at all.
	w := doUpload(t, r, user.ID, "attachment", "photo.jpg", []byte("x"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a missing 'file' part (body: %s)", w.Code, w.Body.String())
	}

	// A body that is not multipart at all is refused the same way.
	w = doPost(t, r, user.ID, "/asset/upload", "application/json", strings.NewReader(`{"file":"x"}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a non-multipart body (body: %s)", w.Code, w.Body.String())
	}
}

// TestDelete_MapsServiceErrorsToStatus covers the status mapping: only the
// uploader and admins get a 200, a stranger 403, and an unknown or
// already-deleted key 404.
func TestDelete_MapsServiceErrorsToStatus(t *testing.T) {
	r, _, db, _ := newTestRouter(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	stranger := seedUser(t, db, "stranger", model.RoleContributor)
	admin := seedUser(t, db, "admin", model.RoleSuperAdmin)

	w := doUpload(t, r, owner.ID, "file", "mine.jpg", []byte("x"))
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var uploaded struct {
		File model.Asset `json:"file"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v (body: %s)", err, w.Body.String())
	}
	key := uploaded.File.StorageKey
	if key == "" {
		t.Fatalf("upload response has no storage key: %s", w.Body.String())
	}

	if w := doDelete(t, r, stranger.ID, key); w.Code != http.StatusForbidden {
		t.Errorf("stranger status = %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
	if w := doDelete(t, r, owner.ID, "no-such-key.jpg"); w.Code != http.StatusNotFound {
		t.Errorf("unknown key status = %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
	if w := doDelete(t, r, owner.ID, key); w.Code != http.StatusOK {
		t.Errorf("owner status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	// Already soft-deleted, so the key is gone for the next caller too.
	if w := doDelete(t, r, owner.ID, key); w.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
	if w := doDelete(t, r, admin.ID, "no-such-key.jpg"); w.Code != http.StatusNotFound {
		t.Errorf("admin unknown key status = %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}

// failingStorage always fails with the error it was given, so the handler's
// error paths can be exercised without a real storage backend.
type failingStorage struct{ err error }

func (s failingStorage) Put(context.Context, string, io.Reader, int64, string) error { return s.err }

func (s failingStorage) Delete(context.Context, string) error { return nil }

func (s failingStorage) Open(context.Context, string) (io.ReadCloser, error) { return nil, s.err }

func (s failingStorage) URL(context.Context, string) (string, error) { return "", s.err }

// TestUpload_DoesNotLeakStorageErrors pins the contract that a persistence
// failure is logged server-side and answered with a generic 500: the storage
// error may carry paths or backend addresses and must never reach the client.
func TestUpload_DoesNotLeakStorageErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const internal = "s3://bucket/../../../etc/shadow: dial tcp 10.0.0.5:9000"
	h := NewHandler(Deps{
		JWTSecret: []byte("test-jwt-secret-for-asset-tests!"),
		Storage:   failingStorage{err: errors.New(internal)},
	})

	r := gin.New()
	r.POST("/asset/upload", func(c *gin.Context) {
		c.Set(middleware.CtxUserIDKey, uint(7))
		h.Upload(c)
	})

	body, contentType := multipartBody(t, "file", "photo.png", []byte("not really a png"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/asset/upload", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", w.Code, w.Body.String())
	}
	for _, leak := range []string{"s3://", "10.0.0.5", "dial tcp", "etc/shadow"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("internal storage detail leaked to client: %q present in body %s", leak, w.Body.String())
		}
	}
}

// failingAssetLookupRepo makes the asset lookup fail with a database-flavored
// error, so the handler's unmapped 500 branch is reached.
type failingAssetLookupRepo struct{ Repository }

func (failingAssetLookupRepo) FindAssetByStorageKey(context.Context, string) (*model.Asset, error) {
	return nil, errors.New("dial tcp 10.0.0.5:5432: connection refused")
}

// TestDelete_InternalFailureIsGeneric500 pins that an unexpected lookup failure
// is a plain 500: it is neither reported as a missing asset nor allowed to
// carry the backend address into the response.
func TestDelete_InternalFailureIsGeneric500(t *testing.T) {
	r, h, db, _ := newTestRouter(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)

	w := doUpload(t, r, owner.ID, "file", "mine.jpg", []byte("x"))
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var uploaded struct {
		File model.Asset `json:"file"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v (body: %s)", err, w.Body.String())
	}

	h.svc.repo = failingAssetLookupRepo{Repository: h.svc.repo}

	w = doDelete(t, r, owner.ID, uploaded.File.StorageKey)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", w.Code, w.Body.String())
	}
	for _, leak := range []string{"10.0.0.5", "5432", "dial tcp", "connection refused"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("internal database detail leaked to client: %q present in body %s", leak, w.Body.String())
		}
	}
}
