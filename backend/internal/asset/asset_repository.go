package asset

import (
	"context"

	"github.com/google/uuid"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type AssetRepository interface {
	Create(ctx context.Context, asset *model.Asset) error
	FindByID(ctx context.Context, id uint) (*model.Asset, error)
	FindByPublicID(ctx context.Context, publicID uuid.UUID) (*model.Asset, error)
	FindPublicByPublicID(ctx context.Context, publicID uuid.UUID) (*model.Asset, error)
	SoftDelete(ctx context.Context, asset *model.Asset) error
}

type assetRepository struct {
	db *gorm.DB
}

func NewAssetRepository(db *gorm.DB) AssetRepository {
	return &assetRepository{db: db}
}

func (r *assetRepository) Create(ctx context.Context, asset *model.Asset) error {
	return r.db.
		WithContext(ctx).
		Create(asset).
		Error
}

func (r *assetRepository) FindByID(ctx context.Context, id uint) (*model.Asset, error) {
	if id == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return r.find(ctx, &model.Asset{ID: id})
}

func (r *assetRepository) FindByPublicID(ctx context.Context, publicID uuid.UUID) (*model.Asset, error) {
	if publicID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}

	return r.find(ctx, &model.Asset{PublicID: publicID})
}

func (r *assetRepository) FindPublicByPublicID(ctx context.Context, publicID uuid.UUID) (*model.Asset, error) {
	if publicID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}

	return r.find(ctx, &model.Asset{
		PublicID:   publicID,
		Visibility: model.AssetVisibilityPublic,
	})
}

func (r *assetRepository) SoftDelete(ctx context.Context, asset *model.Asset) error {
	if asset.ID == 0 {
		return gorm.ErrRecordNotFound
	}

	return r.db.
		WithContext(ctx).
		Delete(asset).
		Error
}

func (r *assetRepository) find(ctx context.Context, query *model.Asset) (*model.Asset, error) {
	var asset model.Asset
	if err := r.db.
		WithContext(ctx).
		Model(&model.Asset{}).
		Preload("Blob").
		Where(query).
		First(&asset).
		Error; err != nil {
		return nil, err
	}
	return &asset, nil
}
