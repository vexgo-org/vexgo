package asset

import (
	"errors"
	"fmt"
	"net/http"

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

// detectMIMEType detects MIME type from the file header.
func detectMIMEType(reader *bufio.Reader) (string, error) {
	header, err := reader.Peek(512)
	if err != nil &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, bufio.ErrBufferFull) {
		return "", fmt.Errorf("read file header: %w", err)
	}

	if len(header) == 0 {
		return "", errors.New("empty file")
	}

	return http.DetectContentType(header), nil
}
