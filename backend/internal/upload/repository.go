// Package upload implements media file storage and management.
package upload

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the upload domain.
type Repository interface {
	CreateMedia(ctx context.Context, media *model.MediaFile) error
	ListMediaByUser(ctx context.Context, userID uint) ([]model.MediaFile, error)
	FindMediaByID(ctx context.Context, id string) (*model.MediaFile, error)
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	DeleteMedia(ctx context.Context, media *model.MediaFile) error
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed upload repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CreateMedia(ctx context.Context, media *model.MediaFile) error {
	return r.db.WithContext(ctx).Create(media).Error
}

func (r *gormRepository) ListMediaByUser(ctx context.Context, userID uint) ([]model.MediaFile, error) {
	var files []model.MediaFile
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *gormRepository) FindMediaByID(ctx context.Context, id string) (*model.MediaFile, error) {
	var media model.MediaFile
	if err := r.db.WithContext(ctx).First(&media, id).Error; err != nil {
		return nil, err
	}
	return &media, nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) DeleteMedia(ctx context.Context, media *model.MediaFile) error {
	return r.db.WithContext(ctx).Delete(media).Error
}
