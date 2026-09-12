// Package public serves the embedded admin SPA, theme assets and the
// server-side-rendered public pages. All state (database, base URL, data
// directory) is injected via NewRenderer.
package public

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// staticFS embeds the admin SPA build. It keeps serving every non-public
// route (login, write, /admin/*, ...) until those move under /admin.
//
//go:embed dist/**/*
//go:embed dist/manifest.json
var staticFS embed.FS

//go:embed dist/index.html
var indexHTML []byte

// defaultThemeFS embeds the built-in theme: Go-template pages (index.html,
// post.html, user.html, 404.html) plus their static assets under assets/.
// It is produced by the frontend-public build.
//
//go:embed default-theme
var defaultThemeFS embed.FS

// ThemeInfo represents metadata for a theme. Preview is the cover image and
// must be an http(s) URL when set; local paths are rejected at upload time.
type ThemeInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Preview     string `json:"preview,omitempty"`
}

// Theme-related constants shared with the settings domain.
const (
	ThemesDir     = "theme"
	FaviconFile   = "favicon.ico"
	DefaultTheme  = "default"
	ThemeMetaFile = "vexgo-theme.json"
)

// Renderer serves static files and themes using the injected database, base
// URL and data directory.
type Renderer struct {
	db        *gorm.DB
	baseURL   string
	dataDir   string
	jwtSecret []byte
}

// NewRenderer creates a Renderer with the given dependencies.
func NewRenderer(db *gorm.DB, baseURL, dataDir string) *Renderer {
	// The themes directory holds uploaded themes. Failing to create it is not
	// fatal here, but left unreported it would surface much later as a
	// baffling "file not found" during a theme install or preview.
	themesDir := filepath.Join(dataDir, ThemesDir)
	if err := os.MkdirAll(themesDir, 0o750); err != nil {
		slog.Warn("failed to create themes directory", "dir", themesDir, "err", err)
	}
	return &Renderer{db: db, baseURL: baseURL, dataDir: dataDir}
}

// SetJWTSecret configures the JWT secret used for draft page previews.
func (r *Renderer) SetJWTSecret(secret []byte) {
	r.jwtSecret = secret
}

// BaseURL returns the configured site base URL used for SSR links.
func (r *Renderer) BaseURL() string {
	return r.baseURL
}

// DataDir returns the configured data directory used for themes and uploads.
func (r *Renderer) DataDir() string {
	return r.dataDir
}

// GetIndexHTML returns the embedded admin SPA index.html content
func GetIndexHTML() []byte {
	return indexHTML
}

// ReadAsset reads an asset file from the embedded admin SPA filesystem.
// The given path is relative to the embedded `dist/` directory.
func ReadAsset(assetPath string) ([]byte, error) {
	// Use forward slashes for embed.FS compatibility across platforms
	return staticFS.ReadFile("dist/" + assetPath)
}

// GetAvailableThemes scans the themes directory and returns a list of available themes.
// Each theme must have a vexgo-theme.json file in its root directory.
// The embedded default theme is always available.
func (r *Renderer) GetAvailableThemes() []ThemeInfo {
	themes := []ThemeInfo{}

	// Add the default embedded theme
	themes = append(themes, ThemeInfo{
		ID:          DefaultTheme,
		Name:        "vexgo default theme",
		Author:      "vexgo",
		Version:     "1.0.0",
		Description: "vexgo default theme",
		URL:         "https://github.com/vexgo-org/vexgo",
	})

	// Scan themes directory
	themesPath := filepath.Join(r.dataDir, ThemesDir)
	entries, err := os.ReadDir(themesPath)
	if err != nil {
		return themes
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeID := entry.Name()
		if themeID == DefaultTheme {
			continue // Skip default theme, already added above
		}

		// Check if vexgo-theme.json exists
		metaPath := filepath.Join(themesPath, themeID, ThemeMetaFile)
		content, err := os.ReadFile(metaPath)
		if err != nil {
			continue // Skip themes without metadata file
		}

		var themeInfo ThemeInfo
		if err := json.Unmarshal(content, &themeInfo); err != nil {
			continue // Skip themes with invalid metadata
		}

		// Set ID if not present in metadata
		if themeInfo.ID == "" {
			themeInfo.ID = themeID
		}

		themes = append(themes, themeInfo)
	}

	return themes
}

