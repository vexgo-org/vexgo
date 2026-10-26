package public

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type faviconTestEnv struct {
	router  *gin.Engine
	svc     *Service
	db      *gorm.DB
	dataDir string
	owner   model.User
}

func newFaviconTestEnv(t *testing.T) *faviconTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Blob{},
		&model.Asset{},
		&model.GeneralSettings{},
		&model.ThemeConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	owner := model.User{Username: "favicon-owner", Email: "favicon@example.com"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	dataDir := t.TempDir()
	svc := NewService(Deps{DB: db, BaseURL: "http://example.test", DataDir: dataDir})
	e := gin.New()
	NewHandler(svc).RegisterRoutes(e, true)
	return &faviconTestEnv{router: e, svc: svc, db: db, dataDir: dataDir, owner: owner}
}

func (e *faviconTestEnv) seedAsset(t *testing.T, mimeType string, blobType model.BlobType, visibility model.AssetVisibility) model.Asset {
	t.Helper()

	blob := model.Blob{
		StorageKey: uuid.NewString(),
		MimeType:   mimeType,
		Type:       blobType,
		Size:       1,
	}
	if err := e.db.Create(&blob).Error; err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	icon := model.Asset{
		OriginalName: "icon.ico",
		UserID:       e.owner.ID,
		BlobID:       blob.ID,
		PublicID:     uuid.New(),
		Visibility:   visibility,
	}
	if err := e.db.Create(&icon).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return icon
}

func (e *faviconTestEnv) setSiteIcon(t *testing.T, iconID uint) {
	t.Helper()
	if err := e.db.Create(&model.GeneralSettings{
		SiteName:   "Icon Site",
		SiteIconID: &iconID,
	}).Error; err != nil {
		t.Fatalf("seed general settings: %v", err)
	}
}

func (e *faviconTestEnv) writeDataFavicon(t *testing.T, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.dataDir, FaviconFile), content, 0o600); err != nil {
		t.Fatalf("write data favicon: %v", err)
	}
}

func (e *faviconTestEnv) setActiveTheme(t *testing.T, themeID string, favicon []byte) {
	t.Helper()
	dir := filepath.Join(e.dataDir, ThemesDir, themeID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, FaviconFile), favicon, 0o600); err != nil {
		t.Fatalf("write theme favicon: %v", err)
	}
	if err := e.db.Create(&model.ThemeConfig{ActiveTheme: themeID}).Error; err != nil {
		t.Fatalf("seed theme config: %v", err)
	}
}

func TestFavicon_ConfiguredIconRedirectsToAssetURL(t *testing.T) {
	env := newFaviconTestEnv(t)
	icon := env.seedAsset(t, "image/png", model.BlobTypeImage, model.AssetVisibilityPublic)
	env.setSiteIcon(t, icon.ID)
	// The configured icon must win over both fallback sources.
	env.writeDataFavicon(t, []byte("local-icon"))
	env.setActiveTheme(t, "fancy", []byte("theme-icon"))

	w := doPublicRequest(t, env.router, "/favicon.ico")
	if w.Code != http.StatusFound {
		t.Fatalf("GET /favicon.ico status = %d, want %d, body: %s", w.Code, http.StatusFound, w.Body.String())
	}
	if got, want := w.Header().Get("Location"), asset.OriginalURL(icon.PublicID); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestFavicon_LocalFileBeatsThemeAndBundled(t *testing.T) {
	env := newFaviconTestEnv(t)
	local := []byte("local-icon")
	env.writeDataFavicon(t, local)
	env.setActiveTheme(t, "fancy", []byte("theme-icon"))

	w := doPublicRequest(t, env.router, "/favicon.ico")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /favicon.ico status = %d, want 200", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), local) {
		t.Errorf("body = %q, want local favicon %q", w.Body.Bytes(), local)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestFavicon_ThemeFaviconBeatsBundled(t *testing.T) {
	env := newFaviconTestEnv(t)
	themeIcon := []byte("theme-icon")
	env.setActiveTheme(t, "fancy", themeIcon)

	w := doPublicRequest(t, env.router, "/favicon.ico")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /favicon.ico status = %d, want 200", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), themeIcon) {
		t.Errorf("body = %q, want theme favicon %q", w.Body.Bytes(), themeIcon)
	}
	if got := w.Header().Get("Content-Type"); got != "image/x-icon" {
		t.Errorf("Content-Type = %q, want image/x-icon", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestFavicon_FallsBackToBundledGlyph(t *testing.T) {
	env := newFaviconTestEnv(t)
	want, err := readAsset(DefaultFaviconAsset)
	if err != nil {
		t.Fatalf("read embedded favicon: %v", err)
	}

	w := doPublicRequest(t, env.router, "/favicon.ico")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /favicon.ico status = %d, want 200", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Errorf("body does not match the embedded favicon")
	}
	if got := w.Header().Get("Content-Type"); got != "image/x-icon" {
		t.Errorf("Content-Type = %q, want image/x-icon", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestSiteIconURL(t *testing.T) {
	env := newFaviconTestEnv(t)
	if got := env.svc.SiteIconURL(t.Context()); got != "" {
		t.Fatalf("SiteIconURL without settings = %q, want empty", got)
	}

	icon := env.seedAsset(t, "image/png", model.BlobTypeImage, model.AssetVisibilityPublic)
	env.setSiteIcon(t, icon.ID)
	want := asset.OriginalURL(icon.PublicID)

	if got := env.svc.SiteIconURL(t.Context()); got != want {
		t.Errorf("SiteIconURL = %q, want %q", got, want)
	}
	if got := env.svc.buildSiteData(t.Context(), "").Icon; got != want {
		t.Errorf("SiteData.Icon = %q, want %q", got, want)
	}
}

func TestSiteIconURL_NoRepository(t *testing.T) {
	svc := NewService(Deps{DataDir: t.TempDir()})
	if got := svc.SiteIconURL(t.Context()); got != "" {
		t.Errorf("SiteIconURL without repository = %q, want empty", got)
	}
}
