package asset

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type BlobRepository interface {
	Create(ctx context.Context, blob *model.Blob) error
}

type blobRepository struct {
	db *gorm.DB
}

func NewBlobRepository(db *gorm.DB) BlobRepository {
	return &blobRepository{db: db}
}

func (r *blobRepository) Create(ctx context.Context, blob *model.Blob) error {
	return r.db.
		WithContext(ctx).
		Create(blob).
		Error
}
