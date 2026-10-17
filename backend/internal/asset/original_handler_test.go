package asset

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Tests for Handler.Original, the GET /assets/:id/original route. They pin the
// wire contract a blog theme depends on: the exact bytes, the blob's MIME type
// and size, and the headers that decide whether a browser renders the file or
// executes it. They also pin the failure mapping (404 / 503 / 500) and that the
// route is mounted public, since embedded post media has to load for readers
// who hold no session.

// originalFixture mounts the original route on its own engine. The identity
// middleware the upload and delete tests use is deliberately absent: this route
// is documented without BearerAuth, and a test that needed a token would hide a
// regression that puts one back.
type originalFixture struct {
	h       *Handler
	engine  *gin.Engine
	db      *gorm.DB
	dataDir string

	store *originalStore
}

// originalStore is the real local backend plus the knobs a test needs: which
// keys Open was asked for, and whether the reader it handed out was closed.
// A nil err means "behave like local storage".
type originalStore struct {
	storage.Storage
	err    error
	opens  []string
	closed bool
}

func (s *originalStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	s.opens = append(s.opens, key)
	if s.err != nil {
		return nil, s.err
	}
	reader, err := s.Storage.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	return &trackedReadCloser{ReadCloser: reader, onClose: func() { s.closed = true }}, nil
}

type trackedReadCloser struct {
	io.ReadCloser
	onClose func()
}

func (r *trackedReadCloser) Close() error {
	r.onClose()
	return r.ReadCloser.Close()
}

func newOriginalFixture(t *testing.T, err error) *originalFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := newTestDB(t)
	dataDir := t.TempDir()
	store := &originalStore{Storage: storage.NewLocalStorage(dataDir), err: err}
	h := NewHandler(Deps{
		DB:        db,
		JWTSecret: handlerJWTSecret,
		Storage:   store,
	})

	engine := gin.New()
	engine.GET("/api/assets/:id/original", h.Original)

	return &originalFixture{h: h, engine: engine, db: db, dataDir: dataDir, store: store}
}

// get requests the route with a raw id, so a test can send shapes a real client
// would never produce.
func (f *originalFixture) get(t *testing.T, rawID string) *httptest.ResponseRecorder {
	t.Helper()
	return perform(
		t,
		f.engine,
		http.MethodGet,
		"/api/assets/"+rawID+"/original",
		nil,
		"",
		nil,
	)
}

// originalOf requests the asset by its real id.
func (f *originalFixture) originalOf(t *testing.T, assetID uint) *httptest.ResponseRecorder {
	t.Helper()
	return f.get(t, strconv.FormatUint(uint64(assetID), 10))
}

// seedOwnedAsset stores content for an asset owned by a freshly seeded user.
func (f *originalFixture) seedOwnedAsset(
	t *testing.T,
	owner model.User,
	name string,
	mimeType string,
	content []byte,
) model.Asset {
	t.Helper()
	return seedStoredAsset(t, f.db, f.dataDir, owner.ID, name, mimeType, content)
}

func TestHandlerOriginal_ServesStoredBytesWithBlobMetadata(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	content := pngPayload(64)
	asset := f.seedOwnedAsset(t, owner, "photo.png", "image/png", content)

	recorder := f.originalOf(t, asset.ID)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.Bytes(); string(got) != string(content) {
		t.Errorf("body = %d bytes, want the %d stored bytes", len(got), len(content))
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
	if got := recorder.Header().Get("Content-Length"); got != strconv.Itoa(len(content)) {
		t.Errorf("Content-Length = %q, want the blob size %d", got, len(content))
	}
}

// TestHandlerOriginal_ForcesDownloadSafeHeaders pins the two headers that decide
// whether the browser renders the file as an image or as a document: the stored
// MIME type alone must not be trusted, since a blob sniffed as HTML would run
// script on this origin. nosniff forbids the browser from second-guessing it,
// and the immutable cache header says the bytes behind an id never change,
// because every upload mints a fresh id.
func TestHandlerOriginal_ForcesDownloadSafeHeaders(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := f.seedOwnedAsset(t, owner, "photo.png", "image/png", pngPayload(64))

	recorder := f.originalOf(t, asset.ID)

	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	cacheControl := recorder.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "immutable") {
		t.Errorf("Cache-Control = %q, want it to mark the asset immutable", cacheControl)
	}
}

