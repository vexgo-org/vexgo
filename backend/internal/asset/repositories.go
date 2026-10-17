package asset

import (
	"gorm.io/gorm"
)

type Repositories interface {
	Assets() AssetRepository
	Blobs() BlobRepository
}

type repositories struct {
	assets AssetRepository
	blobs  BlobRepository
}

func newRepositories(db *gorm.DB) Repositories {
	return &repositories{
		assets: NewAssetRepository(db),
		blobs:  NewBlobRepository(db),
	}
}

func (r *repositories) Assets() AssetRepository {
	return r.assets
}

func (r *repositories) Blobs() BlobRepository {
	return r.blobs
}
