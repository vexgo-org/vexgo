package public

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// defaultThemeFS embeds the built-in theme: Go-template pages (index.html,
// post.html, user.html, 404.html) plus their static assets under assets/.
// It is produced by the standalone vexgo-default-theme build; run
// scripts/fetch-default-theme.sh (or `just build-theme`) before compiling.
//
//go:embed default-theme
var defaultThemeFS embed.FS

// Theme-related constants shared with the settings domain.
const (
	ThemesDir     = "theme"
	FaviconFile   = "favicon.ico"
	DefaultTheme  = "default"
	ThemeMetaFile = "vexgo-theme.json"

	// DefaultFaviconAsset is the admin SPA's bundled glyph, relative to the
	// embedded dist/ directory. A site that configured no icon falls back to
	// it so the public pages show the same mark as the console.
	DefaultFaviconAsset = "assets/vexgo-dark.ico"

	// DevTheme is the id used when a local theme directory is injected via
	// the dev theme workflow (`vexgo dev --theme-dir`). It is not a real
	// installed theme: themeFS resolves it straight from disk, bypassing the
	// data/theme layout and the embedded default theme.
	DevTheme = "dev"
)

// Template file names a theme may provide. Every template is a complete HTML
// document; shared fragments use Go's {{define}}/{{template}} mechanism.
// PageTemplate is the generic fallback for custom pages; any <slug>.html file
// in the theme is additionally usable as the dedicated template of the page
// with that slug (discovered dynamically, not listed here).
const (
	IndexTemplate    = "index.html"
	PostTemplate     = "post.html"
	PageTemplate     = "page.html"
	UserTemplate     = "user.html"
	NotFoundTemplate = "404.html"
)

// ThemeTemplateNames lists the base templates a theme may provide, in the
// order they are parsed into the shared template set. Extra <slug>.html
// files are discovered dynamically by parseThemeTemplates.
var ThemeTemplateNames = []string{IndexTemplate, PostTemplate, PageTemplate, UserTemplate, NotFoundTemplate}

// gold parses post markdown into HTML for server-side rendering. Raw HTML in
// the markdown is escaped (safe defaults, matching the client-side renderer),
// while GFM tables/strikethrough are enabled for parity with remark-gfm.
var gold = goldmark.New(goldmark.WithExtensions(extension.GFM))

// RenderMarkdown converts markdown into safe HTML. The result is typed as
// template.HTML so themes can emit it with {{.ContentHTML}} without Go
// re-escaping it.
func RenderMarkdown(src string) template.HTML {
	var buf bytes.Buffer
	if err := gold.Convert([]byte(src), &buf); err != nil {
		// Rendering failures are extremely unlikely; fall back to escaped
		// plain text so the page still renders something safe.
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(buf.String())
}

// baseTemplateFuncs holds the language-independent helpers. The translatable
// t function is bound per request (see boundTemplateFuncs) so themes declare
// {{t "key"}} without naming a language; the backend resolves it.
func baseTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"date": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
		// add sums two integers; used to render 1-based list positions.
		"add": func(a, b int) int {
			return a + b
		},
		// first clamps a slice to at most n items; unlike the builtin slice it
		// tolerates short/empty inputs (used to cap the tag pills on cards).
		"first": func(items []string, n int) []string {
			if n < 1 || len(items) <= n {
				return items
			}
			return items[:n]
		},
		// truncate caps a string at max runes, appending an ellipsis when it
		// cut. It counts runes so multi-byte text (CJK titles, excerpts) is
		// never cut mid-character on the page. A non-positive max returns the
		// string unchanged, like the sibling first helper.
		"truncate": func(s string, max int) string {
			s = strings.TrimSpace(s)
			if max < 1 {
				return s
			}
			runes := []rune(s)
			if len(runes) <= max {
				return s
			}
			return string(runes[:max]) + "..."
		},
		// userURL builds the public user profile URL. Helper funcs keep attribute
		// actions free of double quotes, which React would escape to &quot; and
		// break Go template parsing.
		"userURL": func(id uint) string {
			return "/user/" + strconv.FormatUint(uint64(id), 10)
		},
		// categoryURL builds the home page category filter URL.
		"categoryURL": func(name string) string {
			return "/?category=" + url.QueryEscape(name)
		},
		// searchURL builds the home page search URL for a tag or keyword.
		"searchURL": func(name string) string {
			return "/?search=" + url.QueryEscape(name)
		},
	}
}

