// Package public serves the embedded admin SPA, theme assets and the
// server-side-rendered public pages. All state (database, base URL, data
// directory) is injected via NewRenderer.
package public

import (
	"embed"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"os"
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

// ThemeInfo represents metadata for a theme
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
	db      *gorm.DB
	baseURL string
	dataDir string
}

// NewRenderer creates a Renderer with the given dependencies.
func NewRenderer(db *gorm.DB, baseURL, dataDir string) *Renderer {
	// Ensure the themes directory exists
	_ = os.MkdirAll(filepath.Join(dataDir, ThemesDir), 0o755)
	return &Renderer{db: db, baseURL: baseURL, dataDir: dataDir}
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

// IsPathInside verifies that targetPath is within basePath. Exported because
// other domains resolve untrusted paths against theme directories too (e.g.
// settings theme previews) and must apply the same containment check.
func IsPathInside(basePath, targetPath string) bool {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}

	cleanTarget := filepath.Clean(targetPath)
	fullPath := filepath.Join(absBase, cleanTarget)
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..") && rel != ".."
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
// It checks the 'theme' query parameter first (for admin preview), then falls back
// to the globally active theme stored in the database.
func (r *Renderer) getRequestedTheme(c *gin.Context) string {
	// Query param takes precedence (allows admin to preview themes)
	if theme := c.Query("theme"); theme != "" {
		return theme
	}
	// Use the DB-stored active theme
	if theme := r.activeTheme(); theme != "" {
		return theme
	}
	return DefaultTheme
}

// RegisterStaticRoutes registers all static file routes, theme support and the
// server-side-rendered public pages.
func (r *Renderer) RegisterStaticRoutes(e *gin.Engine, s3Enabled bool) {
	// Serve local uploads if S3 is not enabled
	if !s3Enabled {
		mediaDir := filepath.Join(r.dataDir, "media")
		e.Static("/uploads", mediaDir)
	}

	// Admin SPA assets. The SPA is built with base /admin/, so its HTML
	// references /admin/assets/... and everything non-public lives under
	// /admin/.
	e.GET("/admin/assets/*filepath", func(c *gin.Context) {
		file := strings.TrimPrefix(c.Param("filepath"), "/")
		content, err := ReadAsset("assets/" + file)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		ext := filepath.Ext(file)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			c.Data(http.StatusOK, mimeType, content)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", content)
	})

	// Theme assets: resolved against the active theme so templates can
	// reference them with a stable /theme-assets/... prefix no matter which
	// theme is active.
	e.GET("/theme-assets/*filepath", func(c *gin.Context) {
		theme := r.getRequestedTheme(c)
		file := strings.TrimPrefix(c.Param("filepath"), "/")
		content, ok := r.readThemeFile(theme, filepath.Join("assets", file))
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		ext := filepath.Ext(file)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			c.Data(http.StatusOK, mimeType, content)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", content)
	})

	// Public pages: server-side rendered by the active theme.
	e.GET("/", r.handleIndex)
	e.GET("/posts/:slug", r.handlePost)
	e.GET("/post/:slug", r.handlePost)
	e.GET("/user/:id", r.handleUser)

	// Favicon: allow overrides via configured site icon (highest priority),
	// then ./data/favicon.ico, then the active theme's favicon.
	e.GET("/favicon.ico", func(c *gin.Context) {
		var settings model.GeneralSettings
		if err := r.db.First(&settings).Error; err == nil && settings.SiteIcon != "" {
			iconURL := settings.SiteIcon
			// If it's a local path (starts with /), serve from the local filesystem
			if strings.HasPrefix(iconURL, "/uploads/") {
				localPath := filepath.Join(r.dataDir, "media", filepath.Base(iconURL))
				if _, err := os.Stat(localPath); err == nil {
					c.File(localPath)
					return
				}
			} else {
				// External URL or S3 URL - redirect
				c.Redirect(http.StatusFound, iconURL)
				return
			}
		}

		localFavicon := filepath.Join(r.dataDir, FaviconFile)
		if _, err := os.Stat(localFavicon); err == nil {
			c.File(localFavicon)
			return
		}

		theme := r.getRequestedTheme(c)
		content, ok := r.readThemeFile(theme, FaviconFile)
		if ok {
			c.Data(http.StatusOK, "image/x-icon", content)
			return
		}
		c.Status(http.StatusNotFound)
	})

	// Theme file route: /themes/:id/*path serves any file of a theme
	// (used by the admin console for previews and raw file access).
	e.GET("/themes/:id/*path", func(c *gin.Context) {
		themeID := c.Param("id")
		reqPath := c.Param("path")
		content, ok := r.readThemeFile(themeID, reqPath)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		mimeType := mime.TypeByExtension(filepath.Ext(reqPath))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.Data(http.StatusOK, mimeType, content)
	})

	// SPA fallback: the admin SPA is the only non-public surface and lives
	// under /admin/. Everything else that used to be an SPA route is
	// redirected to its /admin/ equivalent so old bookmarks and emailed links
	// keep working; unknown paths are 404s.
	e.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}
		if strings.HasPrefix(path, "/theme-assets/") {
			c.Status(http.StatusNotFound)
			return
		}
		if path == "/admin" || strings.HasPrefix(path, "/admin/") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}
		if target, ok := legacyAdminRedirect(c.Request.URL); ok {
			c.Redirect(http.StatusMovedPermanently, target)
			return
		}
		c.Status(http.StatusNotFound)
	})
}
