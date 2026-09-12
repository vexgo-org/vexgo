package public

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newPublicRouter builds a gin engine with the public routes registered and
// an in-memory database seeded with one published post.
func newPublicRouter(t *testing.T) (*gin.Engine, *Renderer, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Tag{},
		&model.Post{},
		&model.Comment{},
		&model.Like{},
		&model.GeneralSettings{},
		&model.ThemeConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	author := model.User{Username: "alice", Email: "alice@example.com", Bio: "writes about go"}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	bob := model.User{Username: "bob", Email: "bob@example.com"}
	carol := model.User{Username: "carol", Email: "carol@example.com"}
	if err := db.Create(&bob).Error; err != nil {
		t.Fatalf("seed bob: %v", err)
	}
	if err := db.Create(&carol).Error; err != nil {
		t.Fatalf("seed carol: %v", err)
	}

	posts := []model.Post{
		{
			Slug:     "hello-world",
			Title:    "Hello World",
			Content:  "**bold** and a [link](/post/hello-world)",
			Excerpt:  "first post",
			Category: "Go",
			AuthorID: author.ID,
			Status:   model.PostStatusPublished,
		},
		{
			Slug:      "popular-post",
			Title:     "Popular Post",
			Content:   "high engagement",
			Category:  "Tech",
			AuthorID:  author.ID,
			Status:    model.PostStatusPublished,
			ViewCount: 100,
		},
		{
			Slug:      "second-post",
			Title:     "Second Post",
			Content:   "less popular",
			AuthorID:  author.ID,
			Status:    model.PostStatusPublished,
			ViewCount: 10,
		},
		{
			Slug:     "draft-post",
			Title:    "Draft",
			Content:  "draft content",
			AuthorID: author.ID,
			Status:   model.PostStatusDraft,
		},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatalf("seed posts: %v", err)
	}

	// Attach tags after creation: batch-inserting posts with inline tag
	// associations makes GORM link the shared "go" tag to only one post.
	tagFor := func(names ...string) []model.Tag {
		var tags []model.Tag
		for _, name := range names {
			tag := model.Tag{Name: name}
			if err := db.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
				t.Fatalf("seed tag %q: %v", name, err)
			}
			tags = append(tags, tag)
		}
		return tags
	}
	if err := db.Model(&posts[0]).Association("Tags").Replace(tagFor("intro")); err != nil {
		t.Fatalf("attach tags hello-world: %v", err)
	}
	if err := db.Model(&posts[1]).Association("Tags").Replace(tagFor("go", "hot")); err != nil {
		t.Fatalf("attach tags popular-post: %v", err)
	}
	if err := db.Model(&posts[2]).Association("Tags").Replace(tagFor("go")); err != nil {
		t.Fatalf("attach tags second-post: %v", err)
	}

	// Popular-post is liked 3 times (score 3*5 + 100 = 115); the others have
	// no likes, so they only rank by raw view count.
	for _, uid := range []uint{author.ID, bob.ID, carol.ID} {
		if err := db.Create(&model.Like{PostID: posts[1].ID, UserID: uid}).Error; err != nil {
			t.Fatalf("seed like: %v", err)
		}
	}

	// One published comment on hello-world by bob.
	if err := db.Create(&model.Comment{
		PostID:  posts[0].ID,
		UserID:  bob.ID,
		Content: "nice post",
		Status:  model.CommentStatusPublished,
	}).Error; err != nil {
		t.Fatalf("seed comment: %v", err)
	}

	r := NewRenderer(db, "http://localhost", t.TempDir())
	e := gin.New()
	r.RegisterStaticRoutes(e, true)
	return e, r, db
}

func doPublicRequest(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPublicRoutes_IndexRendersPublishedPosts(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/")
	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, body: %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"Hello World", "/post/hello-world", "alice"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("index html missing %q:\n%s", want, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "Draft") {
		t.Errorf("index html must not expose draft posts")
	}
	assertNoPlaceholders(t, w.Body.String())
}

func TestPublicRoutes_PostRendersContent(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	for _, path := range []string{"/post/hello-world", "/posts/hello-world"} {
		w := doPublicRequest(t, r, path)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, body: %s", path, w.Code, w.Body.String())
		}
		body := w.Body.String()
		// Markdown must be server-rendered to HTML.
		if !strings.Contains(body, "<strong>bold</strong>") {
			t.Errorf("post content not rendered as markdown:\n%s", body)
		}
		if !strings.Contains(body, "Hello World") || !strings.Contains(body, "Go") {
			t.Errorf("post html missing title/category:\n%s", body)
		}
		assertNoPlaceholders(t, body)
	}
}

