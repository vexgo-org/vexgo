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

	"github.com/google/uuid"
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

// cachedPost is the cache wire shape of a post: an explicit snapshot, never
// model.Post. The cache may be a shared Valkey, so the database row must not
// decide what reaches it. Only guest-visible fields are stored; the author is
// reduced to the attribution a public page shows.
type cachedPost struct {
	ID                 uint             `json:"id"`
	Slug               string           `json:"slug"`
	Title              string           `json:"title"`
	Content            string           `json:"content"`
	Excerpt            string           `json:"excerpt"`
	ViewCount          int              `json:"viewCount"`
	AuthorID           uint             `json:"authorId"`
	Author             cachedAuthor     `json:"author"`
	Category           string           `json:"category"`
	Tags               []cachedTag      `json:"tags"`
	Status             model.PostStatus `json:"status"`
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
	CoverImagePublicID *uuid.UUID       `json:"coverImagePublicID,omitempty"`
}

type cachedAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type cachedTag struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func cachePost(post *model.Post) cachedPost {
	cached := cachedPost{
		ID:        post.ID,
		Slug:      post.Slug,
		Title:     post.Title,
		Content:   post.Content,
		Excerpt:   post.Excerpt,
		ViewCount: post.ViewCount,
		AuthorID:  post.AuthorID,
		Author:    cachedAuthor{ID: post.Author.ID, Username: post.Author.Username},
		Category:  post.Category,
		Status:    post.Status,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
	for _, tag := range post.Tags {
		cached.Tags = append(cached.Tags, cachedTag{ID: tag.ID, Name: tag.Name})
	}
	if post.CoverImage != nil {
		publicID := post.CoverImage.PublicID
		cached.CoverImagePublicID = &publicID
	}
	return cached
}

// post rebuilds the decoded row, restoring the cover asset the response layer
// needs. Only the public ID is restored: it is the sole field URL derivation
// reads.
func (c cachedPost) post() model.Post {
	post := model.Post{
		ID:        c.ID,
		Slug:      c.Slug,
		Title:     c.Title,
		Content:   c.Content,
		Excerpt:   c.Excerpt,
		ViewCount: c.ViewCount,
		AuthorID:  c.AuthorID,
		Author:    model.User{ID: c.Author.ID, Username: c.Author.Username},
		Category:  c.Category,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	for _, tag := range c.Tags {
		post.Tags = append(post.Tags, model.Tag{ID: tag.ID, Name: tag.Name})
	}
	if c.CoverImagePublicID != nil {
		post.CoverImage = &model.Asset{PublicID: *c.CoverImagePublicID}
	}
	return post
}

func cachePosts(posts []model.Post) []cachedPost {
	cached := make([]cachedPost, len(posts))
	for i := range posts {
		cached[i] = cachePost(&posts[i])
	}
	return cached
}

// uncachePosts always returns a non-nil slice, so a decoded null stays an
// empty JSON array rather than null.
func uncachePosts(posts []cachedPost) []model.Post {
	decoded := make([]model.Post, len(posts))
	for i := range posts {
		decoded[i] = posts[i].post()
	}
	return decoded
}

// FindBySlug serves the post by slug through the cache. The stored copy is
// the pristine database row with guest-invisible author data stripped;
// per-request enrichment (like and comment counts) happens in the service on
// a freshly decoded copy. Not-found lookups are never cached.
func (r *cachedRepository) FindBySlug(ctx context.Context, slug string) (*model.Post, error) {
	key := fmt.Sprintf("post:g%s:slug:%s", r.generation(ctx), slug)
	var cached cachedPost
	if r.getJSON(ctx, key, &cached) {
		post := cached.post()
		return &post, nil
	}

	post, err := r.Repository.FindBySlug(ctx, slug)
	if err != nil {
		return post, err
	}
	sanitizeForCache(post)
	r.setJSON(ctx, key, cachePost(post))
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
		Posts []cachedPost
		Total int64
	}
	key := fmt.Sprintf("post:g%s:list:%d:%d:%s", r.generation(ctx), f.Page, f.Limit, listFilterKey(f.CategoryID, f.Status))
	var page listPage
	if r.getJSON(ctx, key, &page) {
		// A decoded null slice must stay an empty JSON array, not null.
		return uncachePosts(page.Posts), page.Total, nil
	}

	posts, total, err := r.Repository.List(ctx, userRole, userID, f)
	if err != nil || len(posts) == 0 {
		return posts, total, err
	}
	for i := range posts {
		sanitizeForCache(&posts[i])
	}
	r.setJSON(ctx, key, listPage{Posts: cachePosts(posts), Total: total})
	return posts, total, nil
}

// cachedPosts is the shared read-through body of the post-pool queries
// (Popular, Latest): on a miss it fetches fresh, drops empty results so keys
// only exist for content that exists, strips guest-invisible author data and
// stores until contentTTL. A decoded null slice comes back as an empty
// slice, never null.
func (r *cachedRepository) cachedPosts(ctx context.Context, key string, fetch func(context.Context) ([]model.Post, error)) ([]model.Post, error) {
	var cached []cachedPost
	if r.getJSON(ctx, key, &cached) {
		return uncachePosts(cached), nil
	}

	posts, err := fetch(ctx)
	if err != nil || len(posts) == 0 {
		return posts, err
	}
	for i := range posts {
		sanitizeForCache(&posts[i])
	}
	r.setJSON(ctx, key, cachePosts(posts))
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
