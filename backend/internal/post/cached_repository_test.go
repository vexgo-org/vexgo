package post

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// fakeRepo is a Repository fake recording calls to the cached methods.
type fakeRepo struct {
	Repository // nil; only the overridden methods may be called

	findBySlugCalls int
	listCalls       int
	popularCalls    int
	latestCalls     int
	saves           int

	post *model.Post
	err  error
}

func (f *fakeRepo) FindBySlug(_ context.Context, _ string) (*model.Post, error) {
	f.findBySlugCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.post, nil
}

func (f *fakeRepo) List(_ context.Context, _ string, _ uint, _ ListFilter) ([]model.Post, int64, error) {
	f.listCalls++
	if f.err != nil {
		return nil, 0, f.err
	}
	if f.post == nil {
		return []model.Post{}, 0, nil
	}
	return []model.Post{*f.post}, 1, nil
}

func (f *fakeRepo) Popular(_ context.Context) ([]model.Post, error) {
	f.popularCalls++
	if f.err != nil {
		return nil, f.err
	}
	return []model.Post{*f.post}, nil
}

func (f *fakeRepo) Latest(_ context.Context, _ int) ([]model.Post, error) {
	f.latestCalls++
	if f.err != nil {
		return nil, f.err
	}
	return []model.Post{*f.post}, nil
}

func (f *fakeRepo) Save(_ context.Context, _ *model.Post) error {
	f.saves++
	return nil
}

func (f *fakeRepo) IncrementViewCount(_ context.Context, _ uint) error { return nil }

// fakeCache is an in-memory ReadCache fake with error injection.
type fakeCache struct {
	values map[string]string
	err    error
}

func (c *fakeCache) Get(_ context.Context, key string) (string, bool, error) {
	if c.err != nil {
		return "", false, c.err
	}
	value, ok := c.values[key]
	return value, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = value
	return nil
}

func (c *fakeCache) Delete(_ context.Context, key string) error { return nil }

func (c *fakeCache) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	n, _ := strconv.ParseInt(c.values[key], 10, 64)
	n++
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = strconv.FormatInt(n, 10)
	return n, nil
}

func newCachedFakeRepo() (*cachedRepository, *fakeRepo, *fakeCache) {
	repo := &fakeRepo{post: &model.Post{Slug: "hello", Title: "Hello"}}
	cache := &fakeCache{}
	return &cachedRepository{Repository: repo, cache: cache}, repo, cache
}

func TestCachedRepository_FindBySlugCaches(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()

	first, err := r.FindBySlug(ctx, "hello")
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	second, err := r.FindBySlug(ctx, "hello")
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if repo.findBySlugCalls != 1 {
		t.Fatalf("expected 1 underlying FindBySlug call, got %d", repo.findBySlugCalls)
	}
	// Cached copies must be distinct objects: per-request enrichment mutates
	// the post without corrupting the cache.
	if first == second {
		t.Fatal("expected distinct post objects for the two reads")
	}
	if second.Slug != "hello" || second.Title != "Hello" {
		t.Fatalf("cached copy lost its data: %+v", second)
	}
}

func TestCachedRepository_WritesInvalidate(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()

	if _, err := r.FindBySlug(ctx, "hello"); err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if err := r.Save(ctx, repo.post); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := r.FindBySlug(ctx, "hello"); err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if repo.findBySlugCalls != 2 {
		t.Fatalf("expected the write to invalidate the cache, underlying calls = %d", repo.findBySlugCalls)
	}
}

func TestCachedRepository_IncrementViewCountDoesNotInvalidate(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()

	if _, err := r.FindBySlug(ctx, "hello"); err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if err := r.IncrementViewCount(ctx, 1); err != nil {
		t.Fatalf("IncrementViewCount: %v", err)
	}
	if _, err := r.FindBySlug(ctx, "hello"); err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if repo.findBySlugCalls != 1 {
		t.Fatalf("expected the view count to keep the cache, underlying calls = %d", repo.findBySlugCalls)
	}
}

