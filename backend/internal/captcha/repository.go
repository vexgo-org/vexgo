// Package captcha implements sliding-puzzle captcha generation and
// verification.
package captcha

import (
	"context"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the captcha domain.
type Repository interface {
	CreateCaptcha(ctx context.Context, captcha *model.Captcha) error
	FindCaptcha(ctx context.Context, id, token string) (*model.Captcha, error)
	SaveCaptcha(ctx context.Context, captcha *model.Captcha) error
	DeleteExpiredCaptchas(ctx context.Context) error
	GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed captcha repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CreateCaptcha(ctx context.Context, captcha *model.Captcha) error {
	return r.db.WithContext(ctx).Create(captcha).Error
}

func (r *gormRepository) FindCaptcha(ctx context.Context, id, token string) (*model.Captcha, error) {
	var captcha model.Captcha
	if err := r.db.WithContext(ctx).Where("id = ? AND token = ?", id, token).First(&captcha).Error; err != nil {
		return nil, err
	}
	return &captcha, nil
}

func (r *gormRepository) SaveCaptcha(ctx context.Context, captcha *model.Captcha) error {
	return r.db.WithContext(ctx).Save(captcha).Error
}

func (r *gormRepository) DeleteExpiredCaptchas(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&model.Captcha{}).Error
}

func (r *gormRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	var settings model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&settings).Error; err != nil {
		return settings, err
	}
	return settings, nil
}
