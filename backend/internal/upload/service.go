package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// ErrNotFound means the media file does not exist.
	ErrNotFound = errors.New("file not found")
	// ErrForbidden means the acting user may not delete this file.
	ErrForbidden = errors.New("forbidden")
)

// Deps holds the dependencies required by the upload domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Storage   Storage
}

// Service contains the business logic of the upload domain.
type Service struct {
	repo    Repository
	storage Storage
}

// NewService creates an upload service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), storage: deps.Storage}
}

// Upload stores a file and records it in the database.
func (s *Service) Upload(ctx context.Context, userID uint, filename string, size int64, src io.Reader) (model.MediaFile, error) {
	url, err := s.storage.Upload(src, filename, "")
	if err != nil {
		return model.MediaFile{}, err
	}

	media := model.MediaFile{
		URL:    url,
		Size:   size,
		Type:   "unknown",
		UserID: userID,
	}
	if err := s.repo.CreateMedia(ctx, &media); err != nil {
		return model.MediaFile{}, fmt.Errorf("failed to save file record: %w", err)
	}
	return media, nil
}

// ListByUser returns the files uploaded by a user.
func (s *Service) ListByUser(ctx context.Context, userID uint) ([]model.MediaFile, error) {
	return s.repo.ListMediaByUser(ctx, userID)
}

// Delete removes a media file (from storage and database) when the acting user
// is its uploader or an admin.
func (s *Service) Delete(ctx context.Context, id string, userID uint) error {
	media, err := s.repo.FindMediaByID(ctx, id)
	if err != nil {
		return ErrNotFound
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err == nil {
		if !model.IsAdmin(user.Role) && media.UserID != userID {
			return ErrForbidden
		}
	}

	// Delete the underlying file; log but continue to delete the DB record
	if err := s.storage.Delete(media.URL); err != nil {
		slog.Warn("failed to delete file", "err", err)
	}

	return s.repo.DeleteMedia(ctx, media)
}
