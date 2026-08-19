// Package sso implements OAuth2 / OIDC login (GitHub, Google, OIDC) and
// SSO account binding.
package sso

import (
	"context"

	"vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Repository is the persistence interface for the sso domain.
type Repository interface {
	FindSSOBinding(ctx context.Context, provider, providerID string) (*model.SSOBinding, error)
	FindUserByID(ctx context.Context, id uint) (*model.User, error)
	FindUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	SaveUser(ctx context.Context, user *model.User) error
	CreateBinding(ctx context.Context, binding *model.SSOBinding) error
	CountUsersByUsername(ctx context.Context, username string) (int64, error)
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed sso repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindSSOBinding(ctx context.Context, provider, providerID string) (*model.SSOBinding, error) {
	var binding model.SSOBinding
	if err := r.db.WithContext(ctx).Where("provider = ? AND provider_id = ?", provider, providerID).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
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

func (r *gormRepository) CreateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *gormRepository) SaveUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *gormRepository) CreateBinding(ctx context.Context, binding *model.SSOBinding) error {
	return r.db.WithContext(ctx).Create(binding).Error
}

func (r *gormRepository) CountUsersByUsername(ctx context.Context, username string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	return count, err
}
