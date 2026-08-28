package post

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// testJWTSecret is the shared secret used to mint tokens for the route tests.
// It must match the secret passed to the handler's middleware.
var testJWTSecret = []byte("issue20-route-test-secret")

// newTestRouter builds a fresh gin engine with the real post routes registered
// and a seeded user database. It returns the engine and the underlying DB so
// callers can inspect persisted rows.
func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
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
	if err := db.AutoMigrate(&model.User{}, &model.Category{}, &model.Tag{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	h := NewHandler(Deps{DB: db, JWTSecret: testJWTSecret})
	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return r, db
}

// mintToken signs a JWT for the given user using the route-test secret. The
// claims mirror what JWTAuth expects: user_id, username, role, a
// password_version matching the seeded user, and an iat that is newer than the
// user's (zero) LastLoginAt so the token is accepted.
func mintToken(t *testing.T, userID uint, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":          float64(userID),
		"username":         "tester",
		"role":             role,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(testJWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func seedRoleUser(t *testing.T, db *gorm.DB, role string) model.User {
	t.Helper()
	u := model.User{Username: "u_" + role, Email: "u_" + role + "@example.com", Role: role, PasswordVersion: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func doPostCategory(t *testing.T, r *gin.Engine, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"name":"tech","description":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/categories", body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doPostTag(t *testing.T, r *gin.Engine, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"name":"golang"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tags", body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestCreateCategory_RoleGating exercises the AC1 acceptance criteria against
// the real middleware+handler chain. guest and unknown roles are expected to be
// rejected with 403, the four contributor-level roles with 201, and anonymous
// requests with 401. Until the routes are gated (issue step 1) the guest /
// unknown / anonymous expectations fail.
func TestCreateCategory_RoleGating(t *testing.T) {
	cases := []struct {
		name     string
		role     string // "" means no token at all
		wantCode int
	}{
		{"anonymous", "", http.StatusUnauthorized}, // 401 (already enforced by JWTAuth)
		{"guest", model.RoleGuest, http.StatusForbidden},
		{"contributor", model.RoleContributor, http.StatusCreated},
		{"author", model.RoleAuthor, http.StatusCreated},
		{"admin", model.RoleAdmin, http.StatusCreated},
		{"super_admin", model.RoleSuperAdmin, http.StatusCreated},
		{"unknown_role", "wizard", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, db := newTestRouter(t)
			var token string
			if tc.role != "" {
				u := seedRoleUser(t, db, tc.role)
				token = mintToken(t, u.ID, tc.role)
			}
			w := doPostCategory(t, r, token)
			if w.Code != tc.wantCode {
				t.Errorf("POST /api/categories role=%q: expected %d, got %d (body=%s)",
					tc.role, tc.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

// TestCreateTag_RoleGating mirrors TestCreateCategory_RoleGating for tags.
func TestCreateTag_RoleGating(t *testing.T) {
	cases := []struct {
		name     string
		role     string
		wantCode int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"guest", model.RoleGuest, http.StatusForbidden},
		{"contributor", model.RoleContributor, http.StatusCreated},
		{"author", model.RoleAuthor, http.StatusCreated},
		{"admin", model.RoleAdmin, http.StatusCreated},
		{"super_admin", model.RoleSuperAdmin, http.StatusCreated},
		{"unknown_role", "wizard", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, db := newTestRouter(t)
			var token string
			if tc.role != "" {
				u := seedRoleUser(t, db, tc.role)
				token = mintToken(t, u.ID, tc.role)
			}
			w := doPostTag(t, r, token)
			if w.Code != tc.wantCode {
				t.Errorf("POST /api/tags role=%q: expected %d, got %d (body=%s)",
					tc.role, tc.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

// TestCreateCategory_DuplicateNameMapping asserts that creating a category with
// a name that already exists returns a clear 4xx rather than 500. The exact
// status (400 validation or 409 conflict) is left open, but a 500 would be a
// silent server error and 201 would mean the duplicate was allowed. Requires
// the duplicate handling from issue step 3, which is not implemented yet, so
// this is expected to be red.
func TestCreateCategory_DuplicateNameMapping(t *testing.T) {
	r, db := newTestRouter(t)
	u := seedRoleUser(t, db, model.RoleContributor)
	token := mintToken(t, u.ID, model.RoleContributor)

	// First creation succeeds (or at least does not error fatally).
	first := doPostCategory(t, r, token)
	if first.Code != http.StatusCreated {
		t.Fatalf("first category creation expected 201, got %d (body=%s)", first.Code, first.Body.String())
	}

	// Second creation with the same name must surface a clear 4xx, not 500 and
	// not a second 201.
	dup := doPostCategory(t, r, token)
	if dup.Code == http.StatusCreated {
		t.Errorf("duplicate category name: expected rejection, got 201 (body=%s)", dup.Body.String())
	}
	if dup.Code < 400 || dup.Code >= 500 {
		t.Errorf("duplicate category name: expected 4xx, got %d (body=%s)", dup.Code, dup.Body.String())
	}
}

// TestCreateCategory_BlankNameRejected verifies that blank category names are
// rejected with ErrBadRequest and that surrounding whitespace is trimmed
// before the name is stored, so " tech " and "tech" cannot coexist.
func TestCreateCategory_BlankNameRejected(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	for _, name := range []string{"", "   ", "\t\n"} {
		if _, err := svc.CreateCategory(ctx, model.RoleContributor, name, ""); !errors.Is(err, ErrBadRequest) {
			t.Errorf("blank category name %q: expected ErrBadRequest, got %v", name, err)
		}
	}

	cat, err := svc.CreateCategory(ctx, model.RoleContributor, "  tech  ", "")
	if err != nil {
		t.Fatalf("trimmed name should be accepted, got error: %v", err)
	}
	if cat.Name != "tech" {
		t.Errorf("category name should be stored trimmed, got %q", cat.Name)
	}
}

// TestCreateTag_BlankNameRejected mirrors TestCreateCategory_BlankNameRejected
// for tags: blank names are rejected and whitespace is trimmed, matching the
// normalization resolveTags already applies when saving posts.
func TestCreateTag_BlankNameRejected(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	for _, name := range []string{"", "   ", "\t\n"} {
		if _, err := svc.CreateTag(ctx, model.RoleContributor, name); !errors.Is(err, ErrBadRequest) {
			t.Errorf("blank tag name %q: expected ErrBadRequest, got %v", name, err)
		}
	}

	tag, err := svc.CreateTag(ctx, model.RoleContributor, "  golang  ")
	if err != nil {
		t.Fatalf("trimmed name should be accepted, got error: %v", err)
	}
	if tag.Name != "golang" {
		t.Errorf("tag name should be stored trimmed, got %q", tag.Name)
	}
}

// TestCreateCategory_BlankNameBadRequest verifies the HTTP mapping: a
// whitespace-only name passes the binding:required check (it is non-empty) but
// must surface as 400 via the service-level ErrBadRequest, not 500.
func TestCreateCategory_BlankNameBadRequest(t *testing.T) {
	r, db := newTestRouter(t)
	u := seedRoleUser(t, db, model.RoleContributor)
	token := mintToken(t, u.ID, model.RoleContributor)

	req := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(`{"name":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("whitespace-only category name: expected 400, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestCreateCategory_ServiceRoleCheck verifies AC2 at the service layer: the
// service must reject non-contributor roles (fail closed) even when no
// middleware is involved. model.IsContributor covers contributor/author/admin/
// super_admin; guest and an empty role must be rejected. This currently passes
// because the service already calls model.IsContributor, but pins the contract.
func TestCreateCategory_ServiceRoleCheck(t *testing.T) {
	for _, role := range []string{
		model.RoleGuest,
		"",
		model.RoleContributor,
		model.RoleAuthor,
		model.RoleAdmin,
		model.RoleSuperAdmin,
	} {
		t.Run(role, func(t *testing.T) {
			svc, _, _, _ := newTestService(t)
			ctx := context.Background()
			_, err := svc.CreateCategory(ctx, role, "cat_"+role, "desc")
			if model.IsContributor(role) {
				if err != nil {
					t.Errorf("role %q should be allowed, got error: %v", role, err)
				}
				return
			}
			if err == nil {
				t.Errorf("role %q should be rejected (fail closed), got nil error", role)
			}
		})
	}
}

// TestCreateTag_ServiceRoleCheck mirrors TestCreateCategory_ServiceRoleCheck for
// tags (AC2, service layer).
func TestCreateTag_ServiceRoleCheck(t *testing.T) {
	for _, role := range []string{
		model.RoleGuest,
		"",
		model.RoleContributor,
		model.RoleAuthor,
		model.RoleAdmin,
		model.RoleSuperAdmin,
	} {
		t.Run(role, func(t *testing.T) {
			svc, _, _, _ := newTestService(t)
			ctx := context.Background()
			_, err := svc.CreateTag(ctx, role, "tag_"+role)
			if model.IsContributor(role) {
				if err != nil {
					t.Errorf("role %q should be allowed, got error: %v", role, err)
				}
				return
			}
			if err == nil {
				t.Errorf("role %q should be rejected (fail closed), got nil error", role)
			}
		})
	}
}

// TestCreateCategory_HandlerFailClosedNoMiddleware verifies AC2 at the HTTP
// layer when the Permission middleware is NOT present: the handler must still
// map a non-contributor service rejection to 403. This is currently RED because
// the service returns the wrong sentinel (ErrGuestViewDenied) and the handler
// maps every error to 500, so an authenticated guest receives 500 instead of
// 403.
func TestCreateCategory_HandlerFailClosedNoMiddleware(t *testing.T) {
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
	if err := db.AutoMigrate(&model.User{}, &model.Category{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	guest := seedRoleUser(t, db, model.RoleGuest)
	token := mintToken(t, guest.ID, model.RoleGuest)

	// Register the route with ONLY JWTAuth (no Permission middleware), so the
	// handler must enforce the role check itself.
	h := NewHandler(Deps{DB: db, JWTSecret: testJWTSecret})
	r := gin.New()
	r.POST("/api/categories", h.mw.JWTAuth(), h.CreateCategory)

	body := strings.NewReader(`{"name":"tech","description":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/categories", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("authenticated guest without Permission middleware: expected 403, got %d (body=%s)",
			w.Code, w.Body.String())
	}
}
