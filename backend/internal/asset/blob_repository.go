package asset

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BlobRepository interface {
	Create(ctx context.Context, blob *model.Blob) error
	FindUnreferencedByIDForUpdate(ctx context.Context, blobIDs []uint) ([]model.Blob, error)
	MarkPendingDelete(ctx context.Context, blobIDs []uint) error
	DeletePendingIfUnreferenced(ctx context.Context, blobID uint) error
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

func (r *blobRepository) FindUnreferencedByIDForUpdate(ctx context.Context, blobIDs []uint) ([]model.Blob, error) {
	if len(blobIDs) == 0 {
		return []model.Blob{}, nil
	}

	var blobs []model.Blob
	err := r.db.WithContext(ctx).
		Model(&model.Blob{}).
		Where("blobs.id IN ?", blobIDs).
		Where("blobs.status = ?", model.BlobStatusActive).
		Where(`
		NOT EXISTS (
			SELECT 1 FROM assets WHERE assets.blob_id = blobs.id
		)
		`).
		Clauses(clause.Locking{
			Strength: "UPDATE",
		}).
		Find(&blobs).
		Error
	if err != nil {
		return nil, err
	}

	return blobs, nil
}

func (r *blobRepository) MarkPendingDelete(ctx context.Context, blobIDs []uint) error {
	if len(blobIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&model.Blob{}).
		Where("id IN ?", blobIDs).
		Where("status = ?", model.BlobStatusActive).
		Update("status", model.BlobStatusPendingDelete).
		Error
}

func (r *blobRepository) DeletePendingIfUnreferenced(ctx context.Context, blobID uint) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Where("id = ?", blobID).
		Where("status = ?", model.BlobStatusPendingDelete).
		Where(`
			NOT EXISTS (
				SELECT 1
				FROM assets
				WHERE assets.blob_id = blobs.id
			)
		`).
		Delete(&model.Blob{}).
		Error
}
