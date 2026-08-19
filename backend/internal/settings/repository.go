// Package settings implements the admin configuration domain: SMTP, AI,
// general site settings and theme configuration.
package settings

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the settings domain.
type Repository interface {
	GetSMTPConfig(ctx context.Context) (model.SMTPConfig, error)
	CreateSMTPConfig(ctx context.Context, config *model.SMTPConfig) error
	SaveSMTPConfig(ctx context.Context, config *model.SMTPConfig) error

	GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error)
	CreateGeneralSettings(ctx context.Context, config *model.GeneralSettings) error
	SaveGeneralSettings(ctx context.Context, config *model.GeneralSettings) error

	GetAIConfig(ctx context.Context) (model.AIConfig, error)
	CreateAIConfig(ctx context.Context, config *model.AIConfig) error
	SaveAIConfig(ctx context.Context, config *model.AIConfig) error

	GetThemeConfig(ctx context.Context) (model.ThemeConfig, error)
	CreateThemeConfig(ctx context.Context, config *model.ThemeConfig) error
	SaveThemeConfig(ctx context.Context, config *model.ThemeConfig) error
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed settings repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetSMTPConfig(ctx context.Context) (model.SMTPConfig, error) {
	var config model.SMTPConfig
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) CreateSMTPConfig(ctx context.Context, config *model.SMTPConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *gormRepository) SaveSMTPConfig(ctx context.Context, config *model.SMTPConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *gormRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	var config model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) CreateGeneralSettings(ctx context.Context, config *model.GeneralSettings) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *gormRepository) SaveGeneralSettings(ctx context.Context, config *model.GeneralSettings) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *gormRepository) GetAIConfig(ctx context.Context) (model.AIConfig, error) {
	var config model.AIConfig
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) CreateAIConfig(ctx context.Context, config *model.AIConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *gormRepository) SaveAIConfig(ctx context.Context, config *model.AIConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *gormRepository) GetThemeConfig(ctx context.Context) (model.ThemeConfig, error) {
	var config model.ThemeConfig
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}

func (r *gormRepository) CreateThemeConfig(ctx context.Context, config *model.ThemeConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *gormRepository) SaveThemeConfig(ctx context.Context, config *model.ThemeConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}
