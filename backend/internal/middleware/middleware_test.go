package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var testSecret = []byte("middleware-test-secret")

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func signToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(testSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

// capture records what the middleware wrote into the gin context.
type capture struct {
	userID   any
	username any
	user     any
	passed   bool
	body     string
}

// runAuth executes a single middleware against a request and returns the
// HTTP status plus the context values the middleware set.
func runAuth(t *testing.T, mw gin.HandlerFunc, header string) (int, capture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var cap capture
	r.GET("/protected", mw, func(c *gin.Context) {
		cap.passed = true
		cap.userID, _ = c.Get("userID")
		cap.username, _ = c.Get("username")
		cap.user, _ = c.Get("user")
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	r.ServeHTTP(w, req)
	cap.body = w.Body.String()
	return w.Code, cap
}

func TestCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user", map[string]any{"id": uint(7), "username": "alice", "role": model.RoleAdmin})

	u, ok := CurrentUser(c)
	if !ok || u.ID != 7 || u.Username != "alice" || u.Role != model.RoleAdmin {
		t.Errorf("expected parsed user, got %+v ok=%v", u, ok)
	}

	// no user key
	empty, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := CurrentUser(empty); ok {
		t.Errorf("expected ok=false when no user in context")
	}

	// wrong type
	wrong, _ := gin.CreateTestContext(httptest.NewRecorder())
	wrong.Set("user", "not-a-map")
	if _, ok := CurrentUser(wrong); ok {
		t.Errorf("expected ok=false when user is not a map")
	}

	// missing key
	missing, _ := gin.CreateTestContext(httptest.NewRecorder())
	missing.Set("user", map[string]any{"id": uint(7), "username": "alice"})
	if _, ok := CurrentUser(missing); ok {
		t.Errorf("expected ok=false when a key is missing")
	}

	// wrong-typed key
	badType, _ := gin.CreateTestContext(httptest.NewRecorder())
	badType.Set("user", map[string]any{"id": "7", "username": "alice", "role": model.RoleAdmin})
	if _, ok := CurrentUser(badType); ok {
		t.Errorf("expected ok=false when a key has the wrong type")
	}
}

func TestCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		val  any
		want uint
	}{
		{"uint", uint(42), 42},
		{"int", int(42), 42},
		{"float64", float64(42), 42},
		{"missing", nil, 0},
		{"wrong-type", "nope", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.val != nil {
				c.Set("userID", tc.val)
			}
			if got := CurrentUserID(c); got != tc.want {
				t.Errorf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestJWTAuth_NoHeader(t *testing.T) {
	a := NewAuth(nil, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestJWTAuth_BadFormat(t *testing.T) {
	a := NewAuth(nil, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "Basic abc")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	a := NewAuth(nil, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "Bearer not-a-jwt")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestJWTAuth_MissingUserIDClaim(t *testing.T) {
	// A signed token without a user_id claim must be rejected with 401,
	// not panic on a bare type assertion.
	tok := signToken(t, jwt.MapClaims{
		"username": "bob",
		"role":     model.RoleGuest,
	})
	a := NewAuth(nil, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestOptionalJWTAuth_MissingUserIDClaim(t *testing.T) {
	// A signed token without a user_id claim must not panic; the request
	// passes through and no user is written to the context.
	tok := signToken(t, jwt.MapClaims{
		"username": "bob",
		"role":     model.RoleGuest,
	})
	a := NewAuth(nil, testSecret)
	code, cap := runAuth(t, a.OptionalJWTAuth(), "Bearer "+tok)
	if code != http.StatusOK || !cap.passed {
		t.Fatalf("expected request to pass through, code=%d", code)
	}
	if cap.userID != nil {
		t.Errorf("expected no userID set, got %v", cap.userID)
	}
	userMap, ok := cap.user.(map[string]any)
	if !ok {
		t.Fatalf("expected user map in context, got %T", cap.user)
	}
	if userMap["id"] != uint(0) {
		t.Errorf("expected zero user id, got %v", userMap["id"])
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	db := newTestDB(t)
	// PasswordVersion carries gorm:"default:1", so a freshly created user
	// stores version 1; the token must carry the same version to be valid.
	u := model.User{Username: "alice", Email: "alice@example.com", Role: model.RoleAuthor, PasswordVersion: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(u.ID),
		"username":         "alice",
		"role":             model.RoleGuest, // db role should win
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(db, testSecret)
	code, cap := runAuth(t, a.JWTAuth(), "Bearer "+tok)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", code, cap.body)
	}
	if !cap.passed {
		t.Error("expected handler to run")
	}
	if cap.userID != u.ID {
		t.Errorf("expected userID %d, got %v", u.ID, cap.userID)
	}
	userMap, ok := cap.user.(map[string]any)
	if !ok {
		t.Fatalf("expected user map in context, got %T", cap.user)
	}
	// role is refreshed from the database, not the token claim.
	if userMap["role"] != model.RoleAuthor {
		t.Errorf("expected db role author, got %v", userMap["role"])
	}
}

func TestJWTAuth_PasswordChanged(t *testing.T) {
	db := newTestDB(t)
	u := model.User{Username: "alice", Email: "alice@example.com", Role: model.RoleAuthor, PasswordVersion: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(u.ID),
		"username":         "alice",
		"role":             model.RoleAuthor,
		"password_version": float64(0), // stale
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(db, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestJWTAuth_TokenBeforeLastLogin(t *testing.T) {
	db := newTestDB(t)
	u := model.User{Username: "alice", Email: "alice@example.com", Role: model.RoleAuthor}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// LastLoginAt in the future → any token issued now is stale.
	if err := db.Model(&u).Update("last_login_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("set last login: %v", err)
	}
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(u.ID),
		"username":         "alice",
		"role":             model.RoleAuthor,
		"password_version": float64(0),
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(db, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestJWTAuth_DeletedUser(t *testing.T) {
	db := newTestDB(t)
	// Token signed for a user id that does not exist in the DB: the token
	// must be rejected instead of falling back to the role claim.
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(99999),
		"username":         "ghost",
		"role":             model.RoleAdmin,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(db, testSecret)
	code, _ := runAuth(t, a.JWTAuth(), "Bearer "+tok)
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401 for deleted user, got %d", code)
	}
}

func TestOptionalJWTAuth_DeletedUser(t *testing.T) {
	db := newTestDB(t)
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(99999),
		"username":         "ghost",
		"role":             model.RoleAdmin,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(db, testSecret)
	code, cap := runAuth(t, a.OptionalJWTAuth(), "Bearer "+tok)
	if code != http.StatusOK || !cap.passed {
		t.Fatalf("expected request to pass through, code=%d", code)
	}
	if cap.userID != nil {
		t.Errorf("expected anonymous for deleted user, got userID %v", cap.userID)
	}
}

func TestPermission_UsesContextRole(t *testing.T) {
	// The role comes from the context (as set by JWTAuth), so the DB is not
	// touched: a nil db must still allow an authorized admin through.
	a := NewAuth(nil, testSecret)
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set("userID", uint(7))
		c.Set("user", map[string]any{"id": uint(7), "username": "admin", "role": model.RoleAdmin})
		c.Next()
	}, a.Permission(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 using context role, got %d", w.Code)
	}
}

func TestJWTAuth_NilDBUsesTokenRole(t *testing.T) {
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(3),
		"username":         "bob",
		"role":             model.RoleGuest,
		"password_version": float64(0),
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(nil, testSecret)
	code, cap := runAuth(t, a.JWTAuth(), "Bearer "+tok)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	userMap, ok := cap.user.(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %T", cap.user)
	}
	if userMap["role"] != model.RoleGuest {
		t.Errorf("expected token role guest, got %v", userMap["role"])
	}
}

func TestOptionalJWTAuth_NoHeader(t *testing.T) {
	a := NewAuth(nil, testSecret)
	code, cap := runAuth(t, a.OptionalJWTAuth(), "")
	if code != http.StatusOK || !cap.passed {
		t.Errorf("expected request to pass through, code=%d", code)
	}
	if cap.userID != nil {
		t.Errorf("expected no userID set, got %v", cap.userID)
	}
}

func TestOptionalJWTAuth_InvalidToken(t *testing.T) {
	a := NewAuth(nil, testSecret)
	code, cap := runAuth(t, a.OptionalJWTAuth(), "Bearer garbage")
	if code != http.StatusOK || !cap.passed {
		t.Errorf("expected request to pass through, code=%d", code)
	}
	if cap.userID != nil {
		t.Errorf("expected no userID set for invalid token, got %v", cap.userID)
	}
}

func TestOptionalJWTAuth_ValidToken(t *testing.T) {
	db := newTestDB(t)
	u := model.User{Username: "alice", Email: "alice@example.com", Role: model.RoleAuthor, PasswordVersion: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	tok := signToken(t, jwt.MapClaims{
		"user_id":          float64(u.ID),
		"username":         "alice",
		"role":             model.RoleGuest,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	})

	a := NewAuth(db, testSecret)
	code, cap := runAuth(t, a.OptionalJWTAuth(), "Bearer "+tok)
	if code != http.StatusOK || !cap.passed {
		t.Fatalf("expected request to pass through, code=%d", code)
	}
	if cap.userID != u.ID {
		t.Errorf("expected userID %d, got %v", u.ID, cap.userID)
	}
}

func TestPermission_NoUserID(t *testing.T) {
	a := NewAuth(nil, testSecret)
	code, _ := runAuth(t, a.Permission(model.RoleAdmin), "")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestPermission_UserNotFound(t *testing.T) {
	db := newTestDB(t)
	a := NewAuth(db, testSecret)
	// userID set in context but no matching user row.
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set("userID", uint(99999))
		c.Next()
	}, a.Permission(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPermission_Forbidden(t *testing.T) {
	db := newTestDB(t)
	u := model.User{Username: "guest", Email: "g@example.com", Role: model.RoleGuest}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a := NewAuth(db, testSecret)
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set("userID", u.ID)
		c.Next()
	}, a.Permission(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestPermission_Allowed(t *testing.T) {
	db := newTestDB(t)
	u := model.User{Username: "admin", Email: "a@example.com", Role: model.RoleAdmin}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a := NewAuth(db, testSecret)
	r := gin.New()
	var userSet any
	r.GET("/admin", func(c *gin.Context) {
		c.Set("userID", u.ID)
		c.Next()
	}, a.Permission(model.RoleAdmin), func(c *gin.Context) {
		userSet, _ = c.Get("user")
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if userSet == nil {
		t.Error("expected user written to context")
	}
}

func TestPermission_SuperAdminAlwaysAllowed(t *testing.T) {
	db := newTestDB(t)
	u := model.User{Username: "root", Email: "r@example.com", Role: model.RoleSuperAdmin}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a := NewAuth(db, testSecret)
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set("userID", u.ID)
		c.Next()
	}, a.Permission(model.RoleContributor), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if w.Code != http.StatusOK {
		t.Errorf("expected super admin allowed, got %d", w.Code)
	}
}

func TestRequestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	oldOut := logrus.StandardLogger().Out
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	defer logrus.SetOutput(oldOut)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(buf.String(), "Request completed successfully") {
		t.Errorf("expected success log line, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "method=GET") {
		t.Errorf("expected method field in log, got: %s", buf.String())
	}
}
