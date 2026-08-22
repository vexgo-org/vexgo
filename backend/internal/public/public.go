// Package public serves the embedded frontend, static assets, themes and the
// server-side-rendered pages. All state (database, base URL, data directory)
// is injected via NewRenderer.
package public

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

//go:embed dist/**/*
//go:embed dist/manifest.json
var staticFS embed.FS

//go:embed dist/index.html
var indexHTML []byte

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

	// Theme internal structure definition
	DistDir   = "dist"       // Static assets directory
	IndexFile = "index.html" // Relative to DistDir
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

// GetIndexHTML returns the embedded index.html content
func GetIndexHTML() []byte {
	return indexHTML
}

// ReadAsset reads an asset file from the embedded filesystem.
// The given path is relative to the embedded `dist/` directory.
func ReadAsset(assetPath string) ([]byte, error) {
	// Use forward slashes for embed.FS compatibility across platforms
	return staticFS.ReadFile("dist/" + assetPath)
}

// AssetExists checks if an asset exists in the embedded filesystem.
func AssetExists(assetPath string) bool {
	_, err := ReadAsset(assetPath)
	return err == nil
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

// isSafePath verifies that targetPath is within basePath
func isSafePath(basePath, targetPath string) bool {
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

// getFileContent reads a file for the given theme, falling back to the embedded default theme.
func (r *Renderer) getFileContent(themeID, relativePath string) ([]byte, string, bool) {
	cleanPath := strings.TrimPrefix(relativePath, "/")
	cleanPath = filepath.Clean(cleanPath)

	if themeID != DefaultTheme {
		if strings.Contains(themeID, "..") || strings.Contains(themeID, "/") || strings.Contains(themeID, "\\") {
			return nil, "", false
		}

		themeBasePath := filepath.Join(r.dataDir, ThemesDir, themeID)
		if !isSafePath(themeBasePath, cleanPath) {
			return nil, "", false
		}

		localPath := filepath.Join(themeBasePath, cleanPath)
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			content, err := os.ReadFile(localPath)
			if err == nil {
				return content, mime.TypeByExtension(filepath.Ext(localPath)), true
			}
		}
		// If local theme file doesn't exist / can't be read, fall back to embedded
	}

	// For default theme, read from embedded filesystem
	// cleanPath should be "dist/index.html" or "dist/assets/..." format
	if content, err := fs.ReadFile(staticFS, cleanPath); err == nil {
		return content, mime.TypeByExtension(filepath.Ext(cleanPath)), true
	}
	return nil, "", false
}

// RegisterStaticRoutes registers all static file routes, including theme support.
func (r *Renderer) RegisterStaticRoutes(e *gin.Engine, s3Enabled bool) {
	// Initialize asset manifest for dynamic asset loading
	if err := LoadAssetManifest(); err != nil {
		// Log error but continue with fallback to hardcoded paths
		slog.Warn("failed to load asset manifest", "err", err)
	}

	// Serve local uploads if S3 is not enabled
	if !s3Enabled {
		mediaDir := filepath.Join(r.dataDir, "media")
		e.Static("/uploads", mediaDir)
	}

	// Serve embedded assets (the default theme's assets)
	e.GET("/assets/*filepath", func(c *gin.Context) {
		file := strings.TrimPrefix(c.Param("filepath"), "/")
		theme := r.getRequestedTheme(c)

		if theme != DefaultTheme {
			targetFile := path.Join(DistDir, "assets", file)
			content, mimeType, exists := r.getFileContent(theme, targetFile)
			if exists {
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				c.Data(http.StatusOK, mimeType, content)
				return
			}
		}

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

	// Post detail page - server-side rendering (plural form)
	e.GET("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		// Check if it's an API request
		if strings.Contains(c.Request.Header.Get("Accept"), "application/json") {
			// Let the API handler process it
			c.Next()
			return
		}

		// Server-side rendering
		var post model.Post
		if err := r.db.Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
			// Post not found, fall back to SPA for frontend 404 handling
			c.Next()
			return
		}

		// Render HTML
		html, err := RenderPostHTML(post, r.baseURL)
		if err != nil {
			// Rendering failed, fall back to SPA
			c.Next()
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	})

	// Post detail page - server-side rendering (singular form)
	e.GET("/post/:id", func(c *gin.Context) {
		id := c.Param("id")

		// Check if it's an API request
		if strings.Contains(c.Request.Header.Get("Accept"), "application/json") {
			// Let the API handler process it
			c.Next()
			return
		}

		// Server-side rendering
		var post model.Post
		if err := r.db.Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
			c.Next()
			return
		}

		html, err := RenderPostHTML(post, r.baseURL)
		if err != nil {
			c.Next()
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	})

	// Root route
	e.GET("/", func(c *gin.Context) {
		if strings.Contains(c.Request.Header.Get("Accept"), "application/json") {
			c.Next()
			return
		}

		// Server-side rendering with proper data initialization
		var posts []model.Post
		r.db.Preload("Author").
			Preload("Tags").
			Where("status = ?", model.PostStatusPublished).
			Order("created_at DESC").
			Limit(10).
			Find(&posts)
		// Always render, even with empty posts or query errors
		html, renderErr := RenderIndexHTML(posts, r.baseURL)
		if renderErr == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", html)
			return
		}

		// Fall back to default HTML
		theme := r.getRequestedTheme(c)
		if theme == DefaultTheme {
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}

		// For custom themes, try to load from local theme directory
		targetFile := path.Join(DistDir, IndexFile)
		content, _, exists := r.getFileContent(theme, targetFile)
		if !exists {
			// Fall back to default theme
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	// Favicon: allow overrides via configured site icon (highest priority), then ./data/favicon.ico, then theme default
	e.GET("/favicon.ico", func(c *gin.Context) {
		// Check if a site icon URL is configured in GeneralSettings
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
		content, mimeType, exists := r.getFileContent(theme, path.Join(DistDir, FaviconFile))
		if exists {
			if mimeType == "" {
				mimeType = "image/x-icon"
			}
			c.Data(http.StatusOK, mimeType, content)
			return
		}
		c.Status(http.StatusNotFound)
	})

	// Theme assets route: /themes/:id/*path
	e.GET("/themes/:id/*path", func(c *gin.Context) {
		themeID := c.Param("id")
		reqPath := c.Param("path")
		content, mimeType, exists := r.getFileContent(themeID, reqPath)
		if !exists {
			c.Status(http.StatusNotFound)
			return
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.Data(http.StatusOK, mimeType, content)
	})

	// SPA fallback (noRoute) - serve index.html from the requested theme
	e.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		theme := r.getRequestedTheme(c)

		// If using default theme or custom theme file not found, serve default
		if theme == DefaultTheme {
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}

		targetFile := path.Join(DistDir, IndexFile)
		content, _, exists := r.getFileContent(theme, targetFile)
		if !exists {
			// Fall back to default theme
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})
}
