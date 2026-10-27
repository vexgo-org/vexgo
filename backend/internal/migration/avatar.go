package migration

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// backfillUserAvatar maps the legacy users.avatar URL onto the asset created by
// backfillAssetBlob, then drops the dead column. When the avatar came from an
// upload, its URL was copied from media_files.url, so matching that column also
// covers S3 URLs without parsing backend-specific paths.
func (s *Service) backfillUserAvatar() error {
	user := &model.User{}
	if !s.db.Migrator().HasTable(user) ||
		!s.db.Migrator().HasColumn(user, "avatar") {
		return nil
	}

	// The column is no longer part of the model, so read it with a raw select.
	type legacyRow struct {
		ID     uint
		Avatar string
	}
	var rows []legacyRow
	if err := s.db.Table("users").
		Select("id, avatar").
		Where("avatar IS NOT NULL AND avatar <> ''").
		Where("avatar_id IS NULL").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("read legacy user avatar: %w", err)
	}

	migrated := 0
	for _, row := range rows {
		var media model.MediaFile
		if err := s.db.
			Where("url = ? AND user_id = ?", row.Avatar, row.ID).
			First(&media).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn(
					"skip user avatar without a migrated media file",
					"userID", row.ID,
					"url", row.Avatar,
				)
				continue
			}
			return fmt.Errorf("find media file for user avatar (user=%d): %w", row.ID, err)
		}

		key := media.StorageKey
		if key == "" {
			var ok bool
			key, ok = strings.CutPrefix(media.URL, "/uploads/")
			if !ok {
				slog.Warn(
					"skip user avatar with no storage key",
					"userID", row.ID,
					"url", row.Avatar,
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
					"skip user avatar without a migrated blob",
					"userID", row.ID,
					"url", row.Avatar,
				)
				continue
			}
			return fmt.Errorf("find user avatar blob %q: %w", key, err)
		}

		// A user avatar must be a public image: the same invariant the
		// profile API enforces when it sets one.
		if blob.Type != model.BlobTypeImage {
			slog.Warn(
				"skip user avatar that is not an image",
				"userID", row.ID,
				"url", row.Avatar,
				"type", blob.Type,
			)
			continue
		}

		var asset model.Asset
		if err := s.db.
			Where("blob_id = ? AND user_id = ? AND visibility = ?",
				blob.ID, row.ID, model.AssetVisibilityPublic).
			First(&asset).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn(
					"skip user avatar without a migrated asset",
					"userID", row.ID,
					"url", row.Avatar,
				)
				continue
			}
			return fmt.Errorf("find user avatar asset (blob=%d): %w", blob.ID, err)
		}

		result := s.db.Model(user).
			Where("id = ? AND avatar_id IS NULL", row.ID).
			Update("avatar_id", asset.ID)
		if result.Error != nil {
			return fmt.Errorf("backfill user avatar (user=%d): %w", row.ID, result.Error)
		}
		if result.RowsAffected > 0 {
			migrated++
		}
	}

	// Drop after mapping, so a later startup cannot resurrect an avatar the
	// user cleared in the meantime.
	if err := dropLegacyColumn(s.db, user, "avatar"); err != nil {
		return fmt.Errorf("drop legacy user avatar column: %w", err)
	}

	if migrated > 0 {
		slog.Info("migrated legacy user avatar to an asset reference", "rows", migrated)
	}
	return nil
}