func TestCachedRepository_ListCachesGuestOnly(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()

	for range 2 {
		if _, _, err := r.List(ctx, "", 0, ListFilter{Page: 1, Limit: 10}); err != nil {
			t.Fatalf("guest List: %v", err)
		}
	}
	if repo.listCalls != 1 {
		t.Fatalf("expected guest lists to be cached, underlying calls = %d", repo.listCalls)
	}

	if _, _, err := r.List(ctx, model.RoleAdmin, 7, ListFilter{Page: 1, Limit: 10}); err != nil {
		t.Fatalf("admin List: %v", err)
	}
	if _, _, err := r.List(ctx, model.RoleAdmin, 7, ListFilter{Page: 1, Limit: 10}); err != nil {
		t.Fatalf("admin List: %v", err)
	}
	if repo.listCalls != 3 {
		t.Fatalf("expected authenticated lists to bypass the cache, underlying calls = %d", repo.listCalls)
	}
}

func TestCachedRepository_EmptyGuestListNotCached(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()
	repo.post = nil

	// Empty pages are never cached: every request for a page beyond the
	// content re-queries the database, so crafted page numbers cannot grow
	// the key space.
	for range 2 {
		posts, total, err := r.List(ctx, "", 0, ListFilter{Page: 99, Limit: 10})
		if err != nil || total != 0 || len(posts) != 0 {
			t.Fatalf("page = %d posts, total %d, %v; want empty", len(posts), total, err)
		}
	}
	if repo.listCalls != 2 {
		t.Fatalf("expected 2 underlying calls for empty pages, got %d", repo.listCalls)
	}
}

// TestCachedRepository_SearchBypassesCache checks that search queries always
// hit the database: their keys would be attacker-enumerable and unbounded.
func TestCachedRepository_SearchBypassesCache(t *testing.T) {
	ctx := context.Background()
	r, repo, cache := newCachedFakeRepo()

	for range 2 {
		if _, _, err := r.List(ctx, "", 0, ListFilter{Page: 1, Limit: 10, Search: "hello"}); err != nil {
			t.Fatalf("search List: %v", err)
		}
	}
	if repo.listCalls != 2 {
		t.Fatalf("expected every search to hit the database, underlying calls = %d", repo.listCalls)
	}
	for key := range cache.values {
		if strings.Contains(key, "list:") {
			t.Fatalf("search term leaked into a list cache key: %q", key)
		}
	}
}

// TestCachedRepository_ListKeyHashesFilters checks that the free-form filter
// components are hashed, keeping keys fixed-size.
func TestCachedRepository_ListKeyHashesFilters(t *testing.T) {
	ctx := context.Background()
	r, _, cache := newCachedFakeRepo()

	if _, _, err := r.List(ctx, "", 0, ListFilter{Page: 1, Limit: 10, CategoryID: "golang"}); err != nil {
		t.Fatalf("List: %v", err)
	}
	for key := range cache.values {
		if strings.Contains(key, "golang") {
			t.Fatalf("raw category leaked into a list cache key: %q", key)
		}
	}
}

func TestCachedRepository_PopularAndLatestCache(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()

	for range 2 {
		if _, err := r.Popular(ctx); err != nil {
			t.Fatalf("Popular: %v", err)
		}
		if _, err := r.Latest(ctx, 5); err != nil {
			t.Fatalf("Latest: %v", err)
		}
	}
	if repo.popularCalls != 1 || repo.latestCalls != 1 {
		t.Fatalf("expected cached Popular/Latest, got %d/%d underlying calls", repo.popularCalls, repo.latestCalls)
	}
}

func TestCachedRepository_CacheErrorFallsThroughToDB(t *testing.T) {
	ctx := context.Background()
	r, repo, cache := newCachedFakeRepo()
	cache.err = context.DeadlineExceeded

	for range 2 {
		if _, err := r.FindBySlug(ctx, "hello"); err != nil {
			t.Fatalf("FindBySlug: %v", err)
		}
	}
	if repo.findBySlugCalls != 2 {
		t.Fatalf("expected every read to hit the database on cache errors, got %d", repo.findBySlugCalls)
	}
}

func TestCachedRepository_RepoErrorNotCached(t *testing.T) {
	ctx := context.Background()
	r, repo, _ := newCachedFakeRepo()
	repo.err = errors.New("db down")

	if _, err := r.FindBySlug(ctx, "hello"); err == nil {
		t.Fatal("expected the repository error to propagate")
	}
	repo.err = nil
	if _, err := r.FindBySlug(ctx, "hello"); err != nil {
		t.Fatalf("FindBySlug after recovery: %v", err)
	}
	if repo.findBySlugCalls != 2 {
		t.Fatalf("expected the failed lookup not to be cached, underlying calls = %d", repo.findBySlugCalls)
	}
}
