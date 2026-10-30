package migration

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// backfillPostCoverImage maps the legacy posts.cover_image URL onto the asset
// created by backfillAssetBlob, then drops the dead column. When the cover came
// from an upload, its URL was copied from media_files.url, so matching that
// column also covers S3 URLs without parsing backend-specific paths.
func (s *Service) backfillPostCoverImage() error {
	post := &model.Post{}
	if !s.db.Migrator().HasTable(post) ||
		!s.db.Migrator().HasColumn(post, "cover_image") {
		return nil
	}

	// The column is no longer part of the model, so read it with a raw select.
	type legacyRow struct {
		ID         uint
		CoverImage string
	}
	var rows []legacyRow
	if err := s.db.Table("posts").
		Select("id, cover_image").
		Where("cover_image IS NOT NULL AND cover_image <> ''").
		Where("cover_image_id IS NULL").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("read legacy post cover image: %w", err)
	}

	migrated := 0
	for _, row := range rows {
		var media model.MediaFile
		if err := s.db.Where("url = ?", row.CoverImage).
			First(&media).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn(
					"skip post cover without a migrated media file",
					"postID", row.ID,
					"url", row.CoverImage,
				)
				continue
			}
			return fmt.Errorf("find media file for post cover (post=%d): %w", row.ID, err)
		}

		key := media.StorageKey
		if key == "" {
			var ok bool
			key, ok = strings.CutPrefix(media.URL, "/uploads/")
			if !ok {
				slog.Warn(
					"skip post cover with no storage key",
					"postID", row.ID,
					"url", row.CoverImage,
				)
				continue
			}
		}

		var blob model.Blob
		if err := s.db.Where("storage_key = ?", key).
			First(&blob).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn(
					"skip post cover without a migrated blob",
					"postID", row.ID,
					"url", row.CoverImage,
				)
				continue
			}
			return fmt.Errorf("find post cover blob %q: %w", key, err)
		}

		// A cover must be a public image: the same invariant the post API
		// enforces. Ownership is not checked because the legacy column does not
		// record who uploaded it and an admin may have set another author's
		// cover.
		if blob.Type != model.BlobTypeImage {
			slog.Warn(
				"skip post cover that is not an image",
				"postID", row.ID,
				"url", row.CoverImage,
				"type", blob.Type,
			)
			continue
		}

		var asset model.Asset
		if err := s.db.
			Where("blob_id = ? AND visibility = ?",
				blob.ID, model.AssetVisibilityPublic).
			First(&asset).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn(
					"skip post cover without a migrated asset",
					"postID", row.ID,
					"url", row.CoverImage,
				)
				continue
			}
			return fmt.Errorf("find post cover asset (blob=%d): %w", blob.ID, err)
		}

		result := s.db.Model(post).
			Where("id = ? AND cover_image_id IS NULL", row.ID).
			Update("cover_image_id", asset.ID)
		if result.Error != nil {
			return fmt.Errorf("backfill post cover (post=%d): %w", row.ID, result.Error)
		}
		if result.RowsAffected > 0 {
			migrated++
		}
	}

	// Drop after mapping, so a later startup cannot resurrect a cover the
	// author cleared in the meantime.
	if err := dropLegacyColumn(s.db, post, "cover_image"); err != nil {
		return fmt.Errorf("drop legacy post cover_image column: %w", err)
	}

	if migrated > 0 {
		slog.Info("migrated legacy post cover image to an asset reference", "rows", migrated)
	}
	return nil
}
