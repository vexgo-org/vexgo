package public

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// newTestRenderer builds a Renderer without a database (engine tests never
// touch it) and with a temp data dir.
func newTestRenderer(t *testing.T) *Renderer {
	t.Helper()
	return NewRenderer(nil, "http://vexgo.example", t.TempDir())
}

// assertNoPlaceholders fails when the rendered html still carries unrendered
// Go template actions.
func assertNoPlaceholders(t *testing.T, html string) {
	t.Helper()
	if strings.Contains(html, "{{") {
		t.Errorf("rendered html contains unrendered placeholder:\n%s", html)
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
	r := newTestRenderer(t)
	site := &SiteData{Name: "VexGo Test", URL: "http://vexgo.example", ItemsPerPage: 20}

	index, err := r.renderTheme(DefaultTheme, IndexTemplate, IndexData{
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

	post, err := r.renderTheme(DefaultTheme, PostTemplate, PostData{
		Site: site,
	})
	_ = post
	_ = err
	// The Post struct is a plain struct; build it via reflection-free literal.
	postData := PostData{Site: site}
	postData.Post.Title = "Hello <World>"
	postData.Post.ContentHTML = "<p>rendered</p>"
	postData.Post.CreatedAt = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	post, err = r.renderTheme(DefaultTheme, PostTemplate, postData)
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

	user, err := r.renderTheme(DefaultTheme, UserTemplate, UserData{
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

	nf, err := r.renderTheme(DefaultTheme, NotFoundTemplate, NotFoundData{Site: site})
	if err != nil {
		t.Fatalf("render 404: %v", err)
	}
	assertNoPlaceholders(t, string(nf))
}

// TestRenderCustomThemeFromDisk exercises the third-party theme path: a theme
// uploaded to data/theme/<id> is parsed from disk and rendered.
func TestRenderCustomThemeFromDisk(t *testing.T) {
	r := newTestRenderer(t)
	themeDir := filepath.Join(r.dataDir, ThemesDir, "fancy")
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

	out, err := r.renderTheme("fancy", IndexTemplate, IndexData{
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
	r := newTestRenderer(t)
	themeDir := filepath.Join(r.dataDir, ThemesDir, "minimal")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, IndexTemplate), []byte("{{.Site.Name}}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := r.renderTheme("minimal", PostTemplate, PostData{}); err == nil {
		t.Fatal("expected error rendering a page the theme does not provide")
	}
}

// TestRenderTheme_RejectsTraversal ensures hostile theme ids never touch the
// file system outside the themes directory.
func TestRenderTheme_RejectsTraversal(t *testing.T) {
	r := newTestRenderer(t)
	for _, id := range []string{"../evil", "a/b", `a\b`, "..", "."} {
		if _, err := r.loadTheme(id); err == nil {
			t.Errorf("theme id %q should be rejected", id)
		}
	}
}

// TestToPostCard maps a post row to its card shape.
func TestToPostCard(t *testing.T) {
	card := toPostCard(model.Post{
		ID:     7,
		Title:  "T",
		Slug:   "t",
		Tags:   []model.Tag{{Name: "go"}, {Name: "web"}},
		Author: model.User{ID: 3, Username: "alice"},
	})
	if card.URL != "/post/t" || card.AuthorName != "alice" || len(card.Tags) != 2 {
		t.Errorf("card mapping wrong: %+v", card)
	}
}

// TestPageURL preserves filters while building paginated URLs.
func TestPageURL(t *testing.T) {
	cases := []struct {
		base, search, category string
		page                   int
		want                   string
	}{
		{"/", "", "", 2, "/?page=2"},
		{"/", "go", "", 3, "/?page=3&search=go"},
		{"/user/1", "", "tech", 1, "/user/1?category=tech"},
		{"/", "", "", 1, "/"},
	}
	for _, c := range cases {
		if got := pageURL(c.base, c.page, c.search, c.category); got != c.want {
			t.Errorf("pageURL(%q, %d, %q, %q) = %q, want %q", c.base, c.page, c.search, c.category, got, c.want)
		}
	}
}
