package public

import (
	"embed"
	"io"
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

// parsePageParam reads the 1-based ?page= query parameter, defaulting to 1.
func parsePageParam(c *gin.Context) int {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// uploadContentTypes maps stored-media extensions to the content type served
// for them. It never maps to a type a browser executes as a script
// (html/xhtml/xml/js are absent), and any unlisted extension falls back to
// application/octet-stream. Because the type is set explicitly before
// http.ServeContent runs, the file server never sniffs the bytes, so an
// uploaded file cannot be rendered as a document on this origin. The upload
// domain keeps its own filename allowlist; an extension it allows but this map
// omits is simply served as an opaque byte stream.
//
// SVG is the deliberate exception: it is a document, so it is served as
// image/svg+xml only together with the isolation policy in serveMediaFile.
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
	".svg":  "image/svg+xml",
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

// Media response hardening. mediaSVGContentType is the one media type a browser
// renders as a document, so navigating straight to an uploaded SVG would
// otherwise run any <script> it carries with the site's own origin — a stored
// XSS. It is therefore served with svgIsolationCSP, which disables scripting
// and same-origin access inside that document. Loaded as an <img>, which is how
// the site icon, avatars and cover images use it, an SVG is static and the
// policy costs nothing.
const (
	mediaSVGContentType = "image/svg+xml"
	svgIsolationCSP     = "default-src 'none'; style-src 'unsafe-inline'; sandbox"
)

// mediaContentType returns the content type served for a stored media file. It
// is derived from the stored name only — never from sniffing the bytes — so a
// hostile upload cannot talk the browser into rendering it as a document.
// Unlisted extensions, including a name whose extension was stripped at upload
// time, are opaque byte streams.
func mediaContentType(name string) string {
	if contentType, ok := uploadContentTypes[strings.ToLower(path.Ext(name))]; ok {
		return contentType
	}
	return "application/octet-stream"
}

// serveMediaFile writes one stored media file with its explicit content type.
// Both the /uploads route and the configured-site-icon path go through here, so
// neither can be tricked into serving an upload as an executable document.
func serveMediaFile(c *gin.Context, name string, info fs.FileInfo, file io.ReadSeeker) {
	contentType := mediaContentType(name)
	c.Header("Content-Type", contentType)
	if contentType == mediaSVGContentType {
		// Append to the baseline policy the security-headers middleware already
		// set, so base-uri, object-src and frame-ancestors keep applying to the
		// SVG document alongside the sandbox.
		policy := c.Writer.Header().Get("Content-Security-Policy")
		if policy == "" {
			policy = svgIsolationCSP
		} else {
			policy += "; " + svgIsolationCSP
		}
		c.Header("Content-Security-Policy", policy)
	}
	http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
}

// uploadHandler serves one local media file. The path is confined to mediaDir
// with os.Root, and the content type comes from uploadContentTypes (never from
// sniffing), so a hostile upload can never be served as an executable document.
func (h *Handler) uploadHandler(mediaDir string) gin.HandlerFunc {
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

		serveMediaFile(c, clean, info, file)
	}
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

// handleFavicon serves /favicon.ico, preferring the configured site icon, then
// ./data/favicon.ico, then the active theme's copy, and finally the bundled
// admin glyph. A site that configured nothing still gets an icon instead of a
// 404, and uploading a site icon switches the public pages and the console
// together because both end up reading this same source.
func (h *Handler) handleFavicon(c *gin.Context) {
	if iconURL := h.svc.SiteIcon(c.Request.Context()); iconURL != "" {
		if h.serveConfiguredIcon(c, iconURL) {
			return
		}
	}

	localFavicon := filepath.Join(h.svc.DataDir(), FaviconFile)
	if _, err := os.Stat(localFavicon); err == nil {
		c.File(localFavicon)
		return
	}

	if content, ok := h.svc.readThemeFile(h.svc.getRequestedTheme(c), FaviconFile); ok {
		c.Data(http.StatusOK, "image/x-icon", content)
		return
	}

	if content, err := readAsset(DefaultFaviconAsset); err == nil {
		c.Data(http.StatusOK, "image/x-icon", content)
		return
	}
	c.Status(http.StatusNotFound)
}

// serveConfiguredIcon serves or redirects the configured site icon and reports
// whether it answered the request. A local upload that is gone from disk
// reports false so the caller can fall back to the other favicon sources.
func (h *Handler) serveConfiguredIcon(c *gin.Context, iconURL string) bool {
	if !strings.HasPrefix(iconURL, "/uploads/") {
		// External or S3 URL: let the browser fetch it directly.
		c.Redirect(http.StatusFound, iconURL)
		return true
	}

	// Serve the upload through serveMediaFile rather than c.File. c.File hands
	// the name to http.ServeFile, which sniffs the bytes when the extension is
	// unknown, so a site icon pointing at an extensionless upload of a hostile
	// HTML file would be served as text/html and execute on direct navigation.
	// The media content types are explicit and never sniffed.
	name := filepath.Base(iconURL)
	root, err := os.OpenRoot(filepath.Join(h.svc.DataDir(), "media"))
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()

	file, err := root.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}

	serveMediaFile(c, name, info, file)
	return true
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