// ThemeExists checks if a theme exists (either custom theme with metadata or default theme)
func (r *Renderer) ThemeExists(themeID string) bool {
	if themeID == DefaultTheme {
		return true
	}

	metaPath := filepath.Join(r.dataDir, ThemesDir, themeID, ThemeMetaFile)
	_, err := os.Stat(metaPath)
	return err == nil
}

// ValidateThemeTemplates parses every template the theme provides and
// returns an error when the theme ships no templates or any template fails
// to parse. Activation requires this to succeed so a broken theme can never
// become the site-wide active theme.
func (r *Renderer) ValidateThemeTemplates(themeID string) error {
	_, err := r.parseThemeTemplates(themeID)
	return err
}

// InvalidateThemeCache drops the parsed-template caches for one theme so the
// next request re-reads it from disk (used after upload/overwrite/delete).
func (r *Renderer) InvalidateThemeCache(themeID string) {
	themeCache.Lock()
	delete(themeCache.themes, themeID)
	themeCache.Unlock()
	themeSourcesCache.Lock()
	delete(themeSourcesCache.themes, themeID)
	themeSourcesCache.Unlock()
}

// activeTheme returns the currently active theme from the database, falling
// back to the default theme.
func (r *Renderer) activeTheme() string {
	var config model.ThemeConfig
	if err := r.db.First(&config).Error; err != nil {
		return DefaultTheme
	}
	if config.ActiveTheme == "" {
		return DefaultTheme
	}
	return config.ActiveTheme
}

// legacyAdminRedirect maps a top-level route that moved under /admin/ to its
// new location, preserving the trailing path and query string (emailed links
// such as /verify-email?token=... must keep their query). It returns ok false
// for paths that never were SPA routes.
func legacyAdminRedirect(u *url.URL) (string, bool) {
	path := u.Path
	if !strings.HasPrefix(path, "/") {
		return "", false
	}
	// Segment 1 is the old top-level route name ("login", "edit-post", ...).
	segments := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	switch segments[0] {
	case "login", "register", "reset-password", "verify-email", "write",
		"profile", "my-posts", "notifications", "settings", "edit-post":
		target := "/admin" + path
		if u.RawQuery != "" {
			target += "?" + u.RawQuery
		}
		return target, true
	default:
		return "", false
	}
}

// getRequestedTheme determines which theme should be used for this request.
// The ?theme= preview switch is honored only for an authenticated admin (the
// same check that gates draft previews); every other visitor gets the globally
// active theme, so a visitor cannot render a theme that is not active.
func (r *Renderer) getRequestedTheme(c *gin.Context) string {
	if theme, _, ok := r.previewThemeOverride(c); ok {
		return theme
	}
	if theme := r.activeTheme(); theme != "" {
		return theme
	}
	return DefaultTheme
}

// uploadContentTypes maps stored-media extensions to the content type served
// for them. It never maps to a type a browser executes as a document
// (html/svg/xml/js are absent), and any unlisted extension falls back to
// application/octet-stream. Because the type is set explicitly before
// http.ServeContent runs, the file server never sniffs the bytes, so an
// uploaded file cannot be rendered as a document on this origin. The upload
// domain keeps its own filename allowlist; an extension it allows but this map
// omits is simply served as an opaque byte stream.
var uploadContentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
	".tif":  "image/tiff",
	".tiff": "image/tiff",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".m4a":  "audio/mp4",
	".pdf":  "application/pdf",
	".txt":  "text/plain; charset=utf-8",
	".csv":  "text/csv; charset=utf-8",
	".md":   "text/plain; charset=utf-8",
}

