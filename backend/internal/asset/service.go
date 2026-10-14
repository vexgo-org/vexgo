package asset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// rollbackUpload removes file from storage when uploading error occurs.
func (s *Service) rollbackUpload(ctx context.Context, key string) {
	if err := s.storage.Delete(ctx, key); err != nil {
		slog.WarnContext(
			ctx,
			"failed to remove uploading file after media creation failed",
			"storage_key", key,
			"err", err,
		)
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
