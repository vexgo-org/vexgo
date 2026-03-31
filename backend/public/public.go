package public

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BaseURL is the site's base URL
var BaseURL = "http://localhost:3001"

// ActiveThemeProvider is a function that returns the currently active theme ID from the database.
// It is set by the handler package after DB initialization to avoid circular imports.
var ActiveThemeProvider func() string

// DBProvider is a function that returns the database instance.
// It is set by the main package after DB initialization to avoid circular imports.
var DBProvider func() *gorm.DB

//go:embed dist/**/*
var staticFS embed.FS

//go:embed dist/index.html
var indexHTML []byte

// GetIndexHTML returns the embedded index.html content
func GetIndexHTML() []byte {
	return indexHTML
}

// ReadAsset reads an asset file from the embedded filesystem.
// The given path is relative to the embedded `dist/` directory.
func ReadAsset(path string) ([]byte, error) {
	// Use forward slashes for embed.FS compatibility across platforms
	return staticFS.ReadFile("dist/" + path)
}

// AssetExists checks if an asset exists in the embedded filesystem.
func AssetExists(path string) bool {
	_, err := ReadAsset(path)
	return err == nil
}

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

// Default constants for theme support
const (
	DataDir       = "./data"
	ThemesDir     = "theme"
	FaviconFile   = "favicon.ico"
	DefaultTheme  = "default"
	ThemeMetaFile = "vexgo-theme.json"

	// Theme internal structure definition
	DistDir   = "dist"       // Static assets directory
	IndexFile = "index.html" // Relative to DistDir
)

func init() {
	_ = os.MkdirAll(filepath.Join(DataDir, ThemesDir), 0755)
}

