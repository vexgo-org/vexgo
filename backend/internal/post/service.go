// Package post implements the post, like, category, tag and post-moderation
// domain.
package post

import (
	"context"
	"errors"

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
)

// Deps holds the dependencies required by the post domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
	Files     FileRemover
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
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier, files: deps.Files}
}

// allowGuestView reports whether anonymous viewers may see posts.
func (s *Service) allowGuestView(ctx context.Context) bool {
	return s.repo.GetGuestViewSetting(ctx)
}
