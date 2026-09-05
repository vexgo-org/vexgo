package settings

import (
	"archive/zip"
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// handlerTestJWTSecret is the shared secret used to mint admin tokens for the
// handler-level route tests. It must match the secret passed to the handler's
// middleware.
var handlerTestJWTSecret = []byte("settings-encryption-route-test-secret")

// newTestAdminRouter builds a fresh gin engine with the real settings config
// routes registered and a cipher wired, plus the underlying DB so callers can
// inspect persisted rows.
func newTestAdminRouter(t *testing.T) (*gin.Engine, *gorm.DB, *secrets.Cipher) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{},
		&model.SMTPConfig{},
		&model.GeneralSettings{},
		&model.AIConfig{},
		&model.ThemeConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{
		Username:        "admin",
		Email:           "admin@example.com",
		Role:            model.RoleSuperAdmin,
		PasswordVersion: 1,
	}).Error; err != nil {
		t.Fatalf("seed admin user: %v", err)
	}

	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	deps := Deps{
		DB:        db,
		JWTSecret: handlerTestJWTSecret,
		Themes:    public.NewRenderer(db, "http://localhost", t.TempDir()),
		Mailer:    mailer.NewService(mailer.Deps{DB: db}),
		Cipher:    cipher,
	}
	r := gin.New()
	jwtAuth := middleware.NewAuth(db, handlerTestJWTSecret)
	g := r.Group("/api", jwtAuth.OptionalJWTAuth())
	api := humagin.NewWithGroup(r, g, huma.DefaultConfig("VexGo API", "0.1.0"))
	api.UseMiddleware(auth.ContextMiddleware)
	NewHandler(deps).RegisterRoutes(api)
	return r, db, cipher
}

