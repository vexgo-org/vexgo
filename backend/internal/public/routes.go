package public

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all static file routes, theme support and the
// server-side-rendered public pages.
func (h *Handler) RegisterRoutes(e *gin.Engine, s3Enabled bool) {
	// Serve local uploads if S3 is not enabled
	if !s3Enabled {
		serveUpload := h.uploadHandler(filepath.Join(h.svc.DataDir(), "media"))
		e.GET("/uploads/*filepath", serveUpload)
		e.HEAD("/uploads/*filepath", serveUpload)
	}

	// Admin SPA assets. The SPA is built with base /admin/, so its HTML
	// references /admin/assets/... and everything non-public lives under
	// /admin/.
	e.GET("/admin/assets/*filepath", h.handleAdminAsset)

	// Theme assets: resolved against the active theme so templates can
	// reference them with a stable /theme-assets/... prefix no matter which
	// theme is active.
	e.GET("/theme-assets/*filepath", h.handleThemeAsset)

	// Public pages: server-side rendered by the active theme.
	e.GET("/", h.handleIndex)
	e.GET("/posts/:slug", h.handlePost)
	e.GET("/post/:slug", h.handlePost)
	e.GET("/user/:id", h.handleUser)

	// Theme file route: /themes/:id/assets/*path serves a theme's static
	// assets, which an admin theme preview needs because browser subresource
	// requests (CSS, JS, images) cannot carry the bearer token. Everything else
	// in a theme — templates, i18n files and the vexgo-theme.json manifest —
	// stays private, so this is not a public read of arbitrary theme files.
	e.GET("/themes/:id/*path", h.handleThemeFile)

	// SPA fallback: the admin SPA is the only non-public surface and lives
	// under /admin/. Everything else that used to be an SPA route is
	// redirected to its /admin/ equivalent so old bookmarks and emailed links
	// keep working; unknown paths are 404s.
	e.NoRoute(h.handleNoRoute)
}

// handleNoRoute answers everything the explicit routes did not match: API and
// theme-asset 404s, the admin SPA with its legacy redirects, and finally the
// custom pages that live at /:slug.
func (h *Handler) handleNoRoute(c *gin.Context) {
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
		c.Data(http.StatusOK, "text/html; charset=utf-8", getIndexHTML())
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
			h.handlePage(c)
			return
		}
	}
	c.Status(http.StatusNotFound)
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