// boundTemplateFuncs copies the base helpers and binds t to one request's
// merged dictionary. Unknown keys fall back to the key itself so
// third-party themes with incomplete translations degrade per key.
func boundTemplateFuncs(dict Dict) template.FuncMap {
	funcs := baseTemplateFuncs()
	funcs["t"] = func(key string) string {
		return dict.translate(key)
	}
	return funcs
}

// languageOf extracts the requested language from template data, defaulting
// to en for data assembled without i18n (tests, legacy callers).
func languageOf(data any) string {
	var site *SiteData
	switch d := data.(type) {
	case IndexData:
		site = d.Site
	case *IndexData:
		if d != nil {
			site = d.Site
		}
	case PostData:
		site = d.Site
	case *PostData:
		if d != nil {
			site = d.Site
		}
	case UserData:
		site = d.Site
	case *UserData:
		if d != nil {
			site = d.Site
		}
	case NotFoundData:
		site = d.Site
	case *NotFoundData:
		if d != nil {
			site = d.Site
		}
	case PageData:
		site = d.Site
	case *PageData:
		if d != nil {
			site = d.Site
		}
	}
	if site == nil {
		return DefaultLanguage
	}
	if lang := NormalizeLanguage(site.Language); lang != "" {
		return lang
	}
	return DefaultLanguage
}

// cachedTheme is a parsed template set plus the newest file mtime it was
// parsed from, so custom themes are re-parsed only when a template changes.
type cachedTheme struct {
	tmpl   *template.Template
	modSum time.Time
}

// themeCacheStore guards the parsed-template cache. Its map is created lazily
// on the first write, so the zero value is usable and no init() function
// ordering is involved.
type themeCacheStore struct {
	sync.Mutex
	themes map[string]cachedTheme
}

// store records a parsed template set, creating the map on first use. Callers
// must hold the mutex.
func (s *themeCacheStore) store(themeID string, entry cachedTheme) {
	if s.themes == nil {
		s.themes = make(map[string]cachedTheme)
	}
	s.themes[themeID] = entry
}

// GetAvailableThemes scans the themes directory and returns a list of available themes.
// Each theme must have a vexgo-theme.json file in its root directory.
// The embedded default theme is always available.
func (s *Service) GetAvailableThemes() []ThemeInfo {
	themes := []ThemeInfo{}

	// Add the default embedded theme
	themes = append(themes, ThemeInfo{
		ID:          DefaultTheme,
		Name:        "vexgo default theme",
		Author:      "vexgo-org",
		Version:     "0.1.0",
		Description: "vexgo default theme",
		URL:         "https://github.com/vexgo-org/vexgo-default-theme",
	})

	// Scan themes directory
	themesPath := filepath.Join(s.dataDir, ThemesDir)
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
func (s *Service) ThemeExists(themeID string) bool {
	if themeID == DefaultTheme {
		return true
	}

	metaPath := filepath.Join(s.dataDir, ThemesDir, themeID, ThemeMetaFile)
	_, err := os.Stat(metaPath)
	return err == nil
}

// ValidateThemeTemplates parses every template the theme provides and
// returns an error when the theme ships no templates or any template fails
// to parse. Activation requires this to succeed so a broken theme can never
// become the site-wide active theme.
func (s *Service) ValidateThemeTemplates(themeID string) error {
	_, err := s.parseThemeTemplates(themeID)
	return err
}

// InvalidateThemeCache drops the parsed-template caches for one theme so the
// next request re-reads it from disk (used after upload/overwrite/delete).
func (s *Service) InvalidateThemeCache(themeID string) {
	s.themeCache.Lock()
	delete(s.themeCache.themes, themeID)
	s.themeCache.Unlock()
	s.themeSourcesCache.Lock()
	delete(s.themeSourcesCache.themes, themeID)
	s.themeSourcesCache.Unlock()
}

// getRequestedTheme determines which theme should be used for this request.
// The ?theme= preview switch is honored only for an authenticated admin (the
// same check that gates draft previews); every other visitor gets the globally
// active theme, so a visitor cannot render a theme that is not active.
func (s *Service) getRequestedTheme(c *gin.Context) string {
	if theme, _, ok := s.previewThemeOverride(c); ok {
		return theme
	}
	if theme := s.activeTheme(c.Request.Context()); theme != "" {
		return theme
	}
	return DefaultTheme
}

// themeFS resolves a theme id to its file system root: the embedded default
// theme for the built-in id, a directory under data/theme for uploaded ones,
// and the injected dev directory for DevTheme (the `vexgo dev --theme-dir`
// workflow). Untrusted ids are rejected before touching the file system.
func (s *Service) themeFS(themeID string) (fs.FS, error) {
	if themeID == DevTheme {
		if s.themeDir == "" {
			return nil, fmt.Errorf("%w: no dev theme directory configured", ErrDevTheme)
		}

		info, err := os.Stat(s.themeDir)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDevTheme, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%w: %q is not a directory", ErrDevTheme, s.themeDir)
		}

		return os.DirFS(s.themeDir), nil
	}
	if themeID == DefaultTheme {
		// The embed keeps the defaulttheme/ directory prefix; expose the
		// theme root so all paths are relative to it.
		return fs.Sub(defaultThemeFS, "default-theme")
	}
	if traversesPath(themeID) {
		return nil, fmt.Errorf("invalid theme id %q", themeID)
	}
	return os.DirFS(filepath.Join(s.dataDir, ThemesDir, themeID)), nil
}

