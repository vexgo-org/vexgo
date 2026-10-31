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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tests for the two HTTP entry points, Handler.Upload and Handler.Delete.
// They pin the wire contract (status codes, the JSON shape the SPA reads, and
// what must never reach the client) and the side effects behind it: one asset
// row, one blob row, and one object in storage per accepted upload, and nothing
// at all for a rejected one.

// handlerJWTSecret must match the secret handed to NewHandler, so tokens
// minted for the end-to-end route pass h.mw.JWTAuth.
var handlerJWTSecret = []byte("asset-handler-test-secret-at-least-32-bytes-long")

// handlerFixture carries the handler plus the three engines the tests need:
// bare routes let a test drive the gin context directly, guarded routes run
// the real JWT middleware end to end, and routed routes mount RegisterRoutes
// itself, so the role gate and the documented paths are what a request meets.
type handlerFixture struct {
	h       *Handler
	bare    *gin.Engine
	guarded *gin.Engine
	routed  *gin.Engine
	dataDir string
	db      *gorm.DB
}

func newHandlerFixture(t *testing.T) *handlerFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := newTestDB(t)
	dataDir := t.TempDir()
	h := NewHandler(Deps{
		DB:        db,
		JWTSecret: handlerJWTSecret,
		Storage:   storage.NewLocalStorage(dataDir),
	})

	// Both engines mount the same two routes; only the middleware differs.
	bare := gin.New()
	bare.Use(testIdentity())
	registerAssetRoutes(bare, h)

	guarded := gin.New()
	guarded.Use(h.mw.JWTAuth())
	registerAssetRoutes(guarded, h)

	// This one goes through RegisterRoutes, so the middleware chain and the
	// paths come from route.go rather than from the test file. The other two
	// engines exist to drive a single handler; this one proves the routes as
	// registered are the ones that enforce the role gate.
	routed := gin.New()
	h.RegisterRoutes(routed.Group("/api"))

	return &handlerFixture{
		h:       h,
		bare:    bare,
		guarded: guarded,
		routed:  routed,
		dataDir: dataDir,
		db:      db,
	}
}

// registerAssetRoutes mounts the two handlers directly at the fixture's own
// paths. They intentionally differ from route.go, which owns the documented
// paths; the routed engine covers those.
func registerAssetRoutes(r *gin.Engine, h *Handler) {
	r.POST("/api/asset/upload", h.Upload)
	r.POST("/api/asset/delete/:publicID", h.Delete)
}

// testIdentity stands in for middleware.Auth.JWTAuth on the bare engine, so a
// test can choose who the handler sees without minting a token. It writes only
// the two context keys the real middleware writes. Headers:
//
//	X-Test-User-ID: numeric user id, absent means anonymous
//	X-Test-Role:    the role to publish alongside the id
//	X-Test-No-Role: set to skip the role, for a context that knows the user but
//	                not its privileges
func testIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("X-Test-User-ID")
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if raw == "" || err != nil {
			c.Next()
			return
		}

		c.Set(middleware.CtxUserIDKey, uint(parsed))
		if c.GetHeader("X-Test-No-Role") != "" {
			c.Next()
			return
		}
		c.Set(middleware.CtxUserKey, map[string]any{
			"id":       uint(parsed),
			"username": "tester",
			"role":     c.GetHeader("X-Test-Role"),
		})
		c.Next()
	}
}

// mintToken signs a token the way the login endpoint does.
func mintToken(t *testing.T, userID uint, role string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":          float64(userID),
		"username":         "tester",
		"role":             role,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	})
	signed, err := token.SignedString(handlerJWTSecret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// multipartBody builds an in-memory multipart form with a single part.
func multipartBody(
	t *testing.T,
	fieldName,
	filename string,
	content []byte,
) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

// streamMultipartBody streams a form file of exactly size bytes, so the size
// guards can be exercised without materializing tens of megabytes in the test
// binary.
func streamMultipartBody(
	t *testing.T,
	fieldName,
	filename string,
	size int64,
) (io.ReadCloser, string) {
	t.Helper()
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	contentType := form.FormDataContentType()

	go func() {
		part, err := form.CreateFormFile(fieldName, filename)
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, io.LimitReader(zeroes{}, size)); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = form.Close()
		_ = writer.Close()
	}()

	// The handler stops reading at its body cap; closing the read end turns the
	// blocked write into an error instead of leaking the goroutine.
	t.Cleanup(func() { _ = reader.Close() })

	return reader, contentType
}