func TestHandlerOriginal_DisclosesFilenameInline(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := f.seedOwnedAsset(t, owner, "holiday photo.png", "image/png", pngPayload(64))

	recorder := f.originalOf(t, asset.ID)

	disposition := recorder.Header().Get("Content-Disposition")
	mediaType, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		t.Fatalf("Content-Disposition %q does not parse: %v", disposition, err)
	}
	if mediaType != "inline" {
		t.Errorf("Content-Disposition type = %q, want %q", mediaType, "inline")
	}
	if params["filename"] != "holiday photo.png" {
		t.Errorf("filename = %q, want %q", params["filename"], "holiday photo.png")
	}
}

// TestHandlerOriginal_EncodesNonASCIIFilename keeps a name the user typed from
// corrupting the response: the header must stay ASCII on the wire, and the
// original name must still survive decoding.
func TestHandlerOriginal_EncodesNonASCIIFilename(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	const name = "写真.png"
	asset := f.seedOwnedAsset(t, owner, name, "image/png", pngPayload(64))

	recorder := f.originalOf(t, asset.ID)

	disposition := recorder.Header().Get("Content-Disposition")
	if !utf8.ValidString(disposition) {
		t.Errorf("Content-Disposition %q is not valid UTF-8 on the wire", disposition)
	}
	for _, r := range disposition {
		if r > 127 {
			t.Errorf("Content-Disposition carries a raw %q on the wire: %q", r, disposition)
		}
	}
	_, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		t.Fatalf("Content-Disposition %q does not parse: %v", disposition, err)
	}
	if params["filename"] != name {
		t.Errorf("filename = %q, want %q", params["filename"], name)
	}
}

// TestHandlerOriginal_OmitsDispositionWithoutFilename covers the row written
// before the name was recorded: an empty disposition must not become
// `filename=""`, which some browsers render as a broken download.
func TestHandlerOriginal_OmitsDispositionWithoutFilename(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := f.seedOwnedAsset(t, owner, "", "image/png", pngPayload(64))

	recorder := f.originalOf(t, asset.ID)

	if got := recorder.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("Content-Disposition = %q, want no header for an unnamed asset", got)
	}
}

func TestHandlerOriginal_RejectsUnusableIDParam(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := f.seedOwnedAsset(t, owner, "photo.png", "image/png", pngPayload(64))

	for _, rawID := range []string{
		"0", "abc", "-1", "1.5", "1%20", "%201", "%2B1", "0x1",
		"18446744073709551616", // one past the widest uint
	} {
		recorder := f.get(t, rawID)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("id %q: status = %d, want 404", rawID, recorder.Code)
		}
		if recorder.Body.Len() != 0 {
			t.Errorf("id %q: body = %q, want it empty", rawID, recorder.Body.String())
		}
	}

	// The same asset answers under its real id, so the 404s above come from the
	// id and not from a fixture that serves nothing.
	if recorder := f.originalOf(t, asset.ID); recorder.Code != http.StatusOK {
		t.Fatalf("the seeded asset is unreachable: status = %d (body %s)",
			recorder.Code, recorder.Body.String())
	}

	// Only the one legitimate lookup reached storage.
	if len(f.store.opens) != 1 {
		t.Errorf("storage was asked for %v, want only the seeded asset's key", f.store.opens)
	}
}

func TestHandlerOriginal_NotFoundForUnknownAsset(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	f.seedOwnedAsset(t, owner, "photo.png", "image/png", pngPayload(64))

	recorder := f.originalOf(t, 4242)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want it empty", recorder.Body.String())
	}
}

