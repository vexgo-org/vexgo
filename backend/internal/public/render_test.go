package public

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// newTestService builds a public service without a database (engine tests
// never touch it) and with a temp data dir.
func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Deps{BaseURL: "http://vexgo.example", DataDir: t.TempDir()})
}

// assertNoPlaceholders fails when the rendered html still carries unrendered
// Go template actions.
func assertNoPlaceholders(t *testing.T, html string) {
	t.Helper()
	if strings.Contains(html, "{{") {
		t.Errorf("rendered html contains unrendered placeholder:\n%s", html)
	}
}

// The truncate template helper counts runes: a byte cap cut multi-byte text
// mid-character and rendered a replacement character on the page.
func TestBaseTemplateFuncs_TruncateCountsRunes(t *testing.T) {
	truncate, ok := baseTemplateFuncs()["truncate"].(func(string, int) string)
	if !ok {
		t.Fatal("truncate helper missing or has an unexpected signature")
	}

	// ASCII behavior is unchanged from the byte-based version.
	if got := truncate("abcdef", 3); got != "abc..." {
		t.Errorf("truncate ASCII = %q, want %q", got, "abc...")
	}
	if got := truncate("abc", 3); got != "abc" {
		t.Errorf("truncate at the cap = %q, want the string unchanged", got)
	}
	if got := truncate("  padded  ", 10); got != "padded" {
		t.Errorf("truncate should trim space first, got %q", got)
	}

	// Multi-byte text is cut on a rune boundary, not a byte boundary.
	got := truncate(strings.Repeat("评", 4), 2)
	want := strings.Repeat("评", 2) + "..."
	if got != want {
		t.Errorf("truncate multi-byte = %q, want %q", got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("truncate produced invalid UTF-8: %q", got)
	}

	// A non-positive cap must not panic (the byte slice did).
	for _, max := range []int{0, -1} {
		if got := truncate("abc", max); got != "abc" {
			t.Errorf("truncate(max=%d) = %q, want the string unchanged", max, got)
		}
	}
}

func TestRenderMarkdown_GFM(t *testing.T) {
	out := RenderMarkdown("| a | b |\n|---|---|\n| 1 | 2 |\n\n~~strike~~ **bold**")
	if !strings.Contains(string(out), "<table>") {
		t.Errorf("GFM table not rendered: %s", out)
	}
	if !strings.Contains(string(out), "<del>strike</del>") {
		t.Errorf("GFM strikethrough not rendered: %s", out)
	}
	if !strings.Contains(string(out), "<strong>bold</strong>") {
		t.Errorf("bold not rendered: %s", out)
	}
}

func TestRenderMarkdown_EscapesRawHTML(t *testing.T) {
	// Raw HTML in post content must never reach the page as executable HTML:
	// goldmark's safe mode omits it entirely.
	out := RenderMarkdown("hello <script>alert(1)</script>")
	if strings.Contains(string(out), "<script>alert(1)</script>") {
		t.Errorf("raw HTML leaked through: %s", out)
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	if out := RenderMarkdown(""); out != "" {
		t.Errorf("empty markdown should render empty, got %q", out)
	}
}

func TestRenderMarkdown_Paragraph(t *testing.T) {
	out := RenderMarkdown("plain text")
	if !strings.Contains(string(out), "<p>plain text</p>") {
		t.Errorf("paragraph not rendered: %s", out)
	}
}

// TestRenderDefaultThemeTemplates guards the built-in theme: every page must
// render without leftover {{ }} placeholders and with the injected data.
func TestRenderDefaultThemeTemplates(t *testing.T) {
	s := newTestService(t)
	site := &SiteData{Name: "VexGo Test", URL: "http://vexgo.example", ItemsPerPage: 20}

	index, err := s.renderTheme(DefaultTheme, IndexTemplate, IndexData{
		Site:  site,
		Posts: []PostCardData{{Title: "First Post", URL: "/post/first", Excerpt: "excerpt"}},
		Pagination: &PaginationData{
			CurrentPage: 1,
			TotalPages:  2,
			HasNext:     true,
			NextURL:     "/?page=2",
		},
		Query:      IndexQueryData{Search: "hello"},
		Categories: []string{"Go"},
	})
	if err != nil {
		t.Fatalf("render index: %v", err)
	}
	assertNoPlaceholders(t, string(index))
	for _, want := range []string{"VexGo Test", "First Post", "/post/first", "/?page=2", "Go", "hello"} {
		if !strings.Contains(string(index), want) {
			t.Errorf("index html missing %q:\n%s", want, index)
		}
	}

	post, err := s.renderTheme(DefaultTheme, PostTemplate, PostData{
		Site: site,
	})
	_ = post
	_ = err
	// The Post struct is a plain struct; build it via reflection-free literal.
	postData := PostData{Site: site}
	postData.Post.Title = "Hello <World>"
	postData.Post.ContentHTML = "<p>rendered</p>"
	postData.Post.CreatedAt = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	post, err = s.renderTheme(DefaultTheme, PostTemplate, postData)
	if err != nil {
		t.Fatalf("render post: %v", err)
	}
	assertNoPlaceholders(t, string(post))
	// Title must be HTML-escaped, content must be raw.
	if !strings.Contains(string(post), "Hello &lt;World&gt;") {
		t.Errorf("post title not escaped:\n%s", post)
	}
	if !strings.Contains(string(post), "<p>rendered</p>") {
		t.Errorf("post content not rendered as HTML:\n%s", post)
	}

	user, err := s.renderTheme(DefaultTheme, UserTemplate, UserData{
		Site: site,
		User: struct {
			ID         uint
			Username   string
			Avatar     string
			Bio        string
			CreatedAt  time.Time
			PostsCount int64
		}{Username: "author", Bio: "bio", PostsCount: 3},
		Pagination: &PaginationData{CurrentPage: 1, TotalPages: 1},
	})
	if err != nil {
		t.Fatalf("render user: %v", err)
	}
	assertNoPlaceholders(t, string(user))
	for _, want := range []string{"author", "bio"} {
		if !strings.Contains(string(user), want) {
			t.Errorf("user html missing %q:\n%s", want, user)
		}
	}

	nf, err := s.renderTheme(DefaultTheme, NotFoundTemplate, NotFoundData{Site: site})
	if err != nil {
		t.Fatalf("render 404: %v", err)
	}
	assertNoPlaceholders(t, string(nf))
}

// TestRenderCustomThemeFromDisk exercises the third-party theme path: a theme
// uploaded to data/theme/<id> is parsed from disk and rendered.
func TestRenderCustomThemeFromDisk(t *testing.T) {
	s := newTestService(t)
	themeDir := filepath.Join(s.dataDir, ThemesDir, "fancy")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	indexHTML := `<!DOCTYPE html><html><head><title>{{.Site.Name}}</title></head>
<body>
{{range .Posts}}<a href="{{.URL}}">{{.Title}}</a>{{end}}
</body></html>`
	if err := os.WriteFile(filepath.Join(themeDir, IndexTemplate), []byte(indexHTML), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := s.renderTheme("fancy", IndexTemplate, IndexData{
		Site:  &SiteData{Name: "Fancy Site"},
		Posts: []PostCardData{{Title: "Post A", URL: "/post/a"}, {Title: "Post B", URL: "/post/b"}},
	})
	if err != nil {
		t.Fatalf("render custom theme: %v", err)
	}
	assertNoPlaceholders(t, string(out))
	for _, want := range []string{"Fancy Site", `<a href="/post/a">Post A</a>`, `<a href="/post/b">Post B</a>`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("custom theme html missing %q:\n%s", want, out)
		}
	}
}

// TestRenderCustomTheme_MissingPage ensures asking for a page the theme does
// not provide is an error, which handlers turn into a 404.
func TestRenderCustomTheme_MissingPage(t *testing.T) {
	s := newTestService(t)
	themeDir := filepath.Join(s.dataDir, ThemesDir, "minimal")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, IndexTemplate), []byte("{{.Site.Name}}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.renderTheme("minimal", PostTemplate, PostData{}); err == nil {
		t.Fatal("expected error rendering a page the theme does not provide")
	}
}

// TestDevThemeDirOverridesActiveTheme guards the `vexgo dev --theme-dir`
// workflow: an injected local directory wins over the database's active theme
// and is re-read from disk on every request so developer edits are served
// without a restart.
func TestDevThemeDirOverridesActiveTheme(t *testing.T) {
	devDir := filepath.Join(t.TempDir(), "dev-theme")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, IndexTemplate), []byte("DEV {{.Site.Name}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewService(Deps{BaseURL: "http://vexgo.example", DataDir: t.TempDir(), ThemeDir: devDir})

	// The dev directory wins over the configured data/theme layout.
	if got := s.activeTheme(t.Context()); got != DevTheme {
		t.Fatalf("active theme should be DevTheme, got %q", got)
	}

	out, err := s.renderTheme(s.activeTheme(t.Context()), IndexTemplate, IndexData{
		Site: &SiteData{Name: "Dev Site"},
	})
	if err != nil {
		t.Fatalf("render dev theme: %v", err)
	}
	if !strings.Contains(string(out), "DEV Dev Site") {
		t.Errorf("dev theme should render the local template, got %q", out)
	}
}

// TestDevThemeDirReflectsEdits guards the no-cache contract: editing the dev
// theme's template on disk is visible to the next render without restarting
// the server or invalidating any cache.
func TestDevThemeDirReflectsEdits(t *testing.T) {
	devDir := filepath.Join(t.TempDir(), "dev-theme")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, IndexTemplate), []byte("v1 {{.Site.Name}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewService(Deps{BaseURL: "http://vexgo.example", DataDir: t.TempDir(), ThemeDir: devDir})

	out, err := s.renderTheme(DevTheme, IndexTemplate, IndexData{Site: &SiteData{Name: "S"}})
	if err != nil {
		t.Fatalf("render v1: %v", err)
	}
	if !strings.Contains(string(out), "v1 S") {
		t.Errorf("expected v1 render, got %q", out)
	}

	// Rewrite the template on disk; the next render must pick it up.
	if err := os.WriteFile(filepath.Join(devDir, IndexTemplate), []byte("v2 {{.Site.Name}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = s.renderTheme(DevTheme, IndexTemplate, IndexData{Site: &SiteData{Name: "S"}})
	if err != nil {
		t.Fatalf("render v2: %v", err)
	}
	if !strings.Contains(string(out), "v2 S") {
		t.Errorf("expected v2 render after edit, got %q", out)
	}
}

// TestDevThemeDirMissingTemplate reports ErrNoThemeTemplates when the injected
// directory ships no templates, so a misconfigured --theme-dir fails loudly
// instead of rendering the default theme silently.
func TestDevThemeDirMissingTemplate(t *testing.T) {
	devDir := filepath.Join(t.TempDir(), "empty-theme")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewService(Deps{BaseURL: "http://vexgo.example", DataDir: t.TempDir(), ThemeDir: devDir})

	if _, err := s.renderTheme(DevTheme, IndexTemplate, IndexData{}); !errors.Is(err, ErrNoThemeTemplates) {
		t.Fatalf("expected ErrNoThemeTemplates, got %v", err)
	}
}

// TestDevThemeDirNotExist rejects a --theme-dir that points at a path which
// does not exist, so a typo fails loudly instead of falling through to the
// embedded default theme or exposing whatever the process can read. It matches
// fs.ErrNotExist rather than the OS wording, which differs between Unix
// ("no such file or directory") and Windows ("The system cannot find the file
// specified.").
func TestDevThemeDirNotExist(t *testing.T) {
	s := NewService(Deps{
		BaseURL:  "http://vexgo.example",
		DataDir:  t.TempDir(),
		ThemeDir: filepath.Join(t.TempDir(), "no-such-theme"),
	})

	if _, err := s.themeFS(DevTheme); err == nil {
		t.Fatal("themeFS should reject a non-existent dev theme dir")
	} else if !errors.Is(err, ErrDevTheme) {
		t.Fatalf("expected the error to wrap ErrDevTheme, got %v", err)
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected a 'does not exist' error, got %v", err)
	}
}

// TestRenderTheme_RejectsTraversal ensures hostile theme ids never touch the
// file system outside the themes directory.
func TestRenderTheme_RejectsTraversal(t *testing.T) {
	s := newTestService(t)
	for _, id := range []string{"../evil", "a/b", `a\b`, "..", "."} {
		if _, err := s.loadTheme(id); err == nil {
			t.Errorf("theme id %q should be rejected", id)
		}
	}
}

// TestThemeCachesAreUsableWithoutInitialization pins the zero-value contract of
// the two theme caches. Each map is created lazily on its first write, so a
// process that never runs an initializer still renders a theme instead of
// panicking on a nil map write. The maps are reset to nil here to exercise
// exactly that path.
func TestThemeCachesAreUsableWithoutInitialization(t *testing.T) {
	s := newTestService(t)

	s.themeCache.Lock()
	s.themeCache.themes = nil
	s.themeCache.Unlock()

	s.themeSourcesCache.Lock()
	s.themeSourcesCache.themes = nil
	s.themeSourcesCache.Unlock()

	if _, err := s.loadTheme(DefaultTheme); err != nil {
		t.Fatalf("loadTheme on an uninitialized cache: %v", err)
	}
	if _, err := s.loadThemeSources(DefaultTheme); err != nil {
		t.Fatalf("loadThemeSources on an uninitialized cache: %v", err)
	}

	// The first write must not only avoid panicking but also populate the map,
	// or every request re-parses the theme.
	s.themeCache.Lock()
	_, cachedTemplate := s.themeCache.themes[DefaultTheme]
	s.themeCache.Unlock()
	if !cachedTemplate {
		t.Error("loadTheme did not cache the parsed template set")
	}

	s.themeSourcesCache.Lock()
	_, cachedSources := s.themeSourcesCache.themes[DefaultTheme]
	s.themeSourcesCache.Unlock()
	if !cachedSources {
		t.Error("loadThemeSources did not cache the theme sources")
	}
}

// TestRenderPageContent_FriendsBlock pins the shared friends-block rendering:
// the fenced block becomes link cards, and every field is escaped.
func TestRenderPageContent_FriendsBlock(t *testing.T) {
	src := "Intro.\n\n```friends\nVexGo | https://example.com | https://example.com/a.png | A CMS\nBad | not-a-url |  | skipped\n```\n"
	out := string(RenderPageContent(src))
	if !strings.Contains(out, "vexgo-friends") {
		t.Errorf("friends block not expanded:\n%s", out)
	}
	if !strings.Contains(out, "https://example.com") {
		t.Errorf("friend url missing:\n%s", out)
	}
	if strings.Contains(out, "not-a-url") {
		t.Errorf("invalid url should be skipped:\n%s", out)
	}
	if strings.Contains(out, "```friends") {
		t.Errorf("fence should be removed:\n%s", out)
	}

	xss := "```friends\n<script> | https://example.com |  | <img src=x onerror=1>\n```\n"
	out = string(RenderPageContent(xss))
	if strings.Contains(out, "<script>") || strings.Contains(out, "onerror=1\">") {
		t.Errorf("friend fields must be escaped:\n%s", out)
	}
}
