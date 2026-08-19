// Package verification implements email verification and sliding-puzzle
// captcha generation/verification.
package verification

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the verification domain.
type Repository interface {
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	FindUserByToken(ctx context.Context, token string) (*model.User, error)
	CreateCaptcha(ctx context.Context, captcha *model.Captcha) error
	FindCaptcha(ctx context.Context, id, token string) (*model.Captcha, error)
	SaveCaptcha(ctx context.Context, captcha *model.Captcha) error
	GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) FindUserByToken(ctx context.Context, token string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("verification_token = ?", token).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
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

func (r *gormRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	var settings model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&settings).Error; err != nil {
		return settings, err
	}
	return settings, nil
}
