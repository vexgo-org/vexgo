package asset

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type AssetRepository interface {
	Create(ctx context.Context, asset *model.Asset) error
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
