// Package home implements the homepage aggregation endpoints (site stats).
package home

import (
	"context"
	"log/slog"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the home domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	// Cache backs the read-through decorator for the aggregate stats. nil
	// disables content caching.
	Cache ReadCache
}

// Service contains the business logic of the home domain.
type Service struct {
	repo Repository
}

// NewService creates a home service with the given dependencies.
func NewService(deps Deps) *Service {
	repo := NewRepository(deps.DB)
	if deps.Cache != nil {
		repo = NewCachedRepository(repo, deps.Cache)
	}
	return &Service{repo: repo}
}

// Stats holds the aggregate site counters returned by /api/stats.
type Stats struct {
	Posts      int64
	Users      int64
	Comments   int64
	Categories int64
	Tags       int64
}

// Stats returns the aggregate site statistics. When the caller is anonymous
// and guest viewing is disabled, all counters are zero.
func (s *Service) Stats(ctx context.Context, userRole string) Stats {
	// Check if guest viewing is allowed
	var allowGuestView bool
	config, err := s.repo.GetGeneralSettings(ctx)
	if err != nil {
		// Default to true if config not found
		allowGuestView = true
	} else {
		allowGuestView = config.AllowGuestViewPosts
	}

	// If not logged in and guest viewing is not allowed, return empty result
	if userRole == "" && !allowGuestView {
		return Stats{}
	}

	// Each counter is independent, so one failing table must not blank the whole
	// dashboard. The failures are logged under one stable message with the
	// counter name as an attribute, keeping log grouping low-cardinality.
	postsCount, err := s.repo.CountPosts(ctx)
	logStatCount("posts", err)
	usersCount, err := s.repo.CountUsers(ctx)
	logStatCount("users", err)
	categoriesCount, err := s.repo.CountCategories(ctx)
	logStatCount("categories", err)
	tagsCount, err := s.repo.CountTags(ctx)
	logStatCount("tags", err)
	commentsCount, err := s.repo.CountComments(ctx)
	logStatCount("comments", err)

	return Stats{
		Posts:      postsCount,
		Users:      usersCount,
		Comments:   commentsCount,
		Categories: categoriesCount,
		Tags:       tagsCount,
	}
}

// logStatCount reports a failed aggregate count. The stat is not worth failing
// the dashboard over, so it falls back to zero — but silently, the cause of a
// permanently zeroed counter would be invisible.
func logStatCount(stat string, err error) {
	if err != nil {
		slog.Warn("failed to count site stat", "stat", stat, "err", err)
	}
}
