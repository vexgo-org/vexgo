package asset

import (
	"gorm.io/gorm"
)

type Repositories interface {
	Assets() AssetRepository
	Blobs() BlobRepository
	Config() ConfigRepository
}

type repositories struct {
	assets AssetRepository
	blobs  BlobRepository
	config ConfigRepository
}

func newRepositories(db *gorm.DB) Repositories {
	return &repositories{
		assets: NewAssetRepository(db),
		blobs:  NewBlobRepository(db),
		config: NewConfigRepository(db),
	}
}

func (r *repositories) Assets() AssetRepository {
	return r.assets
}

func (r *repositories) Blobs() BlobRepository {
	return r.blobs
}

func (r *repositories) Config() ConfigRepository {
	return r.config
}