// zeroes is an endless zero stream, used to fill an oversized form part.
type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// perform issues one request against the given engine.
func perform(
	t *testing.T,
	engine *gin.Engine,
	method,
	path string,
	body io.Reader,
	contentType string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

// uploadAs performs an upload as the given user through the bare engine.
func (f *handlerFixture) uploadAs(
	t *testing.T,
	user model.User,
	body io.Reader,
	contentType string,
) *httptest.ResponseRecorder {
	t.Helper()
	return perform(t, f.bare, http.MethodPost, "/api/asset/upload", body, contentType,
		identityOf(user))
}

func (f *handlerFixture) uploadAnonymous(
	t *testing.T,
	body io.Reader,
	contentType string,
) *httptest.ResponseRecorder {
	t.Helper()
	return perform(t, f.bare, http.MethodPost, "/api/asset/upload", body, contentType, nil)
}

// deleteAs performs a delete as the given user through the bare engine. The
// public ID goes into the path verbatim, so a test can send shapes a real
// client would never produce on purpose.
func (f *handlerFixture) deleteAs(
	t *testing.T,
	user model.User,
	publicID string,
) *httptest.ResponseRecorder {
	t.Helper()
	return perform(t, f.bare, http.MethodPost, "/api/asset/delete/"+publicID, nil, "", identityOf(user))
}

// identityOf builds the headers testIdentity turns into an authenticated
// context.
func identityOf(user model.User) map[string]string {
	return map[string]string{
		"X-Test-User-ID": strconv.FormatUint(uint64(user.ID), 10),
		"X-Test-Role":    user.Role,
	}
}

// bearerOf builds the header the real JWTAuth middleware reads, so a request
// through the routed engine is authenticated exactly as the API authenticates
// it.
func bearerOf(t *testing.T, user model.User) map[string]string {
	t.Helper()
	return map[string]string{
		"Authorization": "Bearer " + mintToken(t, user.ID, user.Role),
	}
}

// uploadOnRoutedRoute performs an upload through RegisterRoutes, so every
// middleware in route.go runs.
func (f *handlerFixture) uploadOnRoutedRoute(
	t *testing.T,
	user model.User,
	body io.Reader,
	contentType string,
) *httptest.ResponseRecorder {
	t.Helper()
	return perform(t, f.routed, http.MethodPost, "/api/assets/upload",
		body, contentType, bearerOf(t, user))
}

// deleteOnRoutedRoute performs a delete through RegisterRoutes, so every
// middleware in route.go runs.
func (f *handlerFixture) deleteOnRoutedRoute(
	t *testing.T,
	user model.User,
	publicID uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()
	return perform(t, f.routed, http.MethodPost,
		"/api/assets/delete/"+publicID.String(), nil, "", bearerOf(t, user))
}

// --- shared assertions --------------------------------------------------

func decodeJSON(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON object: %v (body %q)", err, recorder.Body.String())
	}
	return body
}

// errorField asserts the api.ErrorResponse contract: a non-2xx answer carries a
// non-empty, human-readable "error" and nothing else that could confuse a
// client.
func errorField(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	body := decodeJSON(t, recorder)
	message, ok := body["error"].(string)
	if !ok || message == "" {
		t.Errorf("error body = %v, want a non-empty %q field", body, "error")
	}
	return message
}

// decodeUploadResponse decodes a success body as UploadResponse, so the
// assertion reads the documented field names instead of guessing them.
func decodeUploadResponse(t *testing.T, recorder *httptest.ResponseRecorder) UploadResponse {
	t.Helper()
	var response UploadResponse
	decodeInto(t, recorder, &response)
	return response
}

// decodeMessageResponse decodes a success body as MessageResponse.
func decodeMessageResponse(t *testing.T, recorder *httptest.ResponseRecorder) api.MessageResponse {
	t.Helper()
	var response api.MessageResponse
	decodeInto(t, recorder, &response)
	return response
}

// decodeInto checks the content type and unmarshals the body into target.
func decodeInto(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("response does not decode: %v (body %q)", err, recorder.Body.String())
	}
}

// assertKeysAre pins the serialized field names, so the generated TypeScript
// client cannot drift from what the SPA reads. Extra keys are as much a
// contract break as missing ones.
func assertKeysAre(t *testing.T, recorder *httptest.ResponseRecorder, want ...string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON object: %v (body %q)", err, recorder.Body.String())
	}
	for _, key := range want {
		if _, ok := body[key]; !ok {
			t.Errorf("response %v is missing the %q field", body, key)
		}
	}
	if len(body) != len(want) {
		t.Errorf("response %v carries %d field(s), want exactly %v", body, len(body), want)
	}
}

// assertNoLeak fails when the body echoes an internal detail. Storage and
// database errors carry paths, keys and backend addresses, none of which may
// reach the client.
func assertNoLeak(t *testing.T, recorder *httptest.ResponseRecorder, internals ...string) {
	t.Helper()
	body := recorder.Body.String()
	for _, internal := range internals {
		if strings.Contains(body, internal) {
			t.Errorf("internal detail %q leaked to the client in %q", internal, body)
		}
	}
}

// assertNothingStored is the invariant every rejected upload must satisfy: no
// row, no object. There is no record to delete an orphan through the API.
func assertNothingStored(t *testing.T, f *handlerFixture) {
	t.Helper()
	if n := countAssets(t, f.db); n != 0 {
		t.Errorf("rejected upload created %d asset record(s)", n)
	}
	if n := countBlobs(t, f.db); n != 0 {
		t.Errorf("rejected upload created %d blob record(s)", n)
	}
	if files := mediaFiles(t, f.dataDir); len(files) != 0 {
		t.Errorf("rejected upload left %v in storage", files)
	}
}

// --- Handler.Upload -----------------------------------------------------

func TestHandlerUpload_RequiresAuthentication(t *testing.T) {
	f := newHandlerFixture(t)
	body, contentType := multipartBody(t, "file", "photo.png", pngPayload(64))

	recorder := f.uploadAnonymous(t, body, contentType)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNothingStored(t, f)
}

// TestHandlerUpload_RequiresAuthenticationOnMountedRoute proves the endpoint
// stays behind the real auth middleware when it is mounted as documented, so a
// missing token is refused before the handler ever runs.
func TestHandlerUpload_RequiresAuthenticationOnMountedRoute(t *testing.T) {
	f := newHandlerFixture(t)
	body, contentType := multipartBody(t, "file", "photo.png", pngPayload(64))

	recorder := perform(t, f.guarded, http.MethodPost, "/api/asset/upload",
		body, contentType, nil)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", recorder.Code, recorder.Body.String())
	}
	assertNothingStored(t, f)
}

