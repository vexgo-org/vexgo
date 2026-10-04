package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"
	"gorm.io/gorm"
)

// Deps holds the dependencies required by the asset domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Storage   storage.Storage
}

// Service contains the business logic of the asset domain.
type Service struct {
	repo    Repository
	storage storage.Storage
}

// NewService creates an asset service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), storage: deps.Storage}
}

// Upload extracts file metadata, store the file to storage, and
// records it in database.
func (s *Service) Upload(
	ctx context.Context,
	userID uint,
	reader io.Reader,
	filename string,
	size int64,
) (*model.Asset, error) {
	// Generate storage key from filename.
	key, err := uuidFromFilename(filename)
	if err != nil {
		return nil, err
	}

	// Determine file types.
	mimeType := model.MimeTypeFromName(filename)
	assetType := model.AssetTypeFromMIME(mimeType)

	// Try to put the file into the storage.
	if err := s.storage.Put(
		ctx,
		key,
		reader,
		size,
		mimeType,
	); err != nil {
		return nil, err
	}

	// Generate URL after uploading.
	u, err := s.storage.URL(ctx, key)
	if err != nil {
		return nil, err
	}

	// Create asset record in database.
	asset := model.Asset{
		OriginalName: filename,
		StorageKey:   key,
		URL:          u,
		MimeType:     mimeType,
		Type:         assetType,
		Size:         size,
		UserID:       userID,
	}

	if err := s.repo.CreateAsset(ctx, &asset); err != nil {
		s.rollbackUpload(ctx, key)
		return nil, fmt.Errorf("failed to save file record: %w", err)
	}

	return &asset, nil
}

// Delete soft-deletes a file by storage key.
func (s *Service) Delete(ctx context.Context, userID uint, key string) error {
	// Find the asset from database.
	asset, err := s.repo.FindAssetByStorageKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to find asset by storage key: %w", err)
	}

	return s.softDelete(ctx, userID, asset)
}

// DeleteByID soft-deletes a files by asset ID.
func (s *Service) DeleteByID(ctx context.Context, userID, assetID uint) error {
	asset, err := s.repo.FindAssetByID(ctx, assetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to find asset by id: %w", err)
	}

	return s.softDelete(ctx, userID, asset)
}

// FindByID finds an asset by asset ID.
func (s *Service) FindByID(ctx context.Context, id uint) (*model.Asset, error) {
	return s.repo.FindAssetByID(ctx, id)
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

// softDelete soft-deletes an asset.
// It only marks the file as deleted in database,
// and keeps original file in storage.
func (s *Service) softDelete(ctx context.Context, userID uint, asset *model.Asset) error {
	// Find the operator.
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Verify user permission of deleting this file.
	// Admin and super admin can delete any files.
	// Other users can only delete their own files.
	if user.ID != asset.UserID && !model.IsAdmin(user.Role) {
		return ErrForbidden
	}

	// Soft-delete asset record in database.
	if err := s.repo.SoftDeleteAsset(ctx, asset); err != nil {
		return fmt.Errorf("failed to soft delete asset: %w", err)
	}

	return nil
}
