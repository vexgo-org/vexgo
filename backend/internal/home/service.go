// Package home implements the homepage aggregation endpoints (site stats).
package home

import (
	"context"

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

	postsCount, _ := s.repo.CountPosts(ctx)
	usersCount, _ := s.repo.CountUsers(ctx)
	categoriesCount, _ := s.repo.CountCategories(ctx)
	tagsCount, _ := s.repo.CountTags(ctx)
	commentsCount, _ := s.repo.CountComments(ctx)

	return Stats{
		Posts:      postsCount,
		Users:      usersCount,
		Comments:   commentsCount,
		Categories: categoriesCount,
		Tags:       tagsCount,
	}
}
