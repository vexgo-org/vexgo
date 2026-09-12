package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// doSettingsGet issues a GET against the settings API with an optional bearer
// token.
func doSettingsGet(t *testing.T, r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// mintTokenFor signs a token for an arbitrary seeded user, so the role boundary
// can be exercised (mintAdminToken is pinned to the seeded super admin).
func mintTokenFor(t *testing.T, userID uint, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":          float64(userID),
		"username":         "user",
		"role":             role,
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

// TestThemePreviewLinkRoutes covers the admin-only endpoint the console uses to
// open a theme preview in a new tab: anonymous is 401, a signed-in non-admin is
// 403, an unknown theme is 404, and an admin receives a signed, theme-bound
// link that carries no credential of its own.
func TestThemePreviewLinkRoutes(t *testing.T) {
	r, db, _ := newTestAdminRouter(t)

	guest := model.User{Username: "guest", Email: "guest@example.com", Role: model.RoleGuest, PasswordVersion: 1}
	if err := db.Create(&guest).Error; err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"guest", mintTokenFor(t, guest.ID, model.RoleGuest), http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := doSettingsGet(t, r, "/api/config/themes/"+public.DefaultTheme+"/preview-link", tc.token)
			if w.Code != tc.want {
				t.Fatalf("status = %d (body %s), want %d", w.Code, w.Body.String(), tc.want)
			}
		})
	}

	// A theme that does not exist must not yield a working link.
	if w := doSettingsGet(t, r, "/api/config/themes/nope/preview-link", mintAdminToken(t)); w.Code != http.StatusNotFound {
		t.Errorf("unknown theme status = %d, want 404", w.Code)
	}

	w := doSettingsGet(t, r, "/api/config/themes/"+public.DefaultTheme+"/preview-link", mintAdminToken(t))
	if w.Code != http.StatusOK {
		t.Fatalf("admin status = %d (body %s), want 200", w.Code, w.Body.String())
	}
	var res ThemePreviewLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.Theme != public.DefaultTheme {
		t.Errorf("theme = %q, want %q", res.Theme, public.DefaultTheme)
	}
	if !strings.Contains(res.URL, "theme="+public.DefaultTheme) ||
		!strings.Contains(res.URL, public.ThemePreviewParam+"=") {
		t.Errorf("link %q is not a signed %s preview", res.URL, public.DefaultTheme)
	}
}
