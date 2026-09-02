package post

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ReadCache is the consumer-declared seam for the read cache. It is satisfied
// structurally by the cache backends (internal/cache) without post depending
// on them. Invalidation is generation-based (Incr), so no Delete is needed.
type ReadCache interface {
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

const (
	// contentTTL bounds how long a cached read stays valid. It also retires
	// keys of superseded cache generations.
	contentTTL = 30 * time.Second
	// generationKey is the counter whose value namespaces every content key.
	// Each post write increments it, instantly invalidating all cached reads;
	// keys of superseded generations expire with contentTTL.
	generationKey = "post:gen"
)

// cachedRepository is a read-through cache decorator over the post
// Repository. It caches the public, user-independent read paths — the guest
// list queries, a post by slug, popular and latest — and invalidates
// everything on post writes. View-count increments do not invalidate: view
// counts are not content, and their staleness is bounded by contentTTL.
//
// Cache errors are logged and ignored: reads fall through to the database.
type cachedRepository struct {
	Repository
	cache ReadCache
}

// NewCachedRepository wraps repo with the read-through cache decorator. The
// cache must be non-nil.
func NewCachedRepository(repo Repository, cache ReadCache) Repository {
	return &cachedRepository{Repository: repo, cache: cache}
}

// generation returns the current cache generation. A missing or unreadable
// counter reads as the first generation; a later write recreates it.
func (r *cachedRepository) generation(ctx context.Context) string {
	if v, ok, err := r.cache.Get(ctx, generationKey); err == nil && ok {
		return v
	}
	return "0"
}

// bumpGeneration invalidates every cached post read. Reads that raced with
// the bump may still write under the old generation; those keys expire with
// contentTTL.
func (r *cachedRepository) bumpGeneration(ctx context.Context) {
	if _, err := r.cache.Incr(ctx, generationKey, 0); err != nil {
		slog.Warn("post cache invalidation failed", "err", err)
	}
}

// getJSON loads and decodes key into dst, reporting whether usable data was
// found.
func (r *cachedRepository) getJSON(ctx context.Context, key string, dst any) bool {
	value, ok, err := r.cache.Get(ctx, key)
	if err != nil {
		slog.Warn("post cache read failed", "key", key, "err", err)
		return false
	}
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(value), dst); err != nil {
		slog.Warn("post cache decode failed", "key", key, "err", err)
		return false
	}
	return true
}

// setJSON encodes and stores v under key until contentTTL elapses.
func (r *cachedRepository) setJSON(ctx context.Context, key string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Warn("post cache encode failed", "key", key, "err", err)
		return
	}
	if err := r.cache.Set(ctx, key, string(data), contentTTL); err != nil {
		slog.Warn("post cache write failed", "key", key, "err", err)
	}
}

// listFilterKey hashes the free-form filter components (category, status) so
// cache keys stay fixed-size regardless of request input. Searches never
// reach a cache key: List bypasses the cache entirely when a search term is
// present.
func listFilterKey(categoryID, status string) string {
	sum := sha256.Sum256([]byte(categoryID + "\x00" + status))
	return hex.EncodeToString(sum[:8])
}

// sanitizeForCache applies guest-view privacy filtering to a post's author
// before the row enters the cache. The cache backend may be a shared Valkey,
// so it must only ever hold data an anonymous visitor could already see:
// profile fields hidden from guests (email, birthday, bio, private profiles)
// stay in the database. Cached read paths therefore serve the guest-filtered
// author block to every viewer, including the author and admins, on hits and
// misses alike. Account credentials and emailed-link tokens never serialize
// in the first place (json:"-" on model.User).
func sanitizeForCache(posts ...*model.Post) {
	for _, post := range posts {
		if post != nil {
			auth.FilterUserByPrivacy(&post.Author, 0, "")
		}
	}
}

// FindBySlug serves the post by slug through the cache. The stored copy is
// the pristine database row with guest-invisible author data stripped;
// per-request enrichment (like and comment counts) happens in the service on
// a freshly decoded copy. Not-found lookups are never cached.
func (r *cachedRepository) FindBySlug(ctx context.Context, slug string) (*model.Post, error) {
	key := fmt.Sprintf("post:g%s:slug:%s", r.generation(ctx), slug)
	var cached model.Post
	if r.getJSON(ctx, key, &cached) {
		return &cached, nil
	}

	post, err := r.Repository.FindBySlug(ctx, slug)
	if err != nil {
		return post, err
	}
	sanitizeForCache(post)
	r.setJSON(ctx, key, post)
	return post, nil
}

