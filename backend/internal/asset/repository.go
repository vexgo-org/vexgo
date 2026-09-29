package asset

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type Repository interface {
	CreateAsset(context.Context, *model.Asset) error
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
