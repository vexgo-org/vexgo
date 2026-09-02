// Package post implements the post, like, category, tag and post-moderation
// domain.
package post

import (
	"context"
	"errors"
	"fmt"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// ErrPostNotFound means the post does not exist.
	ErrPostNotFound = errors.New("post not found")
	// ErrForbidden means the acting user may not modify this post.
	ErrForbidden = errors.New("forbidden")
	// ErrGuestViewDenied means guest viewing is disabled and the caller is anonymous.
	ErrGuestViewDenied = errors.New("guest view denied")
	// ErrBadRequest means the request is invalid for the current state.
	ErrBadRequest = errors.New("bad request")
	// ErrDuplicateName means a category or tag with the same name already exists.
	ErrDuplicateName = errors.New("duplicate name")
	// ErrCategoryNotFound means the category does not exist.
	ErrCategoryNotFound = errors.New("category not found")
	// ErrTagNotFound means the tag does not exist.
	ErrTagNotFound = errors.New("tag not found")
)

// InUseError means a category or tag is still referenced by posts; Count
// carries the number of referencing posts so callers can render it.
type InUseError struct {
	Count int64
}

// Error renders the in-use reason.
func (e *InUseError) Error() string {
	return fmt.Sprintf("in use by %d posts", e.Count)
}

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
