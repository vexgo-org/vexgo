// Package post implements the post, like, category, tag and post-moderation
// domain.
package post

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the post domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
	Files     FileRemover
	// Cache backs the read-through decorator for the public read paths. nil
	// disables content caching.
	Cache ReadCache
}

// Notifier is the seam for creating notifications; implemented by the notification domain.
// FileRemover deletes a stored file by its public URL; implemented by upload.Storage.
type (
	Notifier    = model.Notifier
	FileRemover = model.FileRemover
)

// Service contains the business logic of the post domain.
type Service struct {
	repo     Repository
	notifier Notifier
	files    FileRemover
}

// NewService creates a post service with the given dependencies.
func NewService(deps Deps) *Service {
	repo := NewRepository(deps.DB)
	if deps.Cache != nil {
		repo = NewCachedRepository(repo, deps.Cache)
	}
	return &Service{repo: repo, notifier: deps.Notifier, files: deps.Files}
}

// allowGuestView reports whether anonymous viewers may see posts.
func (s *Service) allowGuestView(ctx context.Context) bool {
	return s.repo.GetGuestViewSetting(ctx)
}
