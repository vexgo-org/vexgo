package upload

import (
	"errors"
	"fmt"
	"io"

	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
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
	db      *gorm.DB
	storage Storage
}

// NewService creates an upload service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{db: deps.DB, storage: deps.Storage}
}

// Upload stores a file and records it in the database.
func (s *Service) Upload(userID uint, filename string, size int64, src io.Reader) (model.MediaFile, error) {
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
	if err := s.db.Create(&media).Error; err != nil {
		return model.MediaFile{}, fmt.Errorf("failed to save file record: %w", err)
	}
	return media, nil
}

// ListByUser returns the files uploaded by a user.
func (s *Service) ListByUser(userID uint) ([]model.MediaFile, error) {
	var files []model.MediaFile
	if err := s.db.Where("user_id = ?", userID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

// Delete removes a media file (from storage and database) when the acting user
// is its uploader or an admin.
func (s *Service) Delete(id string, userID uint) error {
	var media model.MediaFile
	if err := s.db.First(&media, id).Error; err != nil {
		return ErrNotFound
	}

	var user model.User
	if err := s.db.First(&user, userID).Error; err == nil {
		if user.Role != model.RoleAdmin && media.UserID != userID {
			return ErrForbidden
		}
	}

	// Delete the underlying file; log but continue to delete the DB record
	if err := s.storage.Delete(media.URL); err != nil {
		logrus.WithError(err).Warn("Failed to delete file")
	}

	return s.db.Delete(&media).Error
}
