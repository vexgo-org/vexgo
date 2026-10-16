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
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// backfillAssetBlob fills asset and blob record from legacy `model.MediaFile`.
func (s *Service) backfillAssetBlob() error {
	if !s.db.Migrator().HasTable(&model.MediaFile{}) {
		return nil
	}

	var mediaFiles []model.MediaFile

	// Query all media file records.
	if err := s.db.Find(&mediaFiles).Error; err != nil {
		return fmt.Errorf("find all media file: %w", err)
	}

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

		var existing model.Blob
		switch err := s.db.Unscoped().
			Where(&model.Blob{StorageKey: key}).
			First(&existing).
			Error; {
		case err == nil:
			continue
		case !errors.Is(err, gorm.ErrRecordNotFound):
			slog.Error(
				"loop up blob for media file failed",
				"id", mediaFile.ID,
				"err", err,
			)
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
				slog.Warn("empty file detected, skip the migration")
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

		blobType := model.BlobTypeFromMIME(mimeType)

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

			newAsset := model.Asset{
				OriginalName: key,
				UserID:       mediaFile.UserID,
				BlobID:       blob.ID,
			}
			if err := tx.Create(&newAsset).Error; err != nil {
				return err
			}

			return nil
		}); err != nil {
			slog.Error("failed to insert new asset and blob", "err", err)
			reader.Close()
			continue
		}

		reader.Close()
	}

	return nil
}
