// Package home implements the homepage aggregation endpoints (site stats).
package home

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the home domain.
type Repository interface {
	GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error)
	CountPosts(ctx context.Context) (int64, error)
	CountUsers(ctx context.Context) (int64, error)
	CountComments(ctx context.Context) (int64, error)
	CountCategories(ctx context.Context) (int64, error)
	CountTags(ctx context.Context) (int64, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed home repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	var config model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) CountPosts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Post{}).Count(&count).Error
	return count, err
}

func (r *gormRepository) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *gormRepository) CountComments(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Comment{}).Count(&count).Error
	return count, err
}

func (r *gormRepository) CountCategories(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Category{}).Count(&count).Error
	return count, err
}

func (r *gormRepository) CountTags(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Tag{}).Count(&count).Error
	return count, err
}
