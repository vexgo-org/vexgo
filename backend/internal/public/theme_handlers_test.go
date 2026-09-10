package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
