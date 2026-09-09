package public

import (
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func seedTestManifest(t *testing.T) {
	t.Helper()
	old := manifest
	manifest = AssetManifest{
		CSS: map[string]string{"index": "/assets/index-TEST.css"},
		JS: map[string]string{
			"index":        "/assets/index-TEST.js",
			"react-vendor": "/assets/react-vendor-TEST.js",
			"ui-vendor":    "/assets/ui-vendor-TEST.js",
			"utils-vendor": "/assets/utils-vendor-TEST.js",
		},
	}
	t.Cleanup(func() { manifest = old })
}

func assertNoPlaceholders(t *testing.T, html string) {
	t.Helper()
	if strings.Contains(html, "{{") {
		t.Errorf("rendered html contains unrendered placeholder:\n%s", html)
	}
}

func TestRenderPostHTML_Content(t *testing.T) {
	seedTestManifest(t)
	post := model.Post{Slug: "hello-world", Title: "Hello World", Excerpt: "Short summary", Content: "body", CoverImage: "https://cdn.example/cover.png"}

	html, err := RenderPostHTML(post, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderPostHTML error: %v", err)
	}
	got := string(html)
	assertNoPlaceholders(t, got)

	for _, want := range []string{
		"<title>Hello World</title>",
		`<meta name="description" content="Short summary">`,
		`<link rel="canonical" href="https://vexgo.example/posts/hello-world">`,
		`<meta property="og:type" content="article">`,
		`<meta property="og:image" content="https://cdn.example/cover.png">`,
		`"slug":"hello-world"`,
		"/assets/index-TEST.js",
		"/assets/index-TEST.css",
		"/assets/react-vendor-TEST.js",
		"/assets/ui-vendor-TEST.js",
		"/assets/utils-vendor-TEST.js",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered html missing %q", want)
		}
	}
	if n := strings.Count(got, "https://vexgo.example/posts/hello-world"); n != 2 {
		t.Errorf("canonical rendered %d times, want 2 (link + og:url)", n)
	}
}

func TestRenderPostHTML_ExcerptFallback(t *testing.T) {
	seedTestManifest(t)
	long := strings.Repeat("a", 200)
	html, err := RenderPostHTML(model.Post{Slug: "s", Title: "T", Content: long}, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderPostHTML error: %v", err)
	}
	if want := long[:150] + "..."; !strings.Contains(string(html), want) {
		t.Errorf("long content not truncated to 150 chars + ellipsis")
	}

	short := "short body"
	html, err = RenderPostHTML(model.Post{Slug: "s", Title: "T", Content: short}, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderPostHTML error: %v", err)
	}
	if got := string(html); !strings.Contains(got, `content="short body"`) || strings.Contains(got, "short body...") {
		t.Errorf("short content should render as-is without ellipsis:\n%s", got)
	}
}

func TestRenderPostHTML_NoCoverImageOmitsOgImage(t *testing.T) {
	seedTestManifest(t)
	html, err := RenderPostHTML(model.Post{Slug: "s", Title: "T", Excerpt: "e"}, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderPostHTML error: %v", err)
	}
	if strings.Contains(string(html), "og:image") {
		t.Errorf("og:image must be omitted without cover:\n%s", html)
	}
}

func TestRenderPostHTML_EscapesTitle(t *testing.T) {
	seedTestManifest(t)
	html, err := RenderPostHTML(model.Post{Slug: "s", Title: `<script>alert(1)</script>`}, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderPostHTML error: %v", err)
	}
	if got := string(html); strings.Contains(got, "<script>alert(1)</script></title>") {
		t.Errorf("title not escaped:\n%s", got)
	}
}

func TestRenderIndexHTML_Content(t *testing.T) {
	seedTestManifest(t)
	posts := []model.Post{
		{Slug: "first", Title: "First"},
		{Slug: "second", Title: "Second"},
	}
	html, err := RenderIndexHTML(posts, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderIndexHTML error: %v", err)
	}
	got := string(html)
	assertNoPlaceholders(t, got)

	for _, want := range []string{
		"<title>Homepage</title>",
		`<meta name="description" content="Latest post: First, Second, and more">`,
		`<link rel="canonical" href="https://vexgo.example">`,
		`<meta property="og:type" content="website">`,
		"hasPosts: true",
		`"slug":"first"`,
		`"slug":"second"`,
		"/assets/index-TEST.js",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered html missing %q", want)
		}
	}
}

func TestRenderIndexHTML_Empty(t *testing.T) {
	seedTestManifest(t)
	html, err := RenderIndexHTML(nil, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderIndexHTML error: %v", err)
	}
	got := string(html)
	assertNoPlaceholders(t, got)
	if !strings.Contains(got, `content="Latest posts and updates"`) {
		t.Errorf("empty posts should use default meta description:\n%s", got)
	}
	if !strings.Contains(got, "hasPosts: false") {
		t.Errorf("empty posts should render hasPosts false:\n%s", got)
	}
}

func TestRenderIndexHTML_SinglePost(t *testing.T) {
	seedTestManifest(t)
	html, err := RenderIndexHTML([]model.Post{{Slug: "only", Title: "Only"}}, "https://vexgo.example")
	if err != nil {
		t.Fatalf("RenderIndexHTML error: %v", err)
	}
	if got := string(html); !strings.Contains(got, `content="Latest post: Only"`) || strings.Contains(got, "and more") {
		t.Errorf("single post meta description wrong:\n%s", got)
	}
}