// uploadHandler serves one local media file. The path is confined to mediaDir
// with os.Root, and the content type comes from uploadContentTypes (never from
// sniffing), so a hostile upload can never be served as an executable document.
func (r *Renderer) uploadHandler(mediaDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clean := path.Clean(strings.TrimPrefix(c.Param("filepath"), "/"))
		if clean == "." || !fs.ValidPath(clean) {
			c.Status(http.StatusNotFound)
			return
		}

		root, err := os.OpenRoot(mediaDir)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer func() { _ = root.Close() }()

		file, err := root.Open(clean)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer func() { _ = file.Close() }()

		info, err := file.Stat()
		if err != nil || info.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}

		contentType, ok := uploadContentTypes[strings.ToLower(path.Ext(clean))]
		if !ok {
			contentType = "application/octet-stream"
		}
		c.Header("Content-Type", contentType)
		http.ServeContent(c.Writer, c.Request, clean, info.ModTime(), file)
	}
}

// RegisterStaticRoutes registers all static file routes, theme support and the
// server-side-rendered public pages.
func (r *Renderer) RegisterStaticRoutes(e *gin.Engine, s3Enabled bool) {
	// Serve local uploads if S3 is not enabled
	if !s3Enabled {
		serveUpload := r.uploadHandler(filepath.Join(r.dataDir, "media"))
		e.GET("/uploads/*filepath", serveUpload)
		e.HEAD("/uploads/*filepath", serveUpload)
	}

	// Admin SPA assets. The SPA is built with base /admin/, so its HTML
	// references /admin/assets/... and everything non-public lives under
	// /admin/.
	e.GET("/admin/assets/*filepath", r.handleAdminAsset)

	// Theme assets: resolved against the active theme so templates can
	// reference them with a stable /theme-assets/... prefix no matter which
	// theme is active.
	e.GET("/theme-assets/*filepath", r.handleThemeAsset)

	// Public pages: server-side rendered by the active theme.
	e.GET("/", r.handleIndex)
	e.GET("/posts/:slug", r.handlePost)
	e.GET("/post/:slug", r.handlePost)
	e.GET("/user/:id", r.handleUser)

	// Favicon: allow overrides via configured site icon (highest priority),
	// then ./data/favicon.ico, then the active theme's favicon.
	e.GET("/favicon.ico", r.handleFavicon)

	// Theme file route: /themes/:id/assets/*path serves a theme's static
	// assets, which an admin theme preview needs because browser subresource
	// requests (CSS, JS, images) cannot carry the bearer token. Everything else
	// in a theme — templates, i18n files and the vexgo-theme.json manifest —
	// stays private, so this is not a public read of arbitrary theme files.
	e.GET("/themes/:id/*path", r.handleThemeFile)

	// SPA fallback: the admin SPA is the only non-public surface and lives
	// under /admin/. Everything else that used to be an SPA route is
	// redirected to its /admin/ equivalent so old bookmarks and emailed links
	// keep working; unknown paths are 404s.
	e.NoRoute(r.handleNoRoute)
}

// serveAsset writes an asset with a content type derived from its extension,
// falling back to a download-safe binary type so an unknown extension is never
// rendered as a document. Shared by the SPA, theme and preview asset routes.
func serveAsset(c *gin.Context, file string, content []byte) {
	mimeType := mime.TypeByExtension(filepath.Ext(file))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Data(http.StatusOK, mimeType, content)
}

// handleAdminAsset serves the built SPA's hashed assets from the embedded dist.
func (r *Renderer) handleAdminAsset(c *gin.Context) {
	file := strings.TrimPrefix(c.Param("filepath"), "/")
	content, err := ReadAsset("assets/" + file)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	serveAsset(c, file, content)
}