// TestHandlerUpload_StoresAssetAndBlob is the happy path: the client gets back
// something it can reference, and the asset, blob and object all exist.
func TestHandlerUpload_StoresAssetAndBlob(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)

	payload := pngPayload(4096)
	body, contentType := multipartBody(t, "file", "holiday.png", payload)

	recorder := f.uploadAs(t, user, body, contentType)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	var asset model.Asset
	if err := f.db.Preload("Blob").First(&asset).Error; err != nil {
		t.Fatalf("no asset row was created: %v", err)
	}

	// The body is UploadResponse: a message plus the public ID the SPA refers
	// to the asset by. Nothing more is documented, so nothing else may be
	// serialized.
	response := decodeUploadResponse(t, recorder)
	if response.Message == "" {
		t.Error("message is empty, want a human-readable confirmation")
	}
	if response.PublicID != asset.PublicID {
		t.Errorf("asset_public_id = %s, want the created asset %s", response.PublicID, asset.PublicID)
	}
	assertKeysAre(t, recorder, "message", "assetPublicID", "assetOriginalURL")
	// The public ID has to cross the wire as a canonical UUID string, because
	// that is what the SPA pastes back into the URL.
	if got := decodeJSON(t, recorder)["assetPublicID"]; got != asset.PublicID.String() {
		t.Errorf("assetPublicID = %v, want the string %q", got, asset.PublicID.String())
	}

	if got := decodeJSON(t, recorder)["assetOriginalURL"]; got != OriginalURL(asset.PublicID) {
		t.Errorf("assetOriginalURL = %v, want the string %q", got, OriginalURL(asset.PublicID))
	}

	// The storage key is deliberately not part of the response: it is
	// addressed server side, and handing it out would let a client reach the
	// backend's layout.
	if strings.Contains(recorder.Body.String(), asset.Blob.StorageKey) {
		t.Errorf("response exposes the storage key %q: %s",
			asset.Blob.StorageKey, recorder.Body.String())
	}

	// The record belongs to the authenticated user, under the name the client
	// sent, with the bytes the client sent.
	if asset.UserID != user.ID {
		t.Errorf("owner = %d, want %d", asset.UserID, user.ID)
	}
	if asset.OriginalName != "holiday.png" {
		t.Errorf("OriginalName = %q, want %q", asset.OriginalName, "holiday.png")
	}
	if asset.Blob.Size != int64(len(payload)) {
		t.Errorf("blob size = %d, want %d", asset.Blob.Size, len(payload))
	}
	if asset.Blob.MimeType != "image/png" {
		t.Errorf("blob mime = %q, want image/png", asset.Blob.MimeType)
	}
	if asset.Blob.Type != model.BlobTypeImage {
		t.Errorf("blob type = %q, want %q", asset.Blob.Type, model.BlobTypeImage)
	}

	stored, err := os.ReadFile(filepath.Join(f.dataDir, "media", asset.Blob.StorageKey))
	if err != nil {
		t.Fatalf("object not written to storage: %v", err)
	}
	if !bytes.Equal(stored, payload) {
		t.Errorf("stored %d bytes, want the %d bytes that were uploaded", len(stored), len(payload))
	}

	if files := mediaFiles(t, f.dataDir); len(files) != 1 {
		t.Errorf("storage holds %v, want exactly the uploaded object", files)
	}
}

