package home

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ReadCache is the consumer-declared seam for the read cache. It is satisfied
// structurally by the cache backends (internal/cache) without home depending
// on them.
type ReadCache interface {
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// statsTTL bounds how long the cached counters and settings stay valid. The
// writes that change them happen in other domains (posts, users, comments),
// so they cannot be invalidated explicitly here; staleness is bounded by the
// TTL alone.
const statsTTL = 30 * time.Second

// cachedRepository is a read-through cache decorator over the home
// Repository. Cache errors are logged and ignored: reads fall through to the
// database.
type cachedRepository struct {
	Repository
	cache ReadCache
}

// NewCachedRepository wraps repo with the read-through cache decorator. The
// cache must be non-nil.
func NewCachedRepository(repo Repository, cache ReadCache) Repository {
	return &cachedRepository{Repository: repo, cache: cache}
}

// getInt serves a cached counter under key.
func (r *cachedRepository) getInt(ctx context.Context, key string) (int64, bool) {
	value, ok, err := r.cache.Get(ctx, key)
	if err != nil {
		slog.Warn("home cache read failed", "key", key, "err", err)
		return 0, false
	}
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		slog.Warn("home cache decode failed", "key", key, "err", err)
		return 0, false
	}
	return n, true
}

// setInt stores a counter under key until statsTTL elapses.
func (r *cachedRepository) setInt(ctx context.Context, key string, n int64) {
	if err := r.cache.Set(ctx, key, strconv.FormatInt(n, 10), statsTTL); err != nil {
		slog.Warn("home cache write failed", "key", key, "err", err)
	}
}

// counter returns the cached counter for the given name, counting fresh
// through the wrapped repository on a miss.
func (r *cachedRepository) counter(ctx context.Context, name string, count func(context.Context) (int64, error)) int64 {
	key := "home:count:" + name
	if n, ok := r.getInt(ctx, key); ok {
		return n
	}
	n, err := count(ctx)
	if err != nil {
		return n
	}
	r.setInt(ctx, key, n)
	return n
}

func (r *cachedRepository) CountPosts(ctx context.Context) (int64, error) {
	return r.counter(ctx, "posts", r.Repository.CountPosts), nil
}

func (r *cachedRepository) CountUsers(ctx context.Context) (int64, error) {
	return r.counter(ctx, "users", r.Repository.CountUsers), nil
}

func (r *cachedRepository) CountComments(ctx context.Context) (int64, error) {
	return r.counter(ctx, "comments", r.Repository.CountComments), nil
}

func (r *cachedRepository) CountCategories(ctx context.Context) (int64, error) {
	return r.counter(ctx, "categories", r.Repository.CountCategories), nil
}

func (r *cachedRepository) CountTags(ctx context.Context) (int64, error) {
	return r.counter(ctx, "tags", r.Repository.CountTags), nil
}

// GetGeneralSettings serves the general settings row through the cache; it
// gates whether anonymous visitors may read content at all.
func (r *cachedRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	key := "home:settings"
	value, ok, err := r.cache.Get(ctx, key)
	if err == nil && ok {
		var config model.GeneralSettings
		if jsonErr := json.Unmarshal([]byte(value), &config); jsonErr == nil {
			return config, nil
		}
		slog.Warn("home cache decode failed", "key", key, "err", err)
	}

	config, err := r.Repository.GetGeneralSettings(ctx)
	if err != nil {
		return config, err
	}
	if data, jsonErr := json.Marshal(config); jsonErr == nil {
		if setErr := r.cache.Set(ctx, key, string(data), statsTTL); setErr != nil {
			slog.Warn("home cache write failed", "key", key, "err", setErr)
		}
	}
	return config, nil
}