// List serves guest-visible list pages through the cache; staff queries and
// search queries always hit the database. Search is excluded because its
// keys would be attacker-enumerable and unbounded in size, and empty pages
// are not cached so keys only ever exist for real content — together these
// bound the key space to what the posts table actually holds.
func (r *cachedRepository) List(ctx context.Context, userRole string, userID uint, f ListFilter) ([]model.Post, int64, error) {
	isStaff := userRole != "" && userRole != model.RoleGuest
	if isStaff || userID != 0 || f.Search != "" {
		return r.Repository.List(ctx, userRole, userID, f)
	}

	type listPage struct {
		Posts []model.Post
		Total int64
	}
	key := fmt.Sprintf("post:g%s:list:%d:%d:%s", r.generation(ctx), f.Page, f.Limit, listFilterKey(f.CategoryID, f.Status))
	var page listPage
	if r.getJSON(ctx, key, &page) {
		// A decoded null slice must stay an empty JSON array, not null.
		if page.Posts == nil {
			page.Posts = []model.Post{}
		}
		return page.Posts, page.Total, nil
	}

	posts, total, err := r.Repository.List(ctx, userRole, userID, f)
	if err != nil || len(posts) == 0 {
		return posts, total, err
	}
	for i := range posts {
		sanitizeForCache(&posts[i])
	}
	r.setJSON(ctx, key, listPage{Posts: posts, Total: total})
	return posts, total, nil
}

// cachedPosts is the shared read-through body of the post-pool queries
// (Popular, Latest): on a miss it fetches fresh, drops empty results so keys
// only exist for content that exists, strips guest-invisible author data and
// stores until contentTTL. A decoded null slice comes back as an empty
// slice, never null.
func (r *cachedRepository) cachedPosts(ctx context.Context, key string, fetch func(context.Context) ([]model.Post, error)) ([]model.Post, error) {
	var posts []model.Post
	if r.getJSON(ctx, key, &posts) {
		if posts == nil {
			posts = []model.Post{}
		}
		return posts, nil
	}

	posts, err := fetch(ctx)
	if err != nil || len(posts) == 0 {
		return posts, err
	}
	for i := range posts {
		sanitizeForCache(&posts[i])
	}
	r.setJSON(ctx, key, posts)
	return posts, nil
}

// Popular serves the published-posts pool through the cache; scoring,
// sorting and limiting happen in the service. Empty pools are not cached.
func (r *cachedRepository) Popular(ctx context.Context) ([]model.Post, error) {
	return r.cachedPosts(ctx, fmt.Sprintf("post:g%s:popular", r.generation(ctx)), r.Repository.Popular)
}

// Latest serves the most recent published posts through the cache. Empty
// results are not cached.
func (r *cachedRepository) Latest(ctx context.Context, limit int) ([]model.Post, error) {
	return r.cachedPosts(ctx, fmt.Sprintf("post:g%s:latest:%d", r.generation(ctx), limit), func(ctx context.Context) ([]model.Post, error) {
		return r.Repository.Latest(ctx, limit)
	})
}

// Create writes through and invalidates the cached reads.
func (r *cachedRepository) Create(ctx context.Context, post *model.Post) error {
	if err := r.Repository.Create(ctx, post); err != nil {
		return err
	}
	r.bumpGeneration(ctx)
	return nil
}

// Save writes through and invalidates the cached reads.
func (r *cachedRepository) Save(ctx context.Context, post *model.Post) error {
	if err := r.Repository.Save(ctx, post); err != nil {
		return err
	}
	r.bumpGeneration(ctx)
	return nil
}

// Delete writes through and invalidates the cached reads.
func (r *cachedRepository) Delete(ctx context.Context, post *model.Post) error {
	if err := r.Repository.Delete(ctx, post); err != nil {
		return err
	}
	r.bumpGeneration(ctx)
	return nil
}

// IncrementViewCount does not invalidate the cache: view counts are not
// content and their staleness is bounded by contentTTL.
func (r *cachedRepository) IncrementViewCount(ctx context.Context, postID uint) error {
	return r.Repository.IncrementViewCount(ctx, postID)
}