// handleThemeAsset serves the active theme's assets/ directory through the
// stable /theme-assets/ prefix, so templates do not have to know which theme
// is active.
func (r *Renderer) handleThemeAsset(c *gin.Context) {
	file := strings.TrimPrefix(c.Param("filepath"), "/")
	content, ok := r.readThemeFile(r.getRequestedTheme(c), filepath.Join("assets", file))
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	serveAsset(c, file, content)
}

// handleFavicon serves /favicon.ico, preferring the configured site icon, then
// ./data/favicon.ico, then the active theme's copy.
func (r *Renderer) handleFavicon(c *gin.Context) {
	var settings model.GeneralSettings
	if err := r.db.First(&settings).Error; err == nil && settings.SiteIcon != "" {
		if r.serveConfiguredIcon(c, settings.SiteIcon) {
			return
		}
	}

	localFavicon := filepath.Join(r.dataDir, FaviconFile)
	if _, err := os.Stat(localFavicon); err == nil {
		c.File(localFavicon)
		return
	}

	if content, ok := r.readThemeFile(r.getRequestedTheme(c), FaviconFile); ok {
		c.Data(http.StatusOK, "image/x-icon", content)
		return
	}
	c.Status(http.StatusNotFound)
}

// serveConfiguredIcon serves or redirects the configured site icon and reports
// whether it answered the request. A local upload that is gone from disk
// reports false so the caller can fall back to the other favicon sources.
func (r *Renderer) serveConfiguredIcon(c *gin.Context, iconURL string) bool {
	if !strings.HasPrefix(iconURL, "/uploads/") {
		// External or S3 URL: let the browser fetch it directly.
		c.Redirect(http.StatusFound, iconURL)
		return true
	}
	localPath := filepath.Join(r.dataDir, "media", filepath.Base(iconURL))
	if _, err := os.Stat(localPath); err != nil {
		return false
	}
	c.File(localPath)
	return true
}

// handleThemeFile serves a non-active theme's static assets for the admin theme
// preview, which cannot carry a bearer token on subresource requests. Only
// assets/ is reachable: templates, i18n files and the vexgo-theme.json manifest
// stay private, so this is not a public read of arbitrary theme files.
func (r *Renderer) handleThemeFile(c *gin.Context) {
	themeID := c.Param("id")
	// Clean before the prefix check so an `assets/../vexgo-theme.json`
	// traversal collapses to the private path and is rejected.
	clean := path.Clean(strings.TrimPrefix(c.Param("path"), "/"))
	if !strings.HasPrefix(clean, "assets/") || !fs.ValidPath(clean) {
		c.Status(http.StatusNotFound)
		return
	}
	content, ok := r.readThemeFile(themeID, clean)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	serveAsset(c, clean, content)
}

// handleNoRoute answers everything the explicit routes did not match: API and
// theme-asset 404s, the admin SPA with its legacy redirects, and finally the
// custom pages that live at /:slug.
func (r *Renderer) handleNoRoute(c *gin.Context) {
	requestPath := c.Request.URL.Path
	if strings.HasPrefix(requestPath, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
		return
	}
	if strings.HasPrefix(requestPath, "/theme-assets/") {
		c.Status(http.StatusNotFound)
		return
	}
	if requestPath == "/admin" || strings.HasPrefix(requestPath, "/admin/") {
		c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
		return
	}
	if target, ok := legacyAdminRedirect(c.Request.URL); ok {
		c.Redirect(http.StatusMovedPermanently, target)
		return
	}
	// Custom pages live at /:slug as the last match. Only single-segment GET
	// paths reach this point (multi-segment and system prefixes have returned
	// above), so delegate to the page handler; unknown slugs render the 404.
	if c.Request.Method == http.MethodGet {
		slug := strings.TrimPrefix(requestPath, "/")
		if slug != "" && !strings.Contains(slug, "/") {
			c.Params = append(c.Params, gin.Param{Key: "slug", Value: slug})
			r.handlePage(c)
			return
		}
	}
	c.Status(http.StatusNotFound)
}
