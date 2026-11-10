package asset

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type ConfigRepository interface {
	Get(ctx context.Context) (*model.AssetConfig, error)
}

type configRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) Get(ctx context.Context) (*model.AssetConfig, error) {
	var config model.AssetConfig
	if err := r.db.WithContext(ctx).
		First(&config).
		Error; err != nil {
		return nil, err
	}
	return &config, nil
}