// traversesPath reports whether name could escape a directory root when
// joined to it: it carries a path separator, or it is the "."/".." segment.
func traversesPath(name string) bool {
	hasSeparator := strings.ContainsAny(name, `/\`)
	isDotSegment := name == "." || name == ".."
	return hasSeparator || isDotSegment
}

// readThemeFile reads a single file from a theme, reporting whether it exists.
func (s *Service) readThemeFile(themeID, relPath string) ([]byte, bool) {
	base, err := s.themeFS(themeID)
	if err != nil {
		return nil, false
	}
	clean := path.Clean(strings.TrimPrefix(relPath, "/"))
	if !fs.ValidPath(clean) {
		return nil, false
	}
	content, err := fs.ReadFile(base, clean)
	if err != nil {
		return nil, false
	}
	return content, true
}

// themeSources is the raw template text of a theme, cached so per-request
// parsing (needed to bind t to the visitor language) never touches disk.
type themeSources struct {
	files  map[string]string
	modSum time.Time
}

// themeSourcesStore guards the raw template text cache. Like themeCacheStore,
// it creates its map lazily so the zero value is usable.
type themeSourcesStore struct {
	sync.Mutex
	themes map[string]themeSources
}

// store records the sources of a theme, creating the map on first use. Callers
// must hold the mutex.
func (s *themeSourcesStore) store(themeID string, sources themeSources) {
	if s.themes == nil {
		s.themes = make(map[string]themeSources)
	}
	s.themes[themeID] = sources
}

// loadThemeSources reads every template file the theme provides into memory.
// Besides the base ThemeTemplateNames, any extra <slug>.html file at the
// theme root is a dedicated custom-page template. Custom themes are re-read
// when a template changes on disk; the embedded default theme is read once.
//
// The returned map is the cache's copy: callers must only read it, never
// mutate it, or every subsequent request sees the corruption.
func (s *Service) loadThemeSources(themeID string) (map[string]string, error) {
	s.themeSourcesCache.Lock()
	defer s.themeSourcesCache.Unlock()

	// A dev theme is edited constantly; never cache it, always re-read from
	// disk so a developer sees their changes without restarting the server.
	if themeID == DevTheme {
		return s.readThemeSources(themeID)
	}

	if cached, ok := s.themeSourcesCache.themes[themeID]; ok {
		if themeID == DefaultTheme {
			return cached.files, nil
		}
		if modSum, err := s.templateModSum(themeID); err == nil && modSum.Equal(cached.modSum) {
			return cached.files, nil
		}
	}

	files, err := s.readThemeSources(themeID)
	if err != nil {
		return nil, err
	}

	// A failed mod sum leaves the cache entry unable to match on the next
	// lookup, so the theme is re-parsed every request. That is correct output at
	// a large cost, which is why it is recorded — at debug level, since the
	// signal is cache effectiveness and this sits on the render path.
	modSum, err := s.templateModSum(themeID)
	if err != nil {
		slog.Debug("failed to compute theme mod sum; caching disabled", "theme", themeID, "err", err)
	}
	s.themeSourcesCache.store(themeID, themeSources{files: files, modSum: modSum})
	return files, nil
}

// readThemeSources reads every template file the theme provides into memory,
// bypassing the cache. It is the uncached half of loadThemeSources and is used
// for the dev theme, which is edited constantly and must always reflect the
// current files on disk.
func (s *Service) readThemeSources(themeID string) (map[string]string, error) {
	base, err := s.themeFS(themeID)
	if err != nil {
		return nil, err
	}
	known := map[string]struct{}{}
	for _, name := range ThemeTemplateNames {
		known[name] = struct{}{}
	}
	names := append([]string{}, ThemeTemplateNames...)
	names = append(names, listExtraPageTemplates(base, known)...)
	files := make(map[string]string, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(base, name)
		if err != nil {
			continue
		}
		files[name] = string(content)
	}
	if len(files) == 0 {
		return nil, ErrNoThemeTemplates
	}
	return files, nil
}

// parseThemeSources parses cached sources with one FuncMap so {{define}}
// fragments stay shared across pages. Names are parsed in sorted order so a
// theme's cross-file {{define}} overrides resolve deterministically.
func parseThemeSources(themeID string, files map[string]string, funcs template.FuncMap) (*template.Template, error) {
	tmpl := template.New("theme").Funcs(funcs)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if _, err := tmpl.New(name).Parse(files[name]); err != nil {
			return nil, fmt.Errorf("parse theme %q template %s: %w", themeID, name, err)
		}
	}
	if tmpl.Lookup(IndexTemplate) == nil &&
		tmpl.Lookup(PostTemplate) == nil &&
		tmpl.Lookup(PageTemplate) == nil &&
		tmpl.Lookup(UserTemplate) == nil &&
		tmpl.Lookup(NotFoundTemplate) == nil {
		return nil, ErrNoThemeTemplates
	}
	return tmpl, nil
}

// parseThemeTemplates reads every template file the theme provides into one
// parsed set, so {{define}} fragments can be shared across pages. Besides the
// base ThemeTemplateNames, any extra <slug>.html file at the theme root is
// parsed as a dedicated custom-page template.
//
// The fallback t returns the key itself so legacy cached renders never fail
// on translatable templates; request renders re-parse with the bound dict.
func (s *Service) parseThemeTemplates(themeID string) (*template.Template, error) {
	files, err := s.loadThemeSources(themeID)
	if err != nil {
		return nil, err
	}
	funcs := baseTemplateFuncs()
	funcs["t"] = func(key string) string { return key }
	return parseThemeSources(themeID, files, funcs)
}

// parseThemeWithDict parses a theme with t bound to one request's merged
// dictionary.
func (s *Service) parseThemeWithDict(themeID string, dict Dict) (*template.Template, error) {
	files, err := s.loadThemeSources(themeID)
	if err != nil {
		return nil, err
	}
	return parseThemeSources(themeID, files, boundTemplateFuncs(dict))
}

// listExtraPageTemplates returns theme-root *.html files beyond the known
// base set (e.g. timeline.html, links.html, about.html). Names with slashes,
// dot segments or uppercase are ignored so only safe <slug>.html files qualify.
func listExtraPageTemplates(base fs.FS, known map[string]struct{}) []string {
	entries, err := fs.ReadDir(base, ".")
	if err != nil {
		return nil
	}
	var extra []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := known[name]; ok {
			continue
		}
		if !isPageTemplateFile(name) {
			continue
		}
		slug := strings.TrimSuffix(name, ".html")
		if slug == "" || slug != strings.ToLower(slug) {
			continue
		}
		extra = append(extra, name)
	}
	sort.Strings(extra)
	return extra
}

// isPageTemplateFile reports whether an entry name is a plain, visible .html
// file that can act as a custom page template.
func isPageTemplateFile(name string) bool {
	isHTML := strings.HasSuffix(name, ".html")
	isPlainName := !strings.Contains(name, "/") && !strings.HasPrefix(name, ".")
	return isHTML && isPlainName
}

// themeProvides reports whether the theme supplies the named template.
func (s *Service) themeProvides(themeID, name string) bool {
	files, err := s.loadThemeSources(themeID)
	if err != nil {
		return false
	}
	_, ok := files[name]
	return ok
}

// templateModSum returns the newest modification time of the theme's template
// files, or the zero time when none exist.
func (s *Service) templateModSum(themeID string) (time.Time, error) {
	base, err := s.themeFS(themeID)
	if err != nil {
		return time.Time{}, err
	}
	known := map[string]struct{}{}
	for _, name := range ThemeTemplateNames {
		known[name] = struct{}{}
	}
	names := append([]string{}, ThemeTemplateNames...)
	names = append(names, listExtraPageTemplates(base, known)...)
	var newest time.Time
	for _, name := range names {
		info, err := fs.Stat(base, name)
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, nil
}

// loadTheme returns the parsed template set for a theme, re-parsing custom
// themes whenever one of their template files changes on disk. The embedded
// default theme is parsed once and cached forever.
func (s *Service) loadTheme(themeID string) (*template.Template, error) {
	// A dev theme is edited constantly; never cache it, always re-parse so a
	// developer sees their changes without restarting the server.
	if themeID == DevTheme {
		tmpl, err := s.parseThemeTemplates(themeID)
		if err != nil {
			return nil, err
		}
		return tmpl, nil
	}

	s.themeCache.Lock()
	defer s.themeCache.Unlock()

	if cached, ok := s.themeCache.themes[themeID]; ok {
		if themeID == DefaultTheme {
			return cached.tmpl, nil
		}
		modSum, err := s.templateModSum(themeID)
		if err == nil && modSum.Equal(cached.modSum) {
			return cached.tmpl, nil
		}
	}

	tmpl, err := s.parseThemeTemplates(themeID)
	if err != nil {
		return nil, err
	}
	// See loadThemeSources: a failed mod sum costs a re-parse per request
	// rather than serving a stale template, so it degrades performance only.
	modSum, err := s.templateModSum(themeID)
	if err != nil {
		slog.Debug("failed to compute theme mod sum; caching disabled", "theme", themeID, "err", err)
	}
	s.themeCache.store(themeID, cachedTheme{tmpl: tmpl, modSum: modSum})
	return tmpl, nil
}

// renderTheme renders one page of a theme with the given data. The page name
// is one of the ThemeTemplateNames constants. The language comes from
// data's Site.Language (defaulting to en); t is bound to the theme's merged
// dictionary so translatable templates render without extra plumbing.
func (s *Service) renderTheme(themeID, page string, data any) ([]byte, error) {
	lang := languageOf(data)
	dict := s.loadMergedDict(themeID, lang, lang)
	return s.renderThemeWithDict(themeID, page, data, dict)
}

// renderThemeWithDict renders with an explicitly merged dictionary (used by
// request handlers that resolve the full visitor > site-default > en chain).
func (s *Service) renderThemeWithDict(themeID, page string, data any, dict Dict) ([]byte, error) {
	tmpl, err := s.parseThemeWithDict(themeID, dict)
	if err != nil {
		return nil, err
	}
	t := tmpl.Lookup(page)
	if t == nil {
		return nil, fmt.Errorf("theme %q provides no %s template", themeID, page)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render theme %q page %s: %w", themeID, page, err)
	}
	return buf.Bytes(), nil
}

// friendsBlockRe matches a fenced ```friends code block in page markdown.
// Each non-empty line inside is "name | url | avatar | description".
var friendsBlockRe = regexp.MustCompile("(?m)^```friends[ \t]*\n([\r\\s\\S]*?)^```[ \t]*$")

// renderFriendsCards converts the body of a friends block into card HTML.
// Fields are HTML-escaped; rows without a name or a valid http(s) URL are
// isAbsoluteHTTPURL reports whether raw is a parseable absolute http(s) URL
// with a host, so a friend link can never be a javascript: or relative URL.
func isAbsoluteHTTPURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	isHTTP := u.Scheme == "http" || u.Scheme == "https"
	return isHTTP && u.Host != ""
}

// skipped so one bad line cannot break the whole block.
func renderFriendsCards(body string) string {
	type friend struct {
		name, url, avatar, desc string
	}
	var friends []friend
	for line := range strings.Lines(strings.TrimSpace(body)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		name, rawURL, avatar, desc := parts[0], parts[1], parts[2], parts[3]
		if name == "" || !isAbsoluteHTTPURL(rawURL) {
			continue
		}
		friends = append(friends, friend{name: name, url: rawURL, avatar: avatar, desc: desc})
	}
	if len(friends) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<div class="vexgo-friends">`)
	for _, f := range friends {
		sb.WriteString(`<a class="vexgo-friend-card" href="`)
		sb.WriteString(template.HTMLEscapeString(f.url))
		sb.WriteString(`" target="_blank" rel="noopener">`)
		if f.avatar != "" {
			sb.WriteString(`<img class="vexgo-friend-avatar" src="`)
			sb.WriteString(template.HTMLEscapeString(f.avatar))
			sb.WriteString(`" alt="`)
			sb.WriteString(template.HTMLEscapeString(f.name))
			sb.WriteString(`" loading="lazy">`)
		} else {
			sb.WriteString(`<span class="vexgo-friend-avatar vexgo-friend-initial">`)
			name := []rune(f.name)
			sb.WriteString(template.HTMLEscapeString(string(name[:1])))
			sb.WriteString(`</span>`)
		}
		sb.WriteString(`<span class="vexgo-friend-meta"><span class="vexgo-friend-name">`)
		sb.WriteString(template.HTMLEscapeString(f.name))
		sb.WriteString(`</span>`)
		if f.desc != "" {
			sb.WriteString(`<span class="vexgo-friend-desc">`)
			sb.WriteString(template.HTMLEscapeString(f.desc))
			sb.WriteString(`</span>`)
		}
		sb.WriteString(`</span></a>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// RenderPageContent converts page markdown into safe HTML, expanding fenced
// ```friends blocks into link-card HTML shared by all themes (themes only
// provide the .vexgo-friends CSS). Placeholders survive the markdown pass
// and are swapped for the card HTML afterwards so goldmark never escapes it.
func RenderPageContent(src string) template.HTML {
	cards := []string{}
	withPlaceholders := friendsBlockRe.ReplaceAllStringFunc(src, func(block string) string {
		m := friendsBlockRe.FindStringSubmatch(block)
		body := ""
		if len(m) == 2 {
			body = m[1]
		}
		html := renderFriendsCards(body)
		if html == "" {
			return ""
		}
		cards = append(cards, html)
		return "\n\nVEXGOFRIENDS" + strconv.Itoa(len(cards)-1) + "\n\n"
	})
	var buf bytes.Buffer
	if err := gold.Convert([]byte(withPlaceholders), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	out := buf.String()
	for i, html := range cards {
		out = strings.ReplaceAll(out, "<p>VEXGOFRIENDS"+strconv.Itoa(i)+"</p>", html)
		out = strings.ReplaceAll(out, "VEXGOFRIENDS"+strconv.Itoa(i), html)
	}
	return template.HTML(out)
}

// themeAssetsPrefix is the stable asset prefix every theme template uses to
// reference its own assets; the server resolves it against the active theme.
const themeAssetsPrefix = "/theme-assets/"

// previewHrefRe matches absolute same-origin hyperlinks in rendered HTML so a
// theme preview can keep the previewed theme across navigation.
var previewHrefRe = regexp.MustCompile(`href="(/[^"]*)"`)

// rewriteThemePreview scopes a rendered page to the theme being previewed.
// Themes hardcode the /theme-assets/ prefix, which the server resolves against
// the *active* theme; subresource requests (CSS, JS, images) carry no ?theme=
// parameter, so without rewriting a preview of a non-active theme would load
// the active theme's assets. Preview pages therefore point at the theme's own
// /themes/<id>/assets/ files and carry ?theme=<id> — plus the signature, when
// the preview was opened from a signed link — on their same-origin links so
// navigation inside the preview stays on that theme.
func rewriteThemePreview(out []byte, themeID, previewToken string) []byte {
	assetPrefix := "/themes/" + url.PathEscape(themeID) + "/assets/"
	if bytes.Contains(out, []byte(themeAssetsPrefix)) {
		out = bytes.ReplaceAll(out, []byte(themeAssetsPrefix), []byte(assetPrefix))
	}
	return previewHrefRe.ReplaceAllFunc(out, func(match []byte) []byte {
		href := string(match[len(`href="`) : len(match)-1])
		rewritten, ok := withThemeParam(href, themeID, previewToken)
		if !ok {
			return match
		}
		return []byte(`href="` + rewritten + `"`)
	})
}

// withThemeParam appends ?theme=<themeID> (and the preview signature, when the
// preview carries one) to a same-origin page link, preserving any existing
// query and fragment. It reports ok false for links that must not be rewritten:
// relative links, protocol-relative URLs and non-page prefixes (API, admin SPA,
// raw theme files).
func withThemeParam(href, themeID, previewToken string) (string, bool) {
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") {
		return "", false
	}
	for _, prefix := range []string{"/api/", "/admin", "/themes/", themeAssetsPrefix} {
		if strings.HasPrefix(href, prefix) {
			return "", false
		}
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	q := u.Query()
	q.Set("theme", themeID)
	if previewToken != "" {
		q.Set(ThemePreviewParam, previewToken)
	}
	u.RawQuery = q.Encode()
	return u.String(), true
}