func TestPublicRoutes_DraftHiddenAndMissing404(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/post/draft-post")
	if w.Code != http.StatusNotFound {
		t.Fatalf("draft post must be hidden, status = %d", w.Code)
	}
	w = doPublicRequest(t, r, "/post/does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing post status = %d", w.Code)
	}
}

func TestPublicRoutes_UserPage(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/user/1")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /user/1 status = %d, body: %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"alice", "writes about go", "Hello World"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("user html missing %q:\n%s", want, w.Body.String())
		}
	}
	assertNoPlaceholders(t, w.Body.String())

	w = doPublicRequest(t, r, "/user/999")
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d", w.Code)
	}
}

func TestPublicRoutes_HomeSidebarPopularData(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/")
	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()

	// Popular posts sidebar: ranked by likes*5 + views, so popular-post
	// (115) leads second-post (10) and hello-world (0).
	if !strings.Contains(body, "Popular Posts") || !strings.Contains(body, "Popular Tags") {
		t.Errorf("home sidebar missing popular sections:\n%s", body)
	}
	popularIdx := strings.Index(body, "Popular Post")
	secondIdx := strings.Index(body, "Second Post")
	if popularIdx == -1 || secondIdx == -1 || popularIdx > secondIdx {
		t.Errorf("popular posts not ordered by score (popular-post before second-post):\n%s", body)
	}

	// Popular tags: go (2 posts) leads hot/intro (1 each). The count sits in
	// a nested span inside the tag pill, so match the pill start and count.
	if !strings.Contains(body, ">go<span") || !strings.Contains(body, "(2)") {
		t.Errorf("popular tags missing ranked go tag:\n%s", body)
	}
	assertNoPlaceholders(t, body)
}

func TestPublicRoutes_PostPageIncludesCommentWidget(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/post/hello-world")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /post/hello-world status = %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="vexgo-comments"`) ||
		!strings.Contains(body, `data-post-id="1"`) {
		t.Errorf("post page missing comment widget container:\n%s", body)
	}
	if !strings.Contains(body, "/theme-assets/comments.js") {
		t.Errorf("post page missing comment widget script:\n%s", body)
	}
	assertNoPlaceholders(t, body)
}

func TestPopularDataHelpers(t *testing.T) {
	_, renderer, db := newPublicRouter(t)
	ctx := t.Context()

	posts := renderer.popularPostsData(ctx, 5)
	if len(posts) != 3 {
		t.Fatalf("popularPostsData = %d posts, want 3: %+v", len(posts), posts)
	}
	if posts[0].Slug != "popular-post" || posts[1].Slug != "second-post" || posts[2].Slug != "hello-world" {
		t.Errorf("popularPostsData order wrong: %+v", posts)
	}
	if posts[0].CommentsCount != 0 || posts[2].CommentsCount != 1 {
		t.Errorf("popularPostsData comment counts wrong: %+v", posts)
	}
	if posts[0].LikesCount != 3 || posts[1].LikesCount != 0 {
		t.Errorf("popularPostsData like counts wrong: %+v", posts)
	}

	tags := renderer.popularTagsData(ctx, 10)
	if len(tags) != 3 {
		t.Fatalf("popularTagsData = %d tags, want 3: %+v", len(tags), tags)
	}
	if tags[0].Name != "go" || tags[0].Count != 2 {
		t.Errorf("popularTagsData top tag wrong: %+v", tags)
	}

	// A narrower limit trims both lists.
	if got := renderer.popularPostsData(ctx, 1); len(got) != 1 || got[0].Slug != "popular-post" {
		t.Errorf("popularPostsData limit ignored: %+v", got)
	}
	if got := renderer.popularTagsData(ctx, 1); len(got) != 1 || got[0].Name != "go" {
		t.Errorf("popularTagsData limit ignored: %+v", got)
	}

	// Draft posts never surface in either list.
	if err := db.Create(&model.Post{
		Slug: "draft-with-tag", Title: "Draft", Content: "x",
		AuthorID: 1, Status: model.PostStatusDraft,
		Tags: []model.Tag{{Name: "go"}},
	}).Error; err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	if got := renderer.popularPostsData(ctx, 5); len(got) != 3 {
		t.Errorf("draft post leaked into popular posts: %+v", got)
	}
	if got := renderer.popularTagsData(ctx, 10); got[0].Count != 2 {
		t.Errorf("draft tag usage leaked into popular tags: %+v", got)
	}
}

