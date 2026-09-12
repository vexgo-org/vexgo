package public

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// previewTestSecret signs the tokens used by the draft-preview tests. It must
// match the secret handed to the renderer.
var previewTestSecret = []byte("public-preview-test-secret")

// previewToken signs a user token with the claims middleware.TokenUser checks.
func previewToken(t *testing.T, userID uint, role string, passwordVersion int, issuedAt time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":          float64(userID),
		"username":         "previewer",
		"role":             role,
		"password_version": float64(passwordVersion),
		"iat":              float64(issuedAt.Unix()),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(previewTestSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func doAuthedGet(t *testing.T, r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestDraftPreview_RequiresCurrentAdminRole is the regression guard for preview
// authorization: the admin decision must come from the stored account, not from
// the token's role claim. A token that still says "role: admin" for a demoted
// account — or one invalidated by a password change or naming a deleted user —
// must not unlock an unpublished page.
func TestDraftPreview_RequiresCurrentAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Page{},
		&model.Post{},
		&model.GeneralSettings{},
		&model.ThemeConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	admin := model.User{Username: "admin", Email: "admin@example.com", Role: model.RoleAdmin, PasswordVersion: 1}
	demoted := model.User{Username: "exadmin", Email: "exadmin@example.com", Role: model.RoleAuthor, PasswordVersion: 1}
	for _, u := range []*model.User{&admin, &demoted} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	pages := []model.Page{
		{Slug: "about", Title: "About", Content: "public", Status: model.PageStatusPublished, AuthorID: admin.ID},
		{Slug: "secret", Title: "Secret", Content: "draft", Status: model.PageStatusDraft, AuthorID: admin.ID},
	}
	if err := db.Create(&pages).Error; err != nil {
		t.Fatalf("seed pages: %v", err)
	}

	renderer := NewRenderer(db, "http://localhost", t.TempDir())
	renderer.SetJWTSecret(previewTestSecret)
	e := gin.New()
	renderer.RegisterStaticRoutes(e, true)

	now := time.Now()
	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"anonymous preview is denied", "", http.StatusNotFound},
		{"admin token unlocks the draft", previewToken(t, admin.ID, model.RoleAdmin, 1, now), http.StatusOK},
		{"demoted admin cannot use its stale claim", previewToken(t, demoted.ID, model.RoleAdmin, 1, now), http.StatusNotFound},
		{"stale password version is denied", previewToken(t, admin.ID, model.RoleAdmin, 0, now), http.StatusNotFound},
		{"deleted user is denied", previewToken(t, 99999, model.RoleAdmin, 1, now), http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doAuthedGet(t, e, "/secret?preview=1", tc.token)
			if w.Code != tc.want {
				t.Fatalf("GET /secret?preview=1 status = %d, want %d", w.Code, tc.want)
			}
		})
	}

	// The published page stays reachable without any preview privileges.
	if w := doAuthedGet(t, e, "/about", ""); w.Code != http.StatusOK {
		t.Fatalf("GET /about status = %d, want 200", w.Code)
	}
}
