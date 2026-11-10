package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestThemeLanguages(t *testing.T) {
	svc, _ := newTestService(t)
	langs, err := svc.ThemeLanguages("default")
	if err != nil {
		t.Fatalf("ThemeLanguages error: %v", err)
	}
	found := map[string]bool{}
	for _, l := range langs {
		found[l] = true
	}
	if !found["en"] || !found["zh"] {
		t.Errorf("default theme should list en and zh, got %v", langs)
	}
	if _, err := svc.ThemeLanguages("no-such-theme"); err == nil {
		t.Error("expected error for unknown theme")
	}
}

func TestGetThemeConfig_Default(t *testing.T) {
	svc, _ := newTestService(t)
	theme, err := svc.GetThemeConfig(context.Background())
	if err != nil {
		t.Fatalf("GetThemeConfig error: %v", err)
	}
	if theme != "default" {
		t.Errorf("expected default theme, got %q", theme)
	}
}

func TestUpdateThemeConfig(t *testing.T) {
	svc, _ := newTestService(t)

	// unknown theme rejected
	if _, err := svc.UpdateThemeConfig(context.Background(), "no-such-theme"); !errors.Is(err, ErrThemeNotFound) {
		t.Errorf("expected ErrThemeNotFound, got %v", err)
	}

	// default theme is always valid
	theme, err := svc.UpdateThemeConfig(context.Background(), "default")
	if err != nil {
		t.Fatalf("UpdateThemeConfig error: %v", err)
	}
	if theme != "default" {
		t.Errorf("expected default theme, got %q", theme)
	}

	got, err := svc.GetThemeConfig(context.Background())
	if err != nil {
		t.Fatalf("GetThemeConfig error: %v", err)
	}
	if got != "default" {
		t.Errorf("expected persisted default theme, got %q", got)
	}
}

func TestValidatePreviewURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"https://example.com/cover.png", true},
		{"http://localhost:3000/cover.jpg", true},
		{"https://example.com/a?x=1#y", true},
		{"ftp://example.com/cover.png", false},
		{"data:image/png;base64,aaa", false},
		{"/assets/cover.png", false},
		{"assets/cover.png", false},
		{"../secret.png", false},
		{"https://", false},
		{"not a url", false},
		{strings.Repeat("https://example.com/", 200), false},
	}
	for _, tc := range cases {
		if got := ValidatePreviewURL(tc.in); got != tc.want {
			t.Errorf("ValidatePreviewURL(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestDeleteTheme_Boundaries covers guest-visible state changes only through
// the service: default/active protection, unknown id, and happy path.
func TestDeleteTheme_Boundaries(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	dataDir := svc.themes.DataDir()

	// default is protected
	if err := svc.DeleteTheme(ctx, "default"); !errors.Is(err, ErrCannotDeleteDefault) {
		t.Errorf("expected ErrCannotDeleteDefault, got %v", err)
	}

	// unknown theme
	if err := svc.DeleteTheme(ctx, "no-such-theme"); !errors.Is(err, ErrThemeNotFound) {
		t.Errorf("expected ErrThemeNotFound, got %v", err)
	}

	// install a valid theme with one template
	themeID := "deletable"
	themeDir := filepath.Join(dataDir, public.ThemesDir, themeID)
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	meta := `{"id": "deletable", "name": "D", "version": "1.0.0", "preview": "https://example.com/c.png"}`
	if err := os.WriteFile(filepath.Join(themeDir, public.ThemeMetaFile), []byte(meta), 0o600); err != nil {
		t.Fatalf("write meta error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "index.html"), []byte(`hello`), 0o600); err != nil {
		t.Fatalf("write template error: %v", err)
	}

	// activate it, then delete must refuse while active
	if _, err := svc.UpdateThemeConfig(ctx, themeID); err != nil {
		t.Fatalf("activate error: %v", err)
	}
	if err := svc.DeleteTheme(ctx, themeID); !errors.Is(err, ErrThemeIsActive) {
		t.Errorf("expected ErrThemeIsActive, got %v", err)
	}

	// switch back to default, then delete succeeds
	if _, err := svc.UpdateThemeConfig(ctx, "default"); err != nil {
		t.Fatalf("deactivate error: %v", err)
	}
	if err := svc.DeleteTheme(ctx, themeID); err != nil {
		t.Fatalf("DeleteTheme error: %v", err)
	}
	if _, err := os.Stat(themeDir); !os.IsNotExist(err) {
		t.Errorf("expected theme dir removed, stat err: %v", err)
	}
}

// TestUpdateThemeConfig_RejectsBrokenTemplates ensures activation never
// promotes a theme whose templates fail to parse.
func TestUpdateThemeConfig_RejectsBrokenTemplates(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	dataDir := svc.themes.DataDir()

	themeID := "broken"
	themeDir := filepath.Join(dataDir, public.ThemesDir, themeID)
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	meta := `{"id": "broken", "name": "B", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(themeDir, public.ThemeMetaFile), []byte(meta), 0o600); err != nil {
		t.Fatalf("write meta error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "index.html"), []byte(`{{ .Unclosed`), 0o600); err != nil {
		t.Fatalf("write template error: %v", err)
	}

	if _, err := svc.UpdateThemeConfig(ctx, themeID); !errors.Is(err, ErrThemeTemplatesInvalid) {
		t.Errorf("expected ErrThemeTemplatesInvalid, got %v", err)
	}
}
