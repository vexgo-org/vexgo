package asset

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/vexgo-org/vexgo/backend/internal/model"
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

// Upload stores a file to the storage backend,
// and creates an asset record with a blob record
// in the database.
func (s *Service) Upload(
	ctx context.Context,
	userID uint,
	reader io.Reader,
	filename string,
	size int64,
) (*model.Asset, error) {
	// Validate parameters.
	if reader == nil {
		return nil, errors.New("file reader is nil")
	}

	if filename == "" {
		return nil, errors.New("filename is empty")
	}

	if size <= 0 {
		return nil, errors.New("invalid file size")
	}

	// Generate UUID storage key from filename.
	key, err := s.keyGenerator.Generate(filename)
	if err != nil {
		return nil, fmt.Errorf("generate UUID for %s: %w", filename, err)
	}

	// Detect MIME type and blob type.
	bufferedReader := bufio.NewReader(reader)
	mimeType, err := detectMIMEType(bufferedReader)
	if err != nil {
		return nil, fmt.Errorf("detect MIME type: %w", err)
	}
	blobType := model.BlobTypeFromMIME(mimeType)

	// Put the file to the storage backend.
	if err := s.storage.Put(
		ctx,
		key,
		bufferedReader,
		size,
		mimeType,
	); err != nil {
		return nil, fmt.Errorf("put file to storage: %w", err)
	}

	// Create blob and asset.
	blob := &model.Blob{
		StorageKey: key,
		MimeType:   mimeType,
		Type:       blobType,
		Size:       size,
	}
	asset := &model.Asset{
		OriginalName: filename,
		UserID:       userID,
	}

	// Record blob and asset in database within a transaction.
	err = s.txManager.WithTransaction(
		ctx,
		func(repos Repositories) error {
			if err := repos.Blobs().Create(ctx, blob); err != nil {
				return err
			}
			asset.BlobID = blob.ID
			if err := repos.Assets().Create(ctx, asset); err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		s.rollbackUpload(ctx, key)
		return nil, fmt.Errorf("create asset and blob record: %w", err)
	}
	asset.Blob = *blob

	return asset, nil
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
