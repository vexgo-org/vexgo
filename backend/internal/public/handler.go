package public

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// staticFS embeds the admin SPA build. It keeps serving every non-public
// route (login, write, /admin/*, ...) until those move under /admin.
//
//go:embed dist/**/*
//go:embed dist/manifest.json
var staticFS embed.FS

//go:embed dist/index.html
var indexHTML []byte

const htmlContentType = "text/html; charset=utf-8"

// Handler exposes the public domain over HTTP: the server-side rendered pages
// and the static assets (admin SPA, theme assets, media uploads, favicon).
type Handler struct {
	svc *Service
}

// NewHandler creates a public HTTP handler with the given dependencies.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// getIndexHTML returns the embedded admin SPA index.html content
func getIndexHTML() []byte {
	return indexHTML
}

// readAsset reads an asset file from the embedded admin SPA filesystem.
// The given path is relative to the embedded `dist/` directory.
func readAsset(assetPath string) ([]byte, error) {
	// Use forward slashes for embed.FS compatibility across platforms
	return staticFS.ReadFile("dist/" + assetPath)
}

// requestContext resolves one SSR request's site data and merged theme
// dictionary with a single settings query: site default first, then the
// ?lang=/cookie/Accept-Language chain, then the merged dict.
func (h *Handler) requestContext(c *gin.Context, theme string) (*SiteData, Dict) {
	site := h.svc.buildSiteData(c.Request.Context(), "")
	lang := h.svc.resolveRequestLanguage(c, site.DefaultLanguage)
	site.Language = lang
	return site, h.svc.loadMergedDict(theme, lang, site.DefaultLanguage)
}

// pageRender is one SSR page to render: which theme and template file to use,
// the data handed to the template, and the theme's language dictionary.
// Keeping the coordinates in one value keeps the render helpers at two
// parameters as they grow.
type pageRender struct {
	Theme string
	Page  string
	Data  any
	Dict  Dict
}

// renderRequestPage renders one SSR page and scopes it to the previewed theme
// when the request is entitled to the ?theme= admin preview switch. Plain
// requests and non-request callers use renderThemeWithDict directly so no
// rewriting leaks into the site-wide render path.
func (h *Handler) renderRequestPage(c *gin.Context, p pageRender) ([]byte, error) {
	out, err := h.svc.renderThemeWithDict(p.Theme, p.Page, p.Data, p.Dict)
	if err != nil {
		return nil, err
	}
	if preview, token, ok := h.svc.previewThemeOverride(c); ok && preview == p.Theme {
		out = rewriteThemePreview(out, p.Theme, token)
	}
	return out, nil
}

// servePage renders one page of the requested theme and writes it as HTML with
// a 200 status. When rendering fails (theme missing a template, template syntax
// error) it falls back to a plain 404 so public routes never crash.
func (h *Handler) servePage(c *gin.Context, p pageRender) {
	out, err := h.renderRequestPage(c, p)
	if err != nil {
		c.Data(http.StatusNotFound, htmlContentType, []byte("Page not found"))
		return
	}
	c.Data(http.StatusOK, htmlContentType, out)
}

// renderNotFound renders the theme's 404 template (when present) with a plain
// fallback otherwise. Both outcomes are a 404, so the body is the only thing
// the render decides.
func (h *Handler) renderNotFound(c *gin.Context, theme string, site *SiteData, dict Dict) {
	data := NotFoundData{Site: site, Pages: h.svc.buildNavPages(c.Request.Context())}
	out, err := h.renderRequestPage(c, pageRender{Theme: theme, Page: NotFoundTemplate, Data: data, Dict: dict})
	if err != nil {
		c.Data(http.StatusNotFound, htmlContentType, []byte("Page not found"))
		return
	}
	c.Data(http.StatusNotFound, htmlContentType, out)
}

// handleIndex renders the home page: published posts, paginated, with the
// optional ?search= / ?category= / ?page= filters.
func (h *Handler) handleIndex(c *gin.Context) {
	theme := h.svc.getRequestedTheme(c)
	site, dict := h.requestContext(c, theme)
	page := parsePageParam(c)
	search := c.Query("search")
	category := c.Query("category")

	data, err := h.svc.IndexView(c.Request.Context(), site, PostListQuery{
		Base:     "/",
		Page:     page,
		Limit:    site.ItemsPerPage,
		Search:   search,
		Category: category,
	})
	if err != nil {
		h.renderNotFound(c, theme, site, dict)
		return
	}

	h.servePage(c, pageRender{Theme: theme, Page: IndexTemplate, Data: *data, Dict: dict})
}

// handlePost renders a published post by slug. Drafts, pending and rejected
// posts stay hidden — the SSR page must not expose them via a guessable slug
// when the API filters them out.
func (h *Handler) handlePost(c *gin.Context) {
	theme := h.svc.getRequestedTheme(c)
	site, dict := h.requestContext(c, theme)
	slug := c.Param("slug")

	data, err := h.svc.PostView(c.Request.Context(), site, slug)
	if err != nil {
		h.renderNotFound(c, theme, site, dict)
		return
	}

	h.servePage(c, pageRender{Theme: theme, Page: PostTemplate, Data: *data, Dict: dict})
}

