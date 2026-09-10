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
		&model.GeneralSettings{},
		&model.ThemeConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	author := model.User{Username: "alice", Email: "alice@example.com", Bio: "writes about go"}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
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
			Tags:     []model.Tag{{Name: "intro"}},
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
	// Non-public routes still serve the admin SPA until they move under /admin.
	w := doPublicRequest(t, r, "/login")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /login status = %d", w.Code)
	}
	w = doPublicRequest(t, r, "/admin")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /admin status = %d", w.Code)
	}
	w = doPublicRequest(t, r, "/api/nope")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown api route status = %d", w.Code)
	}
}
