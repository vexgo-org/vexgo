// Package auth implements authentication: login, registration, profile and
// password management.
package auth

import (
	"context"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the auth domain.
type Repository interface {
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	FindUserByEmail(ctx context.Context, email string) (*model.User, error)
	FindUserByEmailExcluding(ctx context.Context, email string, excludeID uint) (*model.User, error)
	FindUserByToken(ctx context.Context, token string) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUserToken(ctx context.Context, userID uint, token string, expiresAt time.Time) error
	SaveUser(ctx context.Context, user *model.User) error
	UpdateEmail(ctx context.Context, userID uint, email string) error
	UpdateVerifiedEmail(ctx context.Context, userID uint, email string) error
	UpdateUserEmailVerified(ctx context.Context, userID uint) error
	ResetPassword(ctx context.Context, userID uint, hashedPassword string) error
	GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error)
	// FindMediaByURL looks up the media_files row behind a stored URL so
	// avatar cleanup can verify ownership before deleting the file.
	FindMediaByURL(ctx context.Context, url string) (*model.MediaFile, error)
	FindCaptcha(ctx context.Context, id, token string) (*model.Captcha, error)
	// DeleteCaptcha removes a consumed or rejected challenge so the same
	// (id, token, x, y) answer cannot be replayed against auth endpoints.
	DeleteCaptcha(ctx context.Context, id, token string) error
	UpdateEmailChangeToken(ctx context.Context, userID uint, newEmail, token string, expiresAt time.Time) error
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed auth repository.
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

func (r *gormRepository) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) FindUserByEmailExcluding(
	ctx context.Context,
	email string,
	excludeID uint,
) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ? AND id != ?", email, excludeID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) FindUserByToken(ctx context.Context, token string) (*model.User, error) {
	var user model.User
	// Tokens are stored hashed (tokenStorageForm); the presented raw token is
	// hashed the same way before the lookup, so the plaintext never exists in
	// the database.
	if err := r.db.WithContext(ctx).Where("verification_token = ?", tokenStorageForm(token)).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) CreateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *gormRepository) SaveUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *gormRepository) UpdateEmail(ctx context.Context, userID uint, email string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).
		Update("email", email).Error
}

func (r *gormRepository) UpdateVerifiedEmail(ctx context.Context, userID uint, email string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(
			map[string]any{
				"email":              email,
				"email_verified":     true, // automatically verify the changed email
				"pending_email":      "",
				"verification_token": "",
				"token_expires_at":   time.Time{},
			},
		).Error
}

func (r *gormRepository) UpdateUserEmailVerified(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(
			map[string]any{
				"email_verified":     true,
				"verification_token": "",
				"token_expires_at":   time.Time{},
			},
		).Error
}

func (r *gormRepository) ResetPassword(ctx context.Context, userID uint, hashedPassword string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{
			"password": hashedPassword,
			// Bumping the version invalidates every outstanding JWT issued
			// before the reset (enforced by middleware.auth), so a stolen
			// session cannot survive a password reset.
			"password_version":   gorm.Expr("password_version + 1"),
			"verification_token": "",
			"token_expires_at":   time.Time{},
		}).Error
}

func (r *gormRepository) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	var settings model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&settings).Error; err != nil {
		return settings, err
	}
	return settings, nil
}

func (r *gormRepository) FindMediaByURL(ctx context.Context, url string) (*model.MediaFile, error) {
	var media model.MediaFile
	if err := r.db.WithContext(ctx).Where("url = ?", url).First(&media).Error; err != nil {
		return nil, err
	}
	return &media, nil
}

func (r *gormRepository) FindCaptcha(ctx context.Context, id, token string) (*model.Captcha, error) {
	var captcha model.Captcha
	if err := r.db.WithContext(ctx).Where("id = ? AND token = ?", id, token).First(&captcha).Error; err != nil {
		return nil, err
	}
	return &captcha, nil
}

func (r *gormRepository) DeleteCaptcha(ctx context.Context, id, token string) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND token = ?", id, token).
		Delete(&model.Captcha{}).Error
}

func (r *gormRepository) UpdateUserToken(ctx context.Context, userID uint, token string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(
			map[string]any{
				"verification_token": token,
				"token_expires_at":   expiresAt,
			},
		).Error
}

func (r *gormRepository) UpdateEmailChangeToken(ctx context.Context, userID uint, newEmail, token string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(
			map[string]any{
				"verification_token": token,
				"token_expires_at":   expiresAt,
				"pending_email":      newEmail,
			},
		).Error
}