// handleUser renders a user's public profile and their published posts.
func (h *Handler) handleUser(c *gin.Context) {
	theme := h.svc.getRequestedTheme(c)
	site, dict := h.requestContext(c, theme)

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || userID == 0 {
		h.renderNotFound(c, theme, site, dict)
		return
	}

	data, err := h.svc.UserView(c.Request.Context(), site, uint(userID), PostListQuery{
		Base:     "/user/" + strconv.FormatUint(userID, 10),
		Page:     parsePageParam(c),
		Limit:    site.ItemsPerPage,
		AuthorID: uint(userID),
	})
	if err != nil {
		h.renderNotFound(c, theme, site, dict)
		return
	}

	h.servePage(c, pageRender{Theme: theme, Page: UserTemplate, Data: *data, Dict: dict})
}

// handlePage renders a custom page at /:slug. Template resolution is
// <slug>.html first, then the generic page.html, then the 404 template with a
// 404 status (per the locked slug.html chain). Drafts are hidden unless the
// request carries ?preview=1 with an admin JWT.
func (h *Handler) handlePage(c *gin.Context) {
	theme := h.svc.getRequestedTheme(c)
	site, dict := h.requestContext(c, theme)
	slug := c.Param("slug")
	if slug == "" || strings.Contains(slug, "/") {
		h.renderNotFound(c, theme, site, dict)
		return
	}

	preview := c.Query("preview") == "1" && h.svc.isPreviewAdmin(c)

	data, err := h.svc.PageView(c.Request.Context(), site, slug, preview)
	if err != nil {
		h.renderNotFound(c, theme, site, dict)
		return
	}

	if dedicated := data.Page.Slug + ".html"; h.svc.themeProvides(theme, dedicated) {
		h.servePage(c, pageRender{Theme: theme, Page: dedicated, Data: *data, Dict: dict})
		return
	}
	if h.svc.themeProvides(theme, PageTemplate) {
		h.servePage(c, pageRender{Theme: theme, Page: PageTemplate, Data: *data, Dict: dict})
		return
	}
	h.renderNotFound(c, theme, site, dict)
}

// handleFavicon serves the configured site icon as a redirect to its asset
// URL, then falls back to data/favicon.ico, the active theme's favicon, and the
// bundled admin glyph.
func (h *Handler) handleFavicon(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")

	// Query the site icon from general settings.
	if iconURL := h.svc.SiteIconURL(c.Request.Context()); iconURL != "" {
		c.Redirect(http.StatusFound, iconURL)
		return
	}

	// Fallback to local `favicon.ico`
	local := filepath.Join(h.svc.DataDir(), FaviconFile)
	_, err := os.Stat(local)
	if err == nil {
		c.File(local)
		return
	}

	// Fallback to the favicon in the theme.
	if content, ok := h.svc.readThemeFile(h.svc.getRequestedTheme(c), FaviconFile); ok {
		c.Data(http.StatusOK, "image/x-icon", content)
		return
	}

	// Fallback to the internal favicon.
	if content, err := readAsset(DefaultFaviconAsset); err == nil {
		c.Data(http.StatusOK, "image/x-icon", content)
		return
	}

	// Favicon is missing.
	c.Status(http.StatusNotFound)
}

// parsePageParam reads the 1-based ?page= query parameter, defaulting to 1.
func parsePageParam(c *gin.Context) int {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 1
	}
	return page
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
func (h *Handler) handleAdminAsset(c *gin.Context) {
	file := strings.TrimPrefix(c.Param("filepath"), "/")
	content, err := readAsset("assets/" + file)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	serveAsset(c, file, content)
}

// handleThemeAsset serves the active theme's assets/ directory through the
// stable /theme-assets/ prefix, so templates do not have to know which theme
// is active.
func (h *Handler) handleThemeAsset(c *gin.Context) {
	file := strings.TrimPrefix(c.Param("filepath"), "/")
	content, ok := h.svc.readThemeFile(h.svc.getRequestedTheme(c), path.Join("assets", file))
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	serveAsset(c, file, content)
}

// handleThemeFile serves a non-active theme's static assets for the admin theme
// preview, which cannot carry a bearer token on subresource requests. Only
// assets/ is reachable: templates, i18n files and the vexgo-theme.json manifest
// stay private, so this is not a public read of arbitrary theme files.
func (h *Handler) handleThemeFile(c *gin.Context) {
	themeID := c.Param("id")
	// Clean before the prefix check so an `assets/../vexgo-theme.json`
	// traversal collapses to the private path and is rejected.
	clean := path.Clean(strings.TrimPrefix(c.Param("path"), "/"))
	if !strings.HasPrefix(clean, "assets/") || !fs.ValidPath(clean) {
		c.Status(http.StatusNotFound)
		return
	}
	content, ok := h.svc.readThemeFile(themeID, clean)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	serveAsset(c, clean, content)
}