func TestPublicRoutes_SearchFilter(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/?search=hello")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /?search=hello status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Hello World") {
		t.Errorf("search result missing post:\n%s", w.Body.String())
	}

	w = doPublicRequest(t, r, "/?search=nomatch")
	if !strings.Contains(w.Body.String(), "No posts found") {
		t.Errorf("empty search should render the empty state:\n%s", w.Body.String())
	}
}

// installPreviewTheme writes a minimal theme to the renderer's data dir so
// preview tests can request a theme that is not the active one.
func installPreviewTheme(t *testing.T, r *Renderer, themeID string) {
	t.Helper()
	themeDir := filepath.Join(r.DataDir(), ThemesDir, themeID)
	if err := os.MkdirAll(filepath.Join(themeDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	files := map[string]string{
		ThemeMetaFile: `{"id": "` + themeID + `", "name": "Alt", "version": "1.0.0"}`,
		"index.html": `<!doctype html><html><head>` +
			`<link rel="stylesheet" href="/theme-assets/style.css"></head><body>` +
			`<h1>ALT HOME</h1><a href="/">home</a>` +
			`<a href="/post/hello-world">post</a><a href="/?category=Go">cat</a></body></html>`,
		"post.html": `<!doctype html><html><head>` +
			`<link rel="stylesheet" href="/theme-assets/style.css"></head><body>` +
			`<h1>ALT POST</h1><p>{{.Post.Title}}</p></body></html>`,
		filepath.Join("assets", "style.css"): `body{background:#123456} /* alt-theme-css */`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(themeDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write theme file %s: %v", name, err)
		}
	}
	r.InvalidateThemeCache(themeID)
}

// seedPreviewAdmin inserts an admin account and enables preview authentication
// on the renderer, returning a bearer token that authorizes the ?theme= and
// ?preview= preview switches.
func seedPreviewAdmin(t *testing.T, renderer *Renderer, db *gorm.DB) string {
	t.Helper()
	admin := model.User{
		Username:        "previewadmin",
		Email:           "previewadmin@example.com",
		Role:            model.RoleAdmin,
		PasswordVersion: 1,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed preview admin: %v", err)
	}
	renderer.SetJWTSecret(previewTestSecret)
	return previewToken(t, admin.ID, model.RoleAdmin, 1, time.Now())
}

// TestThemePreview_NonActiveThemeScopesAssets reproduces the preview bug: with
// the default theme active, previewing another theme must load that theme's
// CSS (subresource requests carry no ?theme=), not the active theme's.
func TestThemePreview_NonActiveThemeScopesAssets(t *testing.T) {
	r, renderer, db := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")
	token := seedPreviewAdmin(t, renderer, db)

	w := doAuthedGet(t, r, "/?theme=alt", token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /?theme=alt status = %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "ALT HOME") {
		t.Errorf("preview did not render the alt theme template:\n%s", body)
	}
	if !strings.Contains(body, `href="/themes/alt/assets/style.css"`) {
		t.Errorf("preview assets not scoped to the previewed theme:\n%s", body)
	}
	if strings.Contains(body, `href="/theme-assets/style.css"`) {
		t.Errorf("preview still points at the active-theme asset prefix:\n%s", body)
	}

	// The scoped asset URL resolves to the previewed theme's file.
	w = doPublicRequest(t, r, "/themes/alt/assets/style.css")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /themes/alt/assets/style.css status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "alt-theme-css") {
		t.Errorf("alt asset content unexpected:\n%s", w.Body.String())
	}

	// The unscoped prefix still resolves to the active theme, unchanged.
	w = doPublicRequest(t, r, "/theme-assets/style.css")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "alt-theme-css") {
		t.Errorf("active theme asset resolution changed: status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestThemePreview_KeepsThemeAcrossNavigation verifies links inside a preview
// keep the previewed theme when followed.
func TestThemePreview_KeepsThemeAcrossNavigation(t *testing.T) {
	r, renderer, db := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")
	token := seedPreviewAdmin(t, renderer, db)

	w := doAuthedGet(t, r, "/?theme=alt", token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /?theme=alt status = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`href="/post/hello-world?theme=alt"`,
		`href="/?theme=alt"`,
		`href="/?category=Go&theme=alt"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview link %s missing:\n%s", want, body)
		}
	}

	// Following a previewed link renders the alt theme again.
	w = doAuthedGet(t, r, "/post/hello-world?theme=alt", token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /post/hello-world?theme=alt status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ALT POST") ||
		!strings.Contains(w.Body.String(), `href="/themes/alt/assets/style.css"`) {
		t.Errorf("navigated preview did not stay on the alt theme:\n%s", w.Body.String())
	}
}

// TestThemePreview_NoOverrideUnchanged ensures normal requests keep the stable
// /theme-assets/ prefix and do not gain a theme query parameter.
func TestThemePreview_NoOverrideUnchanged(t *testing.T) {
	r, renderer, _ := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")

	w := doPublicRequest(t, r, "/")
	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `href="/theme-assets/style.css"`) {
		t.Errorf("normal page lost the stable asset prefix:\n%s", body)
	}
	if strings.Contains(body, "/themes/alt/") || strings.Contains(body, "?theme=") {
		t.Errorf("normal page was rewritten for preview:\n%s", body)
	}
}

// TestThemePreview_RequiresAdmin is the regression guard for the ?theme=
// switch: it exists for the admin console's preview, so a visitor — anonymous or
// merely signed in — must not be able to render a theme that is not active.
func TestThemePreview_RequiresAdmin(t *testing.T) {
	r, renderer, db := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")

	renderer.SetJWTSecret(previewTestSecret)
	contributor := model.User{
		Username:        "writer",
		Email:           "writer@example.com",
		Role:            model.RoleContributor,
		PasswordVersion: 1,
	}
	if err := db.Create(&contributor).Error; err != nil {
		t.Fatalf("seed contributor: %v", err)
	}

	cases := []struct {
		name  string
		token string
	}{
		{"anonymous", ""},
		{"contributor", previewToken(t, contributor.ID, model.RoleContributor, 1, time.Now())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doAuthedGet(t, r, "/?theme=alt", tc.token)
			if w.Code != http.StatusOK {
				t.Fatalf("GET /?theme=alt status = %d", w.Code)
			}
			body := w.Body.String()
			if strings.Contains(body, "ALT HOME") {
				t.Errorf("non-admin rendered a non-active theme via ?theme=:\n%s", body)
			}
			if strings.Contains(body, "/themes/alt/") {
				t.Errorf("non-admin preview leaked the alt theme asset prefix:\n%s", body)
			}
			if !strings.Contains(body, `href="/theme-assets/style.css"`) {
				t.Errorf("the active theme did not render:\n%s", body)
			}
		})
	}
}

// TestThemeFileRoute_ServesOnlyAssets pins the scope of the public theme file
// route. A preview's subresource requests cannot carry the bearer token, so
// assets stay public; the manifest and templates must not, since the route
// would otherwise expose every theme's sources to anyone who knows its id.
func TestThemeFileRoute_ServesOnlyAssets(t *testing.T) {
	r, renderer, _ := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")

	// Assets are served, including the embedded default theme's.
	for _, path := range []string{"/themes/alt/assets/style.css", "/themes/default/assets/style.css"} {
		if w := doPublicRequest(t, r, path); w.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, w.Code)
		}
	}

	// Sources are not readable.
	for _, path := range []string{"/themes/alt/" + ThemeMetaFile, "/themes/alt/index.html", "/themes/alt/"} {
		if w := doPublicRequest(t, r, path); w.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, w.Code)
		}
	}

	// Traversal out of assets/ collapses to the private path and is refused.
	for _, path := range []string{
		"/themes/alt/assets/../" + ThemeMetaFile,
		"/themes/alt/assets/../../alt/index.html",
	} {
		w := doPublicRequest(t, r, path)
		if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "ALT HOME") {
			t.Errorf("GET %s escaped assets/ and served a theme template", path)
		}
		if strings.Contains(w.Body.String(), `"name": "Alt"`) {
			t.Errorf("GET %s exposed the theme manifest", path)
		}
	}
}

// TestPublicRoutes_UploadsNeverRenderAsDocuments is the regression guard for
// stored XSS through /uploads: a hostile HTML payload must be served as an
// opaque byte stream (or a safe non-document type), never as text/html, and
// path traversal must not escape the media directory.
func TestPublicRoutes_UploadsNeverRenderAsDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	dataDir := t.TempDir()
	mediaDir := filepath.Join(dataDir, "media")
	if err := os.MkdirAll(mediaDir, 0o750); err != nil {
		t.Fatalf("mkdir media: %v", err)
	}

	evil := `<script>alert(document.domain)</script>`
	for name, content := range map[string]string{
		"evil.html": evil,
		"evil.svg":  `<svg onload="alert(1)"></svg>`,
		"deadbeef":  evil, // extensionless file, as a stripped upload becomes
		"pic.png":   "not-really-a-png",
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dataDir, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("seed decoy: %v", err)
	}

	renderer := NewRenderer(db, "http://localhost", dataDir)
	e := gin.New()
	renderer.RegisterStaticRoutes(e, false)

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/uploads/evil.html", "application/octet-stream"},
		{"/uploads/evil.svg", "application/octet-stream"},
		{"/uploads/deadbeef", "application/octet-stream"},
		{"/uploads/pic.png", "image/png"},
	} {
		w := doPublicRequest(t, e, tc.path)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", tc.path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != tc.want {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.path, ct, tc.want)
		}
	}

	// Traversal must not escape the media directory.
	for _, path := range []string{"/uploads/../secret.txt", "/uploads/..%2fsecret.txt"} {
		if w := doPublicRequest(t, e, path); w.Code == http.StatusOK {
			t.Errorf("GET %s served a file outside media (status 200)", path)
		}
	}
}

func TestPublicRoutes_ThemeAssetsServed(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	w := doPublicRequest(t, r, "/theme-assets/style.css")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /theme-assets/style.css status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "background") {
		t.Errorf("style.css content unexpected:\n%.200s", w.Body.String())
	}
}

func TestPublicRoutes_SpaFallbackForAdminPaths(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	// The admin SPA serves every /admin/* path, including /admin itself.
	for _, path := range []string{"/admin", "/admin/login", "/admin/write"} {
		w := doPublicRequest(t, r, path)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, w.Code)
		}
		if !strings.Contains(w.Body.String(), "<div id=\"root\"></div>") {
			t.Errorf("GET %s should serve the SPA index, got:\n%.200s", path, w.Body.String())
		}
	}
	w := doPublicRequest(t, r, "/api/nope")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown api route status = %d", w.Code)
	}
}

func TestPublicRoutes_LegacyPathsRedirectUnderAdmin(t *testing.T) {
	r, _, _ := newPublicRouter(t)
	tests := []struct {
		path string
		want string
	}{
		{"/login", "/admin/login"},
		{"/register", "/admin/register"},
		{"/reset-password", "/admin/reset-password"},
		{"/write", "/admin/write"},
		{"/edit-post/42", "/admin/edit-post/42"},
		{"/verify-email?token=abc-123", "/admin/verify-email?token=abc-123"},
	}
	for _, tt := range tests {
		w := doPublicRequest(t, r, tt.path)
		if w.Code != http.StatusMovedPermanently {
			t.Fatalf("GET %s status = %d, want 301", tt.path, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != tt.want {
			t.Errorf("GET %s Location = %q, want %q", tt.path, loc, tt.want)
		}
	}

	// Unknown non-public paths are plain 404s, not SPA fallbacks.
	w := doPublicRequest(t, r, "/about")
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /about status = %d, want 404", w.Code)
	}
}
