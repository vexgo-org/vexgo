package asset

import (
	"context"
	"errors"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type Repository interface {
	CreateAsset(context.Context, *model.Asset) error
	FindAssetByStorageKey(context.Context, string) (*model.Asset, error)
	FindUserByID(context.Context, uint) (*model.User, error)
	SoftDeleteAsset(context.Context, *model.Asset) error
}

// gormRepository is the GORM-backed implementation of Repository.
type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a GORM-backed asset repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

// CreateAsset inserts an asset record.
func (r *gormRepository) CreateAsset(ctx context.Context, asset *model.Asset) error {
	return r.db.
		WithContext(ctx).
		Create(asset).
		Error
}

func (r *gormRepository) FindAssetByStorageKey(ctx context.Context, key string) (*model.Asset, error) {
	if key == "" {
		return nil, errors.New("storage key cannot be empty")
	}

	var asset model.Asset
	if err := r.db.
		WithContext(ctx).
		Where(&model.Asset{
			StorageKey: key,
		}).
		First(&asset).
		Error; err != nil {
		return nil, err
	}

	return &asset, nil
}

func (r *gormRepository) FindUserByID(ctx context.Context, userID uint) (*model.User, error) {
	var user model.User
	if err := r.db.
		WithContext(ctx).
		First(&user, userID).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormRepository) SoftDeleteAsset(ctx context.Context, asset *model.Asset) error {
	return r.db.
		WithContext(ctx).
		Delete(asset).
		Error
}
