package public

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPageTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Page{}, &model.Post{}, &model.GeneralSettings{}, &model.ThemeConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin := model.User{Username: "admin", Email: "admin@example.com", Role: model.RoleAdmin}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	pages := []model.Page{
		{Slug: "about", Title: "About", Content: "Hello **world**", ShowInNav: true, SortOrder: 1, Status: model.PageStatusPublished, AuthorID: admin.ID},
		{Slug: "secret", Title: "Secret", Content: "draft", Status: model.PageStatusDraft, AuthorID: admin.ID},
	}
	if err := db.Create(&pages).Error; err != nil {
		t.Fatalf("seed pages: %v", err)
	}
	r := NewRenderer(db, "http://localhost", t.TempDir())
	e := gin.New()
	r.RegisterStaticRoutes(e, true)
	return e
}

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

func TestPageRoutes_PublishedRendersDraftHidden(t *testing.T) {
	e := newPageTestRouter(t)

	w := doPublicRequest(t, e, "/about")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /about status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "About") {
		t.Errorf("page html missing title:\n%s", w.Body.String())
	}

	w = doPublicRequest(t, e, "/secret")
	if w.Code != http.StatusNotFound {
		t.Errorf("draft page should 404 for guests, got %d", w.Code)
	}

	w = doPublicRequest(t, e, "/no-such-page")
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown slug should 404, got %d", w.Code)
	}

	// System routes keep precedence over the /:slug catch-all.
	w = doPublicRequest(t, e, "/admin")
	if w.Code != http.StatusOK {
		t.Errorf("GET /admin should serve SPA, got %d", w.Code)
	}
}

func TestPageNav_AppearsInIndex(t *testing.T) {
	e := newPageTestRouter(t)
	w := doPublicRequest(t, e, "/")
	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `href="/about"`) {
		t.Errorf("index should link nav page /about:\n%s", w.Body.String())
	}
}