// TestHandlerUpload_RandomizesKeyButKeepsFilename pins the trust boundary: the
// stored name is generated server side, while the original name is only
// metadata. A client cannot choose where its bytes land.
func TestHandlerUpload_RandomizesKeyButKeepsFilename(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)

	body, contentType := multipartBody(t, "file", "My Holiday Photo.PNG", pngPayload(2048))
	if recorder := f.uploadAs(t, user, body, contentType); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	var asset model.Asset
	if err := f.db.Preload("Blob").First(&asset).Error; err != nil {
		t.Fatalf("no asset row was created: %v", err)
	}

	if !strings.HasSuffix(asset.Blob.StorageKey, ".png") {
		t.Errorf("storage key = %q, want a normalized .png suffix", asset.Blob.StorageKey)
	}
	stem := strings.TrimSuffix(asset.Blob.StorageKey, ".png")
	if _, err := uuid.Parse(stem); err != nil {
		t.Errorf("storage key %q is not `uuid.ext`: %v", asset.Blob.StorageKey, err)
	}
	if strings.ContainsAny(asset.Blob.StorageKey, `/\`) {
		t.Errorf("storage key %q contains a path separator", asset.Blob.StorageKey)
	}
	if strings.Contains(strings.ToLower(asset.Blob.StorageKey), "holiday") {
		t.Errorf("storage key %q leaked the client filename", asset.Blob.StorageKey)
	}
	if asset.OriginalName != "My Holiday Photo.PNG" {
		t.Errorf("OriginalName = %q, want the name the client sent", asset.OriginalName)
	}
}

// TestHandlerUpload_StripsPathFromClientFilename covers a hostile filename: a
// multipart part may name "../../../etc/passwd", and nothing about that may
// escape the media directory or reach the key.
func TestHandlerUpload_StripsPathFromClientFilename(t *testing.T) {
	for _, filename := range []string{
		"../../../etc/passwd.png",
		"/absolute/evil.png",
		"dir/subdir/photo.png",
	} {
		t.Run(filename, func(t *testing.T) {
			f := newHandlerFixture(t)
			user := seedUser(t, f.db, "uploader", model.RoleContributor)

			body, contentType := multipartBody(t, "file", filename, pngPayload(1024))
			recorder := f.uploadAs(t, user, body, contentType)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}

			var asset model.Asset
			if err := f.db.Preload("Blob").First(&asset).Error; err != nil {
				t.Fatalf("no asset row was created: %v", err)
			}
			// mime/multipart applies filepath.Base to the part's filename, so a
			// forward-slash path never survives as the recorded name.
			if strings.Contains(asset.OriginalName, "/") {
				t.Errorf("OriginalName = %q, want the directory stripped", asset.OriginalName)
			}
			if strings.ContainsAny(asset.Blob.StorageKey, `/\`) {
				t.Errorf("storage key %q contains a path separator", asset.Blob.StorageKey)
			}

			assertContained(t, f, asset.Blob.StorageKey)
		})
	}
}

// TestHandlerUpload_KeepsWindowsStyleFilenameAsMetadata pins the one name shape
// that does survive: filepath.Base splits on "/" only, so a backslash path is
// kept verbatim. That is acceptable precisely because the name is inert
// metadata — the stored object is addressed by the generated key, so it cannot
// steer where bytes land. If OriginalName ever grows a use that joins it onto a
// path, this is the assertion that should stop passing.
func TestHandlerUpload_KeepsWindowsStyleFilenameAsMetadata(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)

	body, contentType := multipartBody(t, "file", `..\..\windows\evil.png`, pngPayload(1024))
	recorder := f.uploadAs(t, user, body, contentType)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	var asset model.Asset
	if err := f.db.Preload("Blob").First(&asset).Error; err != nil {
		t.Fatalf("no asset row was created: %v", err)
	}

	// What matters: the key is generated, so the separators are gone, and the
	// object is inside the media directory.
	if strings.ContainsAny(asset.Blob.StorageKey, `/\`) {
		t.Errorf("storage key %q contains a path separator", asset.Blob.StorageKey)
	}
	if !strings.HasSuffix(asset.Blob.StorageKey, ".png") {
		t.Errorf("storage key = %q, want the .png suffix to survive", asset.Blob.StorageKey)
	}
	assertContained(t, f, asset.Blob.StorageKey)
}

// assertContained checks that an upload produced exactly one object inside the
// media directory and nothing next to it.
func assertContained(t *testing.T, f *handlerFixture, storageKey string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(f.dataDir, "media", storageKey)); err != nil {
		t.Errorf("object %q is not inside the media directory: %v", storageKey, err)
	}
	for _, escaped := range []string{"passwd.png", "evil.png", "windows"} {
		if _, err := os.Stat(filepath.Join(f.dataDir, escaped)); err == nil {
			t.Errorf("%q escaped the media directory", escaped)
		}
	}
	if files := mediaFiles(t, f.dataDir); len(files) != 1 {
		t.Errorf("storage holds %v, want exactly the uploaded object", files)
	}
}

// TestHandlerUpload_RejectsMalformedForm covers every way the "file" part can
// be missing or unreadable. All of them are the client's problem, and none may
// reach storage.
func TestHandlerUpload_RejectsMalformedForm(t *testing.T) {
	tests := []struct {
		name        string
		body        func(t *testing.T) (io.Reader, string)
		wantMessage string
	}{
		{
			name: "no body at all",
			body: func(*testing.T) (io.Reader, string) {
				return nil, ""
			},
		},
		{
			name: "json body instead of multipart",
			body: func(*testing.T) (io.Reader, string) {
				return strings.NewReader(`{"file":"photo.png"}`), "application/json"
			},
		},
		{
			name: "multipart with another field name",
			body: func(t *testing.T) (io.Reader, string) {
				return multipartBody(t, "upload", "photo.png", pngPayload(64))
			},
		},
		{
			name: "multipart with no parts",
			body: func(*testing.T) (io.Reader, string) {
				var buffer bytes.Buffer
				writer := multipart.NewWriter(&buffer)
				if err := writer.Close(); err != nil {
					t.Fatalf("failed to close multipart writer: %v", err)
				}
				return &buffer, writer.FormDataContentType()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newHandlerFixture(t)
			user := seedUser(t, f.db, "uploader", model.RoleContributor)

			body, contentType := tt.body(t)
			recorder := f.uploadAs(t, user, body, contentType)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			errorField(t, recorder)
			assertNothingStored(t, f)
		})
	}
}

// TestHandlerUpload_RejectsInvalidFileContent covers the client mistakes the
// service refuses after the request was well formed: content whose sniffed MIME
// type has no blob type, and a filename too long for the column. Each is the
// client's to fix, so each is a 400 rather than the 500 the handler used to
// answer, and none of them may leave an object behind.
func TestHandlerUpload_RejectsInvalidFileContent(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
	}{
		{
			name:     "pdf is not a blob type",
			filename: "paper.pdf",
			content:  []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\n"),
		},
		{
			name:     "text behind an image name",
			filename: "photo.png",
			content:  []byte("just some plain text"),
		},
		{
			name:     "filename over the column limit",
			filename: strings.Repeat("a", model.MaxAssetFilenameLength) + ".png",
			content:  pngPayload(128),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newHandlerFixture(t)
			user := seedUser(t, f.db, "uploader", model.RoleContributor)

			body, contentType := multipartBody(t, "file", tt.filename, tt.content)
			recorder := f.uploadAs(t, user, body, contentType)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			errorField(t, recorder)
			assertNothingStored(t, f)
		})
	}
}

// TestHandlerUpload_RejectsDisallowedExtension: the allowlist is enforced, and
// the rejection is the client's to fix, so it is a 400 and never a stored row.
func TestHandlerUpload_RejectsDisallowedExtension(t *testing.T) {
	for _, filename := range []string{
		"evil.html", "evil.htm", "evil.xhtml", "evil.xml", "evil.js",
		"evil.exe", "evil.sh", "archive.tar.gz", "noextension",
	} {
		t.Run(filename, func(t *testing.T) {
			f := newHandlerFixture(t)
			user := seedUser(t, f.db, "uploader", model.RoleContributor)

			body, contentType := multipartBody(t, "file", filename, []byte("<script>alert(1)</script>"))
			recorder := f.uploadAs(t, user, body, contentType)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			errorField(t, recorder)
			assertNothingStored(t, f)
		})
	}
}

// TestHandlerUpload_RejectsEmptyFile: a zero-byte part has no header to sniff,
// so it cannot produce a blob. The client sent it, so the answer is a 4xx that
// says which part of the request was wrong.
func TestHandlerUpload_RejectsEmptyFile(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)

	body, contentType := multipartBody(t, "file", "empty.png", nil)

	recorder := f.uploadAs(t, user, body, contentType)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: an empty part is the client's mistake, not a server fault (body %s)",
			recorder.Code, recorder.Body.String())
	}
	if message := errorField(t, recorder); !strings.Contains(strings.ToLower(message), "empty") {
		t.Errorf("error = %q, want it to say the file is empty", message)
	}
	assertNothingStored(t, f)
}

// TestHandlerUpload_RejectsFileTooLarge exercises the declared-size guard with a
// part one byte over the cap.
func TestHandlerUpload_RejectsFileTooLarge(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)

	body, contentType := streamMultipartBody(t, "file", "huge.png", model.MaxBlobBytes+1)

	recorder := f.uploadAs(t, user, body, contentType)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
	}
	if message := errorField(t, recorder); !strings.Contains(strings.ToLower(message), "large") {
		t.Errorf("error = %q, want it to say the file is too large", message)
	}
	assertNothingStored(t, f)
}

// TestHandlerUpload_RejectsOversizedRequest covers the request cap: framing
// overhead must not let a body through that the per-file guard would refuse.
func TestHandlerUpload_RejectsOversizedRequest(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)

	body, contentType := streamMultipartBody(
		t, "file", "huge.png", model.MaxBlobRequestBytes+(1<<20),
	)

	recorder := f.uploadAs(t, user, body, contentType)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNothingStored(t, f)
}

// TestHandlerUpload_HidesStorageFailures pins the contract for a persistence
// failure: logged server side, answered with a generic 500. The storage error
// can carry backend addresses and absolute paths.
func TestHandlerUpload_HidesStorageFailures(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)
	repos := mixedRepositories{assets: &stubAssetRepo{}, blobs: &stubBlobRepo{}}
	f.h.svc = newFakeService(
		repos,
		&fakeStorage{putErr: errDB},
		&fakeTxManager{repos: repos},
		NewKeyGenerator(),
	)

	body, contentType := multipartBody(t, "file", "photo.png", pngPayload(2048))
	recorder := f.uploadAs(t, user, body, contentType)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNoLeak(t, recorder, "s3://", "10.0.0.5", "dial tcp", "etc/shadow", "storage")
	assertNothingStored(t, f)
}

// TestHandlerUpload_HidesRecordFailures: a database failure is answered the same
// way, so a driver or table name never reaches the client either.
func TestHandlerUpload_HidesRecordFailures(t *testing.T) {
	f := newHandlerFixture(t)
	user := seedUser(t, f.db, "uploader", model.RoleContributor)
	repos := mixedRepositories{
		assets: &stubAssetRepo{},
		blobs:  &stubBlobRepo{createErr: errors.New("no such table: assets")},
	}
	f.h.svc = newFakeService(
		repos,
		&fakeStorage{},
		&fakeTxManager{repos: repos},
		NewKeyGenerator(),
	)

	body, contentType := multipartBody(t, "file", "photo.png", pngPayload(2048))
	recorder := f.uploadAs(t, user, body, contentType)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNoLeak(t, recorder, "no such table", "assets", "SQL")
}

// --- Handler.Delete -----------------------------------------------------

// TestHandlerDelete_RequiresAuthentication drives the handler's own guard on the
// bare engine, where no middleware has run. The status itself is not pinned:
// mounted behind JWTAuth, an anonymous request never gets here, so this is
// defense in depth and any 4xx refuses it. What is pinned is that nothing is
// deleted. (401 remains the code that would tell a client to log in; the
// handler answers 400 here.)
func TestHandlerDelete_RequiresAuthentication(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	asset := seedAsset(t, f.db, owner.ID, "mine.png")

	recorder := perform(t, f.bare, http.MethodPost,
		"/api/asset/delete/"+asset.PublicID.String(), nil, "", nil)

	if recorder.Code < 400 || recorder.Code >= 500 {
		t.Fatalf("status = %d, want a client error (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
		t.Errorf("unauthenticated delete removed the asset: %v", err)
	}
}

func TestHandlerDelete_RequiresAuthenticationOnMountedRoute(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	asset := seedAsset(t, f.db, owner.ID, "mine.png")

	recorder := perform(t, f.guarded, http.MethodPost,
		"/api/asset/delete/"+asset.PublicID.String(), nil, "", nil)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", recorder.Code, recorder.Body.String())
	}
	if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
		t.Errorf("unauthenticated delete removed the asset: %v", err)
	}
}

// TestRoutes_DisallowGuestUpload pins the role gate in route.go. A guest is a
// valid session, so authentication passes and the permission middleware is the
// only thing refusing the request: it must answer 403 and leave no asset, no
// blob and no object behind.
func TestRoutes_DisallowGuestUpload(t *testing.T) {
	f := newHandlerFixture(t)
	guest := seedUser(t, f.db, "guest", model.RoleGuest)
	body, contentType := multipartBody(t, "file", "photo.png", pngPayload(256))

	recorder := f.uploadOnRoutedRoute(t, guest, body, contentType)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNothingStored(t, f)
}

// TestRoutes_DisallowGuestDelete covers both ownership directions, because the
// gate sits in front of the handler: a guest cannot delete an asset it owns
// either, so the 403 cannot be confused with the ownership check in
// Service.Delete.
func TestRoutes_DisallowGuestDelete(t *testing.T) {
	f := newHandlerFixture(t)
	guest := seedUser(t, f.db, "guest", model.RoleGuest)
	admin := seedUser(t, f.db, "admin", model.RoleAdmin)

	for _, tt := range []struct {
		name  string
		owner model.User
	}{
		{name: "own asset", owner: guest},
		{name: "another user's asset", owner: admin},
	} {
		t.Run(tt.name, func(t *testing.T) {
			asset := seedAsset(t, f.db, tt.owner.ID, "mine.png")

			recorder := f.deleteOnRoutedRoute(t, guest, asset.PublicID)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (body %s)", recorder.Code, recorder.Body.String())
			}
			errorField(t, recorder)
			if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
				t.Errorf("guest delete removed the asset: %v", err)
			}
		})
	}
}

// TestRoutes_AllowNonGuestRoles pins the other half: the gate names contributor
// and up, so every one of those roles still reaches the handler. An allowlist
// that accidentally omits a role would otherwise only show up in production.
func TestRoutes_AllowNonGuestRoles(t *testing.T) {
	for _, role := range []string{
		model.RoleContributor,
		model.RoleAuthor,
		model.RoleAdmin,
		model.RoleSuperAdmin,
	} {
		t.Run(role, func(t *testing.T) {
			f := newHandlerFixture(t)
			user := seedUser(t, f.db, role, role)

			body, contentType := multipartBody(t, "file", "photo.png", pngPayload(256))
			upload := f.uploadOnRoutedRoute(t, user, body, contentType)
			if upload.Code != http.StatusOK {
				t.Fatalf("upload status = %d, want 200 (body %s)", upload.Code, upload.Body.String())
			}
			created := decodeUploadResponse(t, upload)

			del := f.deleteOnRoutedRoute(t, user, created.PublicID)
			if del.Code != http.StatusOK {
				t.Fatalf("delete status = %d, want 200 (body %s)", del.Code, del.Body.String())
			}
			err := f.db.Where("public_id = ?", created.PublicID).First(&model.Asset{}).Error
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Errorf("asset %s survived its own delete: %v", created.PublicID, err)
			}
		})
	}
}

// TestRoutes_UnknownRoleIsRefused closes the gate from the other side: a role
// string nobody recognizes must not be treated as permitted.
func TestRoutes_UnknownRoleIsRefused(t *testing.T) {
	f := newHandlerFixture(t)
	stranger := seedUser(t, f.db, "stranger", "wizard")
	body, contentType := multipartBody(t, "file", "photo.png", pngPayload(256))

	recorder := f.uploadOnRoutedRoute(t, stranger, body, contentType)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body %s)", recorder.Code, recorder.Body.String())
	}
	assertNothingStored(t, f)
}

// TestHandlerDelete_OwnerSucceeds is the happy path: a soft delete, a message,
// and nothing else touched.
func TestHandlerDelete_OwnerSucceeds(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleGuest)
	asset := seedAsset(t, f.db, owner.ID, "mine.png")
	key := writeObject(t, f.db, f.dataDir, asset.BlobID, "stored-bytes")

	recorder := f.deleteAs(t, owner, asset.PublicID.String())

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	// The body is MessageResponse: a message and nothing else, since the
	// asset is gone and there is no row to describe.
	response := decodeMessageResponse(t, recorder)
	if response.Message == "" {
		t.Error("message is empty, want a human-readable confirmation")
	}
	assertKeysAre(t, recorder, "message")

	if err := f.db.First(&model.Asset{}, asset.ID).Error; err == nil {
		t.Error("the asset is still visible after a successful delete")
	}
	var unscoped model.Asset
	if err := f.db.Unscoped().First(&unscoped, asset.ID).Error; err != nil {
		t.Fatalf("the row was hard deleted: %v", err)
	}
	if !unscoped.DeletedAt.Valid {
		t.Error("DeletedAt not set on the soft deleted asset")
	}

	// The blob and its object survive: a soft delete removes the asset from the
	// catalogue, not the bytes.
	if n := countBlobs(t, f.db); n != 1 {
		t.Errorf("blob count = %d, want the blob to survive", n)
	}
	assertObject(t, f.dataDir, key, "stored-bytes")
}

// TestHandlerDelete_OwnerCanDeletePrivateAsset: the visibility gate belongs to
// the public read path, not to deletion; a private asset must not become
// undeletable.
func TestHandlerDelete_OwnerCanDeletePrivateAsset(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	asset := seedAsset(t, f.db, owner.ID, "draft.png")
	setVisibility(t, f.db, asset.ID, model.AssetVisibilityPrivate)

	recorder := f.deleteAs(t, owner, asset.PublicID.String())

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if err := f.db.First(&model.Asset{}, asset.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("private asset survived its owner's delete: %v", err)
	}
}

func TestHandlerDelete_TwiceReturnsNotFound(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	asset := seedAsset(t, f.db, owner.ID, "twice.png")
	id := asset.PublicID.String()

	if recorder := f.deleteAs(t, owner, id); recorder.Code != http.StatusOK {
		t.Fatalf("first delete status = %d, want 200 (body %s)",
			recorder.Code, recorder.Body.String())
	}

	recorder := f.deleteAs(t, owner, id)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404 (body %s)",
			recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
}

// TestHandlerDelete_NotFound uses well-formed public IDs that name nothing, so
// the handler gets as far as the lookup and the answer is a 404. Malformed
// values are covered by TestHandlerDelete_RejectsUnusablePublicIDParam; a bare
// numeric id, the pre-public-ID URL format, is one of them now.
func TestHandlerDelete_NotFound(t *testing.T) {
	f := newHandlerFixture(t)
	admin := seedUser(t, f.db, "admin", model.RoleAdmin)

	for _, publicID := range []uuid.UUID{uuid.New(), uuid.New()} {
		recorder := f.deleteAs(t, admin, publicID.String())
		if recorder.Code != http.StatusNotFound {
			t.Errorf("delete of absent asset %s: status = %d, want 404 (body %s)",
				publicID, recorder.Code, recorder.Body.String())
		}
		errorField(t, recorder)
	}
}

// TestHandlerDelete_AdminCanDeleteAnotherUsersAsset: an admin does not need to
// own the asset, and the role has to reach the service from the context.
func TestHandlerDelete_AdminCanDeleteAnotherUsersAsset(t *testing.T) {
	for _, role := range []string{model.RoleAdmin, model.RoleSuperAdmin} {
		t.Run(role, func(t *testing.T) {
			f := newHandlerFixture(t)
			owner := seedUser(t, f.db, "owner", model.RoleContributor)
			admin := seedUser(t, f.db, "admin", role)
			asset := seedAsset(t, f.db, owner.ID, "theirs.png")

			recorder := f.deleteAs(t, admin, asset.PublicID.String())

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			if err := f.db.First(&model.Asset{}, asset.ID).Error; err == nil {
				t.Error("the asset is still visible after an admin delete")
			}
		})
	}
}

// TestHandlerDelete_ForbiddenForOtherUsers: nobody but the owner and an admin
// may delete, and a rejected attempt must change nothing.
func TestHandlerDelete_ForbiddenForOtherUsers(t *testing.T) {
	for _, role := range []string{
		model.RoleGuest,
		model.RoleContributor,
		model.RoleAuthor,
		"",
		"Admin", // role matching is exact, not case insensitive
	} {
		t.Run("role="+role, func(t *testing.T) {
			f := newHandlerFixture(t)
			owner := seedUser(t, f.db, "owner", model.RoleAuthor)
			other := seedUser(t, f.db, "other", role)
			asset := seedAsset(t, f.db, owner.ID, "theirs.png")

			recorder := f.deleteAs(t, other, asset.PublicID.String())

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (body %s)", recorder.Code, recorder.Body.String())
			}
			errorField(t, recorder)
			if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
				t.Errorf("a forbidden delete removed the asset: %v", err)
			}
		})
	}
}

// TestHandlerDelete_RejectsUnusablePublicIDParam covers every shape the
// `:publicID` path param can arrive in that is not a usable public ID, as
// declared by `@Param publicID path string`. None may be coerced into one,
// because the coercion is what turns a typo into someone else's asset — and a
// bare numeric id, the pre-public-ID URL format, is one such typo.
//
// It also pins that a rejected param stops the request: writing the 400 is not
// enough, the handler must not go on to ask the service about the unusable ID
// it parsed.
func TestHandlerDelete_RejectsUnusablePublicIDParam(t *testing.T) {
	for _, publicID := range []string{
		"1", // the old numeric URL format
		"abc",
		"12abc",
		"1.5",
		"-1",
		"+1",
		"%2012", // percent-encoded: the handler must not trim the param
		"12%20",
		"0x1f",
		"1e3",
		"0",
		"99999999999999999999999",
		"0192",                                 // a UUID truncated to its timestamp prefix
		"01920000-0000-7000-8000-00000000000g", // a UUID with a non-hex character
	} {
		t.Run("publicID="+publicID, func(t *testing.T) {
			f := newHandlerFixture(t)
			admin := seedUser(t, f.db, "admin", model.RoleAdmin)
			asset := seedAsset(t, f.db, admin.ID, "mine.png")

			// A recording repository proves the service was never consulted,
			// rather than inferring it from a row that happens to survive.
			repo := &recordingAssetRepo{}
			f.h.svc = &Service{assets: repo}

			recorder := perform(t, f.bare, http.MethodPost,
				"/api/asset/delete/"+publicID, nil, "", identityOf(admin))

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			errorField(t, recorder)
			if len(repo.publicLookups) != 0 {
				t.Errorf("the handler kept going after rejecting %q: looked up %v",
					publicID, repo.publicLookups)
			}
			if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
				t.Errorf("a malformed public ID deleted an asset: %v", err)
			}
		})
	}
}

// recordingAssetRepo counts the public IDs it is asked for, so a test can tell
// "the handler refused and never called the service" apart from "the service
// was called and changed nothing".
type recordingAssetRepo struct {
	publicLookups []uuid.UUID
}

func (r *recordingAssetRepo) Create(context.Context, *model.Asset) error {
	return nil
}

func (r *recordingAssetRepo) FindByID(context.Context, uint) (*model.Asset, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *recordingAssetRepo) FindByPublicID(
	_ context.Context,
	publicID uuid.UUID,
) (*model.Asset, error) {
	r.publicLookups = append(r.publicLookups, publicID)
	return nil, gorm.ErrRecordNotFound
}

func (r *recordingAssetRepo) FindByPublicIDs(
	_ context.Context,
	publicIDs []uuid.UUID,
) ([]model.Asset, error) {
	r.publicLookups = append(r.publicLookups, publicIDs...)
	return nil, nil
}

func (r *recordingAssetRepo) FindPublicByPublicID(
	context.Context,
	uuid.UUID,
) (*model.Asset, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *recordingAssetRepo) SoftDelete(context.Context, *model.Asset) error {
	return nil
}

// TestHandlerDelete_NilPublicIDIsNotFound is the lookup edge: the nil UUID
// parses cleanly but identifies nothing, so it must not select the first asset
// in the table.
func TestHandlerDelete_NilPublicIDIsNotFound(t *testing.T) {
	f := newHandlerFixture(t)
	admin := seedUser(t, f.db, "admin", model.RoleAdmin)
	first := seedAsset(t, f.db, admin.ID, "first.png")
	seedAsset(t, f.db, admin.ID, "second.png")

	recorder := f.deleteAs(t, admin, uuid.Nil.String())

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	if err := f.db.First(&model.Asset{}, first.ID).Error; err != nil {
		t.Errorf("a nil public ID removed an unrelated asset: %v", err)
	}
	if n := countAssets(t, f.db); n != 2 {
		t.Errorf("visible asset count = %d, want both assets intact", n)
	}
}

// TestHandlerDelete_UnknownRoleCannotDeleteAnotherUsersAsset: a context that
// carries the user id but no role must fail closed, never assume admin. Only
// the refusal is pinned, not which 4xx says it: the point is that a caller
// without a role in hand cannot reach someone else's asset.
func TestHandlerDelete_UnknownRoleCannotDeleteAnotherUsersAsset(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	stranger := seedUser(t, f.db, "stranger", model.RoleAdmin)
	asset := seedAsset(t, f.db, owner.ID, "theirs.png")

	repo := &recordingAssetRepo{}
	f.h.svc = &Service{assets: repo}

	recorder := perform(t, f.bare, http.MethodPost,
		"/api/asset/delete/"+asset.PublicID.String(), nil, "", map[string]string{
			"X-Test-User-ID": strconv.FormatUint(uint64(stranger.ID), 10),
			"X-Test-No-Role": "1",
		})

	if recorder.Code < 400 || recorder.Code >= 500 {
		t.Fatalf("status = %d, want a client error (body %s)", recorder.Code, recorder.Body.String())
	}
	if len(repo.publicLookups) != 0 {
		t.Errorf("a roleless caller reached the service with %v", repo.publicLookups)
	}
	if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
		t.Errorf("a roleless delete removed someone else's asset: %v", err)
	}
}

// TestHandlerDelete_IgnoresRoleClaimWhenDatabaseDisagrees: the token is signed,
// but the role in it is only a claim. A demoted admin whose token still says
// admin must not keep deleting other people's assets.
func TestHandlerDelete_IgnoresRoleClaimWhenDatabaseDisagrees(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	// Seeded as a contributor, so the middleware reads contributor from the
	// database even though the token claims admin.
	demoted := seedUser(t, f.db, "demoted", model.RoleContributor)
	asset := seedAsset(t, f.db, owner.ID, "theirs.png")

	recorder := perform(t, f.guarded, http.MethodPost,
		"/api/asset/delete/"+asset.PublicID.String(), nil, "", map[string]string{
			"Authorization": "Bearer " + mintToken(t, demoted.ID, model.RoleAdmin),
		})

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body %s)", recorder.Code, recorder.Body.String())
	}
	if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
		t.Errorf("a demoted admin deleted an asset they do not own: %v", err)
	}
}

// TestHandlerDelete_AllowsAdminOnMountedRoute: the same request succeeds once
// the database really does say admin.
func TestHandlerDelete_AllowsAdminOnMountedRoute(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	admin := seedUser(t, f.db, "admin", model.RoleAdmin)
	asset := seedAsset(t, f.db, owner.ID, "theirs.png")

	recorder := perform(t, f.guarded, http.MethodPost,
		"/api/asset/delete/"+asset.PublicID.String(), nil, "", map[string]string{
			"Authorization": "Bearer " + mintToken(t, admin.ID, model.RoleAdmin),
		})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if err := f.db.First(&model.Asset{}, asset.ID).Error; err == nil {
		t.Error("the asset is still visible after an admin delete")
	}
}

// TestHandlerDelete_HidesRepositoryFailures: an infrastructure failure is a 500,
// not a 404. Reporting "not found" for a database outage tells the client to
// give up on an asset that is still there.
func TestHandlerDelete_HidesRepositoryFailures(t *testing.T) {
	f := newHandlerFixture(t)
	owner := seedUser(t, f.db, "owner", model.RoleContributor)
	asset := seedAsset(t, f.db, owner.ID, "mine.png")
	f.h.svc = &Service{assets: &stubAssetRepo{findErr: errors.New("dial tcp 10.0.0.5:5432: refused")}}

	recorder := f.deleteAs(t, owner, asset.PublicID.String())

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", recorder.Code, recorder.Body.String())
	}
	errorField(t, recorder)
	assertNoLeak(t, recorder, "dial tcp", "10.0.0.5", "5432", "refused", "record not found")
	if err := f.db.First(&model.Asset{}, asset.ID).Error; err != nil {
		t.Errorf("a failed delete removed the asset: %v", err)
	}
}