// mintAdminToken signs a JWT for the seeded admin user, mirroring the claims
// JWTAuth expects (see post/routes_test.go for the same pattern).
func mintAdminToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":          float64(1),
		"username":         "admin",
		"role":             model.RoleSuperAdmin,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(handlerTestJWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func doSMTPRequest(t *testing.T, r *gin.Engine, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/config/smtp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// smtpSaveBody builds a PUT /api/config/smtp payload for the given host and
// password (empty password means "keep the stored value").
func smtpSaveBody(host, password string) string {
	return fmt.Sprintf(
		`{"enabled":true,"host":%q,"port":587,"username":"user","password":%q,"fromEmail":"a@example.com"}`,
		host, password,
	)
}

// TC-ENC-025: through the admin API, a saved SMTP password is stored as
// ciphertext in the raw DB row and GET returns an empty password.
func TestSMTPRoutes_StoresEncryptedPasswordAndMasksResponses(t *testing.T) {
	r, db, cipher := newTestAdminRouter(t)
	token := mintAdminToken(t)

	w := doSMTPRequest(t, r, token, smtpSaveBody("smtp.example.com", "plain-secret"))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /config/smtp status = %d, body: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "plain-secret") {
		t.Errorf("PUT response must not contain the plaintext password, got: %s", w.Body.String())
	}

	var stored model.SMTPConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	if !secrets.IsEncrypted(stored.Password) {
		t.Errorf("expected stored password to carry the encrypted marker, got %q", stored.Password)
	}
	decrypted, err := cipher.Decrypt(stored.Password)
	if err != nil || decrypted != "plain-secret" {
		t.Errorf("stored ciphertext round trip failed: got %q, err %v", decrypted, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/smtp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /config/smtp status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "plain-secret") {
		t.Errorf("GET response must not contain the plaintext password, got: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"password":"`+stored.Password) {
		t.Errorf("GET response must mask the password, got: %s", w.Body.String())
	}
}

// TC-ENC-026: an empty password in a PUT keeps the stored value, which stays
// decryptable in the raw DB row.
func TestSMTPRoutes_EmptyPasswordKeepsStoredValueDecryptable(t *testing.T) {
	r, db, cipher := newTestAdminRouter(t)
	token := mintAdminToken(t)

	if w := doSMTPRequest(t, r, token, smtpSaveBody("smtp.example.com", "plain-secret")); w.Code != http.StatusOK {
		t.Fatalf("first PUT status = %d, body: %s", w.Code, w.Body.String())
	}

	if w := doSMTPRequest(t, r, token, smtpSaveBody("smtp2.example.com", "")); w.Code != http.StatusOK {
		t.Fatalf("second PUT status = %d, body: %s", w.Code, w.Body.String())
	}

	var stored model.SMTPConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	decrypted, err := cipher.Decrypt(stored.Password)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "plain-secret" {
		t.Errorf("expected preserved password to stay decryptable, got %q", decrypted)
	}
}

// The SMTP test-send route still works end to end with an encrypted stored
// password: the send is attempted (and captured) rather than failing on the
// decryption step.
func TestSMTPRoutes_TestSendWorksWithEncryptedPassword(t *testing.T) {
	r, db, cipher := newTestAdminRouter(t)
	token := mintAdminToken(t)

	encrypted, err := cipher.Encrypt("plain-secret")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if err := db.Create(&model.SMTPConfig{
		Enabled:   true,
		Host:      "127.0.0.1",
		Port:      2525,
		Username:  "user",
		Password:  encrypted,
		FromEmail: "a@example.com",
		TestEmail: "to@example.com",
	}).Error; err != nil {
		t.Fatalf("seed smtp config: %v", err)
	}

	capture := &struct{ to, subject string }{}
	mailer.SetMailCaptureHook(func(to, subject, _, _ string) {
		capture.to, capture.subject = to, subject
	})
	t.Cleanup(func() { mailer.SetMailCaptureHook(nil) })

	req := httptest.NewRequest(http.MethodPost, "/api/config/smtp/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// The mailer service reads the row itself and decrypts with the wired
	// cipher; a 200 with a captured email means the send path (including
	// decryption) succeeded.
	if w.Code != http.StatusOK {
		t.Fatalf("POST /config/smtp/test status = %d, body: %s", w.Code, w.Body.String())
	}
	if capture.to != "to@example.com" {
		t.Errorf("expected a captured test email to %q, got %q", "to@example.com", capture.to)
	}
}

// buildThemeZip packs the given entries (name → content) into a zip archive.
func buildThemeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %q: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// doThemeUpload POSTs a zip to the admin theme upload endpoint.
func doThemeUpload(t *testing.T, r *gin.Engine, token string, zipBytes []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("theme", "theme.zip")
	if err != nil {
		t.Fatalf("multipart form file: %v", err)
	}
	if _, err := fw.Write(zipBytes); err != nil {
		t.Fatalf("multipart write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/themes/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestUploadTheme_RejectsZipSlipEntries ensures hostile zip entry names —
// relative traversal, absolute paths and Windows-style traversal — are
// rejected with 400 instead of being extracted.
func TestUploadTheme_RejectsZipSlipEntries(t *testing.T) {
	r, _, _ := newTestAdminRouter(t)
	token := mintAdminToken(t)

	for name, entry := range map[string]string{
		"parent traversal":     "../evil.txt",
		"nested traversal":     "assets/../../evil.txt",
		"absolute path":        "/tmp/vexgo-evil.txt",
		"windows traversal":    `..\..\evil.txt`,
		"windows drive letter": `C:\vexgo-evil.txt`,
	} {
		t.Run(name, func(t *testing.T) {
			zipBytes := buildThemeZip(t, map[string]string{entry: "evil"})
			w := doThemeUpload(t, r, token, zipBytes)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for entry %q, got %d: %s", entry, w.Code, w.Body.String())
			}
		})
	}
}

// TestUploadTheme_AcceptsWellFormedZip exercises the happy path: a zip with a
// valid vexgo-theme.json and nested assets installs successfully.
func TestUploadTheme_AcceptsWellFormedZip(t *testing.T) {
	r, _, _ := newTestAdminRouter(t)
	token := mintAdminToken(t)

	zipBytes := buildThemeZip(t, map[string]string{
		"vexgo-theme.json":    `{"id": "testtheme", "name": "Test Theme"}`,
		"assets/style.css":    "body {}",
		"templates/home.html": "<html></html>",
	})

	w := doThemeUpload(t, r, token, zipBytes)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
