package asset

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"unicode/utf8"

	"github.com/vexgo-org/vexgo/backend/internal/idgen"
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
	assets            AssetRepository
	blobs             BlobRepository
	txManager         TransactionManager
	storage           storage.Storage
	keyGenerator      KeyGenerator
	publicIDGenerator idgen.Generator
}

func NewService(deps Deps) *Service {
	repos := newRepositories(deps.DB)

	return &Service{
		assets:            repos.Assets(),
		blobs:             repos.Blobs(),
		txManager:         NewTransactionManager(deps.DB),
		storage:           deps.Storage,
		keyGenerator:      NewKeyGenerator(),
		publicIDGenerator: idgen.NewUUIDv7Generator(),
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
		return nil, ErrEmptyFilename
	}

	if utf8.RuneCountInString(filename) > model.MaxAssetFilenameLength {
		return nil, ErrFilenameTooLong
	}

	if size <= 0 {
		return nil, ErrInvalidFileSize
	}

	// Generate UUID storage key from filename.
	key, err := s.keyGenerator.Generate(filename)
	if err != nil {
		return nil, fmt.Errorf("generate UUID for %s: %w", filename, err)
	}

	// Detect MIME type and blob type.
	bufferedReader := bufio.NewReader(reader)
	mimeType, err := DetectMIMEType(bufferedReader)
	if err != nil {
		return nil, fmt.Errorf("detect MIME type: %w", err)
	}

	blobType, err := model.BlobTypeFromMIME(mimeType)
	if err != nil {
		return nil, err
	}

	publicID, err := s.publicIDGenerator.New()
	if err != nil {
		return nil, fmt.Errorf("generate asset public ID: %w", err)
	}

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
		PublicID:     publicID,
		Visibility:   model.AssetVisibilityPublic,
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

// Delete soft deletes an asset.
func (s *Service) Delete(ctx context.Context, actor Actor, assetID uint) error {
	// Find the asset.
	asset, err := s.assets.FindByID(ctx, assetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("find asset by ID: %w", err)
	}

	// Check the user permission.
	// Only allow admin and asset owner to perform deletion.
	if asset.UserID != actor.ID && !model.IsAdmin(actor.Role) {
		return ErrForbidden
	}

	// Soft delete asset only.
	if err := s.assets.SoftDelete(ctx, asset); err != nil {
		return fmt.Errorf("soft delete asset: %w", err)
	}

	return nil
}

// ResolveOriginal resolves original content of an asset,
// and returns content and metadata of the asset.
func (s *Service) ResolveOriginal(ctx context.Context, assetID uint) (*OriginalContent, error) {
	if assetID == 0 {
		return nil, ErrNotFound
	}

	asset, err := s.assets.FindByID(ctx, assetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("find asset: %d: %w", assetID, err)
	}

	reader, err := s.storage.Open(ctx, asset.Blob.StorageKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrContentUnavailable
		}
		return nil, fmt.Errorf("open storage object for asset %d: %w", assetID, err)
	}

	return &OriginalContent{
		Reader:   reader,
		MIMEType: asset.Blob.MimeType,
		Size:     asset.Blob.Size,
		Filename: asset.OriginalName,
	}, nil
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
