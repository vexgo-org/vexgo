package migration

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/idgen"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// backfillAssetBlob fills asset and blob record from legacy `model.MediaFile`.
func (s *Service) backfillAssetBlob() error {
	if !s.db.Migrator().HasTable(&model.MediaFile{}) {
		return nil
	}

	var mediaFiles []model.MediaFile
	publicIDGenerator := idgen.NewUUIDv7Generator()

	// Query all media file records.
	if err := s.db.Find(&mediaFiles).Error; err != nil {
		return fmt.Errorf("find all media file: %w", err)
	}

	// Load every blob key once, so the loop skips already-migrated media with
	// a map lookup instead of a query per row. Unscoped keeps the soft-deleted
	// blobs, whose keys still occupy the unique index.
	var storageKeys []string
	if err := s.db.Model(&model.Blob{}).
		Unscoped().
		Pluck("storage_key", &storageKeys).
		Error; err != nil {
		return fmt.Errorf("list migrated storage keys: %w", err)
	}

	migrated := make(map[string]struct{}, len(storageKeys))
	for _, storageKey := range storageKeys {
		migrated[storageKey] = struct{}{}
	}

	slog.Info("migrating media files", "count", len(mediaFiles))

	count := 0

	// Migrate media files one by one.
	for _, mediaFile := range mediaFiles {
		key := mediaFile.StorageKey
		if key == "" {
			var ok bool
			if key, ok = strings.CutPrefix(mediaFile.URL, "/uploads/"); !ok {
				slog.Warn(
					"skip media file with a non-local URL",
					"id", mediaFile.ID,
					"url", mediaFile.URL,
				)
				continue
			}
		}

		if _, ok := migrated[key]; ok {
			continue
		}

		reader, err := s.storage.Open(context.Background(), key)
		if err != nil {
			slog.Error(
				"failed to open file",
				"id", mediaFile.ID,
				"filename", key,
				"err", err,
			)
			continue
		}

		buffered := bufio.NewReader(reader)
		mimeType, err := asset.DetectMIMEType(buffered)
		if err != nil {
			if errors.Is(err, asset.ErrEmptyFile) {
				slog.Warn(
					"empty file detected, skip the migration",
					"id", mediaFile.ID,
					"filename", key,
				)
			} else {
				slog.Error(
					"failed to detect MIME type",
					"id", mediaFile.ID,
					"filename", key,
					"err", err,
				)
			}
			reader.Close()
			continue
		}

		blobType, err := model.BlobTypeFromMIME(mimeType)
		if err != nil {
			slog.Error(
				"invalid MIME type",
				"id", mediaFile.ID,
				"filename", key,
				"type", mimeType,
				"err", err,
			)
			reader.Close()
			continue
		}

		blobSize, err := io.Copy(io.Discard, buffered)
		if err != nil {
			slog.Warn(
				"failed to calculate the true blob size, fallback to previous size",
				"id", mediaFile.ID,
				"filename", key,
				"err", err,
			)
			blobSize = mediaFile.Size
		}

		if err := s.db.Transaction(func(tx *gorm.DB) error {
			blob := model.Blob{
				StorageKey: key,
				MimeType:   mimeType,
				Type:       blobType,
				Size:       blobSize,
			}
			if err := tx.Create(&blob).Error; err != nil {
				return err
			}

			publicID, err := publicIDGenerator.New()
			if err != nil {
				return err
			}

			newAsset := model.Asset{
				OriginalName: key,
				UserID:       mediaFile.UserID,
				BlobID:       blob.ID,
				PublicID:     publicID,
				Visibility:   model.AssetVisibilityPublic,
			}
			if err := tx.Create(&newAsset).Error; err != nil {
				return err
			}

			return nil
		}); err != nil {
			slog.Error(
				"failed to insert new asset and blob",
				"id", mediaFile.ID,
				"filename", key,
				"err", err,
			)
			reader.Close()
			continue
		}

		count++
		migrated[key] = struct{}{}
		reader.Close()
	}

	slog.Info(
		"media files migrated",
		"migrated", count,
		"total", len(mediaFiles),
	)

	return nil
}
