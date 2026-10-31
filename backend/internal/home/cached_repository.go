package home

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
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
// through the wrapped repository on a miss. Counter failures are logged here
// and read as zero: the stats endpoint deliberately serves partial numbers
// instead of failing (its only caller discards errors), so this log is the
// single handling point of the failure.
func (r *cachedRepository) counter(ctx context.Context, name string, count func(context.Context) (int64, error)) int64 {
	key := "home:count:" + name
	if n, ok := r.getInt(ctx, key); ok {
		return n
	}
	n, err := count(ctx)
	if err != nil {
		slog.Warn("home counter failed", "key", key, "err", err)
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

// cachedSettings is the cache wire shape of the settings row: an explicit
// snapshot, never model.GeneralSettings. The cache may be a shared Valkey, so
// the attached site-icon asset (and its blob storage key) stays out of it.
type cachedSettings struct {
	ID                  uint       `json:"id"`
	CaptchaEnabled      bool       `json:"captchaEnabled"`
	RegistrationEnabled bool       `json:"registrationEnabled"`
	AllowGuestViewPosts bool       `json:"allowGuestViewPosts"`
	SiteName            string     `json:"siteName"`
	SiteDescription     string     `json:"siteDescription"`
	ItemsPerPage        int        `json:"itemsPerPage"`
	SiteLanguage        string     `json:"siteLanguage"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	SiteIconID          *uint      `json:"siteIconId,omitempty"`
	SiteIconPublicID    *uuid.UUID `json:"siteIconPublicID,omitempty"`
}

func cacheSettings(config model.GeneralSettings) cachedSettings {
	cached := cachedSettings{
		ID:                  config.ID,
		CaptchaEnabled:      config.CaptchaEnabled,
		RegistrationEnabled: config.RegistrationEnabled,
		AllowGuestViewPosts: config.AllowGuestViewPosts,
		SiteName:            config.SiteName,
		SiteDescription:     config.SiteDescription,
		ItemsPerPage:        config.ItemsPerPage,
		SiteLanguage:        config.SiteLanguage,
		CreatedAt:           config.CreatedAt,
		UpdatedAt:           config.UpdatedAt,
		SiteIconID:          config.SiteIconID,
	}
	if config.SiteIcon != nil {
		publicID := config.SiteIcon.PublicID
		cached.SiteIconPublicID = &publicID
	}
	return cached
}

func (c cachedSettings) settings() model.GeneralSettings {
	config := model.GeneralSettings{
		ID:                  c.ID,
		CaptchaEnabled:      c.CaptchaEnabled,
		RegistrationEnabled: c.RegistrationEnabled,
		AllowGuestViewPosts: c.AllowGuestViewPosts,
		SiteName:            c.SiteName,
		SiteDescription:     c.SiteDescription,
		ItemsPerPage:        c.ItemsPerPage,
		SiteLanguage:        c.SiteLanguage,
		CreatedAt:           c.CreatedAt,
		UpdatedAt:           c.UpdatedAt,
		SiteIconID:          c.SiteIconID,
	}
	if c.SiteIconPublicID != nil {
		config.SiteIcon = &model.Asset{PublicID: *c.SiteIconPublicID}
	}
	return config
}

// getSettings loads and decodes the cached settings row, reporting whether
// usable data was found. Decode failures are logged here and read as a miss;
// the fall-through re-reads the database and overwrites the bad value.
func (r *cachedRepository) getSettings(ctx context.Context, key string) (model.GeneralSettings, bool) {
	value, ok, err := r.cache.Get(ctx, key)
	if err != nil || !ok {
		return model.GeneralSettings{}, false
	}
	var cached cachedSettings
	if jsonErr := json.Unmarshal([]byte(value), &cached); jsonErr != nil {
		slog.Warn("home cache decode failed", "key", key, "err", jsonErr)
		return model.GeneralSettings{}, false
	}
	return cached.settings(), true
}

// GetGeneralSettings serves the general settings row through the cache; it
// gates whether anonymous visitors may read content at all.
func (r *cachedRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	const key = "home:settings"
	if config, ok := r.getSettings(ctx, key); ok {
		return config, nil
	}

	config, err := r.Repository.GetGeneralSettings(ctx)
	if err != nil {
		return config, err
	}
	if data, jsonErr := json.Marshal(cacheSettings(config)); jsonErr == nil {
		if setErr := r.cache.Set(ctx, key, string(data), statsTTL); setErr != nil {
			slog.Warn("home cache write failed", "key", key, "err", setErr)
		}
	}
	return config, nil
}
