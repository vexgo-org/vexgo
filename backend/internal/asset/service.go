package asset

import (
	"github.com/vexgo-org/vexgo/backend/internal/storage"
	"gorm.io/gorm"
)

type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Storage   storage.Storage
}

type Service struct {
	assets       AssetRepository
	blobs        BlobRepository
	txManager    TransactionManager
	storage      storage.Storage
	keyGenerator KeyGenerator
}

func NewService(deps Deps) *Service {
	repos := newRepositories(deps.DB)

	return &Service{
		assets:       repos.Assets(),
		blobs:        repos.Blobs(),
		txManager:    NewTransactionManager(deps.DB),
		storage:      deps.Storage,
		keyGenerator: NewKeyGenerator(),
	}
}
