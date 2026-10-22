package public

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// A themes directory that cannot be created would otherwise surface much later
// as a baffling "file not found" during a theme install or preview, so the
// failure has to be reported where it happens.
func TestNewService_ReportsUncreatableThemesDir(t *testing.T) {
	// A regular file where the data directory belongs makes MkdirAll fail.
	blocker := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	// The failure must not stop the service from being constructed.
	svc := NewService(Deps{BaseURL: "http://localhost", DataDir: blocker})
	if svc == nil {
		t.Fatal("expected a service even when the themes directory cannot be created")
	}
	if !strings.Contains(logs.String(), "themes directory") {
		t.Errorf("expected the failed themes directory creation to be logged, got %q", logs.String())
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

// TestBuildPageLinks builds the windowed page-link list: first/last pages
// always present, current ± 1, with ellipsis markers between the gaps.
func TestBuildPageLinks(t *testing.T) {
	urlFor := func(p int) string { return "/?page=" + strconv.Itoa(p) }

	// Single page: no links at all.
	if pages := buildPageLinks(1, 1, urlFor); len(pages) != 0 {
		t.Errorf("single page should have no links, got %+v", pages)
	}

	// Two pages: both linked, current marked.
	pages := buildPageLinks(1, 2, urlFor)
	if len(pages) != 2 || pages[0].Number != 1 || !pages[0].IsCurrent || pages[1].Number != 2 {
		t.Errorf("two-page window wrong: %+v", pages)
	}

	// Middle of a long range: 1 ... 4 5 6 ... 10
	pages = buildPageLinks(5, 10, urlFor)
	want := []struct {
		Number   int
		Ellipsis bool
		IsCur    bool
	}{
		{1, false, false},
		{0, true, false},
		{4, false, false},
		{5, false, true},
		{6, false, false},
		{0, true, false},
		{10, false, false},
	}
	if len(pages) != len(want) {
		t.Fatalf("middle window = %d links, want %d: %+v", len(pages), len(want), pages)
	}
	for i, w := range want {
		if pages[i].Number != w.Number || pages[i].Ellipsis != w.Ellipsis || pages[i].IsCurrent != w.IsCur {
			t.Errorf("link %d = %+v, want %+v", i, pages[i], w)
		}
		if !pages[i].Ellipsis && pages[i].URL != urlFor(pages[i].Number) {
			t.Errorf("link %d url = %q, want %q", i, pages[i].URL, urlFor(pages[i].Number))
		}
	}

	// Early page: 1 2 3 ... 10 (no leading ellipsis)
	pages = buildPageLinks(2, 10, urlFor)
	if len(pages) != 5 || pages[0].Number != 1 || pages[2].Number != 3 ||
		!pages[3].Ellipsis || pages[4].Number != 10 {
		t.Errorf("early window wrong: %+v", pages)
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

// fakeRepository is a Repository stub. It proves the service layer is
// assembled from the persistence interface alone, without a database.
type fakeRepository struct {
	settings   model.GeneralSettings
	posts      []model.Post
	total      int64
	categories []string
	likes      map[uint]int64
	comments   map[uint]int64
	tags       []PopularTag
	navPages   []model.Page
}

func (f *fakeRepository) GetGeneralSettings(context.Context) (model.GeneralSettings, error) {
	return f.settings, nil
}

func (f *fakeRepository) GetThemeConfig(context.Context) (model.ThemeConfig, error) {
	return model.ThemeConfig{}, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ListPublishedPosts(context.Context, PostListQuery) ([]model.Post, int64, error) {
	return f.posts, f.total, nil
}

func (f *fakeRepository) FindPublishedPostBySlug(context.Context, string) (*model.Post, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ListPublishedCategories(context.Context) ([]string, error) {
	return f.categories, nil
}

func (f *fakeRepository) ListRecentPublishedPosts(context.Context, int) ([]model.Post, error) {
	return f.posts, nil
}

func (f *fakeRepository) ListPopularTags(context.Context, int) ([]PopularTag, error) {
	return f.tags, nil
}

func (f *fakeRepository) IncrementPostViews(context.Context, uint) error {
	return nil
}

func (f *fakeRepository) CountLikesByPostIDs(context.Context, []uint) (map[uint]int64, error) {
	return f.likes, nil
}

func (f *fakeRepository) CountCommentsByPostIDs(context.Context, []uint) (map[uint]int64, error) {
	return f.comments, nil
}

func (f *fakeRepository) FindUserByID(context.Context, uint) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) ListNavPages(context.Context) ([]model.Page, error) {
	return f.navPages, nil
}

func (f *fakeRepository) FindPageBySlug(context.Context, string, bool) (*model.Page, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) FindFirstUserByRole(context.Context, string) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

// TestService_IndexViewAssemblesFromRepository locks the layering: the home
// page data is built from the Repository interface alone, with the counts
// batched by the repository instead of one query per post.
func TestService_IndexViewAssemblesFromRepository(t *testing.T) {
	post := model.Post{
		ID:       7,
		Title:    "Hello",
		Slug:     "hello",
		Excerpt:  "hi",
		Category: "Go",
		Status:   model.PostStatusPublished,
		Tags:     []model.Tag{{Name: "go"}},
		Author:   model.User{ID: 3, Username: "alice"},
	}
	repo := &fakeRepository{
		settings:   model.GeneralSettings{SiteName: "VexGo", ItemsPerPage: 20},
		posts:      []model.Post{post},
		total:      1,
		categories: []string{"Go"},
		likes:      map[uint]int64{7: 3},
		comments:   map[uint]int64{7: 2},
		tags:       []PopularTag{{Name: "go", Count: 1}},
		navPages:   []model.Page{{Slug: "about", Title: "About", SortOrder: 1}},
	}
	svc := &Service{repo: repo, baseURL: "http://vexgo.example"}

	site := svc.buildSiteData(t.Context(), "")
	if site.Name != "VexGo" {
		t.Fatalf("site name = %q, want VexGo", site.Name)
	}

	data, err := svc.IndexView(t.Context(), site, PostListQuery{Base: "/", Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("IndexView: %v", err)
	}
	if len(data.Posts) != 1 || data.Posts[0].Title != "Hello" {
		t.Fatalf("posts = %+v", data.Posts)
	}
	if data.Posts[0].LikesCount != 3 || data.Posts[0].CommentsCount != 2 {
		t.Errorf("batch counts wrong: %+v", data.Posts[0])
	}
	if len(data.Categories) != 1 || data.Categories[0] != "Go" {
		t.Errorf("categories = %+v", data.Categories)
	}
	if len(data.Pages) != 1 || data.Pages[0].Slug != "about" {
		t.Errorf("nav pages = %+v", data.Pages)
	}
	if len(data.PopularTags) != 1 || data.PopularTags[0].Name != "go" {
		t.Errorf("popular tags = %+v", data.PopularTags)
	}
	if data.Pagination == nil || data.Pagination.CurrentPage != 1 {
		t.Errorf("pagination = %+v", data.Pagination)
	}
}
