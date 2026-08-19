// Package home implements the homepage aggregation endpoints (site stats).
package home

import (
	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the home domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
}

// Service contains the business logic of the home domain.
type Service struct {
	db *gorm.DB
}

// NewService creates a home service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{db: deps.DB}
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
func (s *Service) Stats(userRole string) Stats {
	// Check if guest viewing is allowed
	var allowGuestView bool
	var config model.GeneralSettings
	if err := s.db.First(&config).Error; err != nil {
		// Default to true if config not found
		allowGuestView = true
	} else {
		allowGuestView = config.AllowGuestViewPosts
	}

	// If not logged in and guest viewing is not allowed, return empty result
	if userRole == "" && !allowGuestView {
		return Stats{}
	}

	var postsCount, usersCount, categoriesCount, tagsCount, commentsCount int64
	s.db.Model(&model.Post{}).Count(&postsCount)
	s.db.Model(&model.User{}).Count(&usersCount)
	s.db.Model(&model.Category{}).Count(&categoriesCount)
	s.db.Model(&model.Tag{}).Count(&tagsCount)
	s.db.Model(&model.Comment{}).Count(&commentsCount)

	return Stats{
		Posts:      postsCount,
		Users:      usersCount,
		Comments:   commentsCount,
		Categories: categoriesCount,
		Tags:       tagsCount,
	}
}