// TestHandlerOriginal_NotFoundAfterDelete is the deletion guarantee on the read
// path: once Delete succeeds the bytes stop being publicly reachable.
func TestHandlerOriginal_NotFoundAfterDelete(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := f.seedOwnedAsset(t, owner, "photo.png", "image/png", pngPayload(64))

	if err := f.h.svc.Delete(
		context.Background(),
		Actor{ID: owner.ID, Role: owner.Role},
		asset.ID,
	); err != nil {
		t.Fatalf("failed to delete asset: %v", err)
	}

	recorder := f.originalOf(t, asset.ID)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 after deletion (body %s)", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "PNG") {
		t.Errorf("a deleted asset still served bytes: %q", recorder.Body.String())
	}
}

// TestHandlerOriginal_ServiceUnavailableWhenObjectMissing pins the middle
// outcome: the record is real, so a 404 would blame the caller's URL, but the
// bytes are gone, so serving anything else is impossible.
func TestHandlerOriginal_ServiceUnavailableWhenObjectMissing(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	// Row only: no object under its key.
	asset := seedAsset(t, f.db, owner.ID, "photo.png")

	recorder := f.originalOf(t, asset.ID)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() == 0 {
		t.Errorf("body is empty, want an error message")
	}
	errorField(t, recorder)
	if len(f.store.opens) != 1 {
		t.Errorf("storage was asked %d times, want 1", len(f.store.opens))
	}
}

func TestHandlerOriginal_HidesStorageFailures(t *testing.T) {
	f := newOriginalFixture(t, errStore)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := seedAsset(t, f.db, owner.ID, "photo.png")

	var blob model.Blob
	if err := f.db.First(&blob, asset.BlobID).Error; err != nil {
		t.Fatalf("failed to load blob: %v", err)
	}

	recorder := f.originalOf(t, asset.ID)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, not the 503 a missing object gets (body %s)",
			recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNoLeak(t, recorder, errStore.Error(), blob.StorageKey)
}

// TestHandlerOriginal_ClosesContentReader guards the file descriptor the reader
// holds: the route is public, so a leaked one exhausts the process under load.
func TestHandlerOriginal_ClosesContentReader(t *testing.T) {
	f := newOriginalFixture(t, nil)
	owner := seedUser(t, f.db, "author", model.RoleAuthor)
	asset := f.seedOwnedAsset(t, owner, "photo.png", "image/png", pngPayload(64))

	if recorder := f.originalOf(t, asset.ID); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !f.store.closed {
		t.Errorf("the content reader was never closed")
	}
}

// TestHandlerOriginal_IsPubliclyReadable mounts the routes exactly as the router
// does and reads an asset with no token at all. Post media is embedded in pages
// served to logged-out readers, so this route has to stay public; the id is
// unguessable and every upload mints a new one. If this ever needs a session,
// the change belongs in the route registration and in this test.
func TestHandlerOriginal_IsPubliclyReadable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDB(t)
	dataDir := t.TempDir()
	h := NewHandler(Deps{
		DB:        db,
		JWTSecret: handlerJWTSecret,
		Storage:   storage.NewLocalStorage(dataDir),
	})

	engine := gin.New()
	h.RegisterRoutes(engine.Group("/api"))

	owner := seedUser(t, db, "author", model.RoleAuthor)
	content := pngPayload(64)
	asset := seedStoredAsset(t, db, dataDir, owner.ID, "photo.png", "image/png", content)

	recorder := perform(
		t,
		engine,
		http.MethodGet,
		"/api/assets/"+strconv.FormatUint(uint64(asset.ID), 10)+"/original",
		nil,
		"",
		nil,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without an Authorization header (body %s)",
			recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.Bytes(); string(got) != string(content) {
		t.Errorf("body = %d bytes, want the %d stored bytes", len(got), len(content))
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
}