// GetAvailableThemes scans the themes directory and returns a list of available themes.
// Each theme must have a vexgo-theme.json file in its root directory.
// The embedded default theme is always available.
func GetAvailableThemes() []ThemeInfo {
	themes := []ThemeInfo{}

	// Add the default embedded theme
	themes = append(themes, ThemeInfo{
		ID:          DefaultTheme,
		Name:        "vexgo default theme",
		Author:      "vexgo",
		Version:     "1.0.0",
		Description: "vexgo default theme",
		URL:         "https://github.com/vexgo/vexgo",
	})

	// Scan themes directory
	themesPath := filepath.Join(DataDir, ThemesDir)
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
func ThemeExists(themeID string) bool {
	if themeID == DefaultTheme {
		return true
	}

	metaPath := filepath.Join(DataDir, ThemesDir, themeID, ThemeMetaFile)
	_, err := os.Stat(metaPath)
	return err == nil
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
// to the globally active theme stored in the database via ActiveThemeProvider.
func getRequestedTheme(c *gin.Context) string {
	// Query param takes precedence (allows admin to preview themes)
	if theme := c.Query("theme"); theme != "" {
		return theme
	}
	// Use the DB-stored active theme via the injected provider
	if ActiveThemeProvider != nil {
		if theme := ActiveThemeProvider(); theme != "" {
			return theme
		}
	}
	return DefaultTheme
}

// getFileContent reads a file for the given theme, falling back to the embedded default theme.
func getFileContent(themeID, relativePath string) ([]byte, string, bool) {
	cleanPath := strings.TrimPrefix(relativePath, "/")
	cleanPath = filepath.Clean(cleanPath)

	if themeID != DefaultTheme {
		if strings.Contains(themeID, "..") || strings.Contains(themeID, "/") || strings.Contains(themeID, "\\") {
			return nil, "", false
		}

		themeBasePath := filepath.Join(DataDir, ThemesDir, themeID)
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
func RegisterStaticRoutes(r *gin.Engine, dataDir string, s3Enabled bool) {
	// Initialize asset manifest for dynamic asset loading
	if err := LoadAssetManifest(); err != nil {
		// Log error but continue with fallback to hardcoded paths
		fmt.Printf("Warning: Failed to load asset manifest: %v\n", err)
	}

	// Serve local uploads if S3 is not enabled
	if !s3Enabled {
		mediaDir := filepath.Join(dataDir, "media")
		r.Static("/uploads", mediaDir)
	}

	// Serve embedded assets (the default theme's assets)
	r.GET("/assets/*filepath", func(c *gin.Context) {
		file := strings.TrimPrefix(c.Param("filepath"), "/")
		theme := getRequestedTheme(c)

		if theme != DefaultTheme {
			targetFile := path.Join(DistDir, "assets", file)
			content, mimeType, exists := getFileContent(theme, targetFile)
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
	r.GET("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		// Check if it's an API request
		if strings.Contains(c.Request.Header.Get("Accept"), "application/json") {
			// Let the API handler process it
			c.Next()
			return
		}

		// Server-side rendering
		if DBProvider == nil {
			// Database not initialized, fall back to SPA
			c.Next()
			return
		}

		var post model.Post
		if err := DBProvider().Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {
			// Post not found, fall back to SPA for frontend 404 handling
			c.Next()
			return
		}

		// Render HTML
		html, err := RenderPostHTML(post, BaseURL)
		if err != nil {
			// Rendering failed, fall back to SPA
			c.Next()
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	})

	// Post detail page - server-side rendering (singular form)
	r.GET("/post/:id", func(c *gin.Context) {
		id := c.Param("id")

		// 检查是否是API请求
		if strings.Contains(c.Request.Header.Get("Accept"), "application/json") {
			// 让API处理器处理
			c.Next()
			return
		}

		// 服务器端渲染
		if DBProvider == nil {
			// 数据库未初始化，回退到SPA
			c.Next()
			return
		}

		var post model.Post
		if err := DBProvider().Preload("Author").Preload("Tags").First(&post, id).Error; err != nil {

			c.Next()
			return
		}

		html, err := RenderPostHTML(post, BaseURL)
		if err != nil {

			c.Next()
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	})

	// Root route
	r.GET("/", func(c *gin.Context) {

		if strings.Contains(c.Request.Header.Get("Accept"), "application/json") {

			c.Next()
			return
		}

		// Server-side rendering with proper data initialization
		if DBProvider != nil {
			var posts []model.Post
			DBProvider().Preload("Author").Preload("Tags").Where("status = ?", "published").Order("created_at DESC").Limit(10).Find(&posts)
			// Always render, even with empty posts or query errors
			html, renderErr := RenderIndexHTML(posts, BaseURL)
			if renderErr == nil {
				c.Data(http.StatusOK, "text/html; charset=utf-8", html)
				return
			}
		}

		// Fall back to default HTML
		theme := getRequestedTheme(c)
		if theme == DefaultTheme {
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}

		// For custom themes, try to load from local theme directory
		targetFile := path.Join(DistDir, IndexFile)
		content, _, exists := getFileContent(theme, targetFile)
		if !exists {
			// Fall back to default theme
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	// Favicon: allow overrides via ./data/favicon.ico (highest priority)
	r.GET("/favicon.ico", func(c *gin.Context) {
		localFavicon := filepath.Join(DataDir, FaviconFile)
		if _, err := os.Stat(localFavicon); err == nil {
			c.File(localFavicon)
			return
		}

		theme := getRequestedTheme(c)
		content, mimeType, exists := getFileContent(theme, path.Join(DistDir, FaviconFile))
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
	r.GET("/themes/:id/*path", func(c *gin.Context) {
		themeID := c.Param("id")
		reqPath := c.Param("path")
		content, mimeType, exists := getFileContent(themeID, reqPath)
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
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		theme := getRequestedTheme(c)

		// If using default theme or custom theme file not found, serve default
		if theme == DefaultTheme {
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}

		targetFile := path.Join(DistDir, IndexFile)
		content, _, exists := getFileContent(theme, targetFile)
		if !exists {
			// Fall back to default theme
			c.Data(http.StatusOK, "text/html; charset=utf-8", GetIndexHTML())
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})
}
