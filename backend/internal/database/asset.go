package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

func backFillAssets(db *gorm.DB) error {
	var assets []model.Asset

	if err := db.
		Unscoped().
		Find(&assets).
		Error; err != nil {
		return fmt.Errorf("failed to load legacy media files: %w", err)
	}

	for i := range assets {
		asset := &assets[i]

		updates := make(map[string]any)

		if asset.StorageKey == "" {
			key, err := storageKeyFromURL(asset.URL)
			if err != nil {
				return fmt.Errorf("failed to generate storage key from URL: %w", err)
			}
			updates["storage_key"] = key
			asset.StorageKey = key
		}

		if asset.OriginalName == "" {
			name := ""

			if asset.StorageKey != "" {
				name = path.Base(asset.StorageKey)
			}

			if name == "" || name == "." || name == "/" {
				name = originalNameFromURL(asset.URL)
			}

			if name == "" {
				name = fmt.Sprintf("asset-%d", asset.ID)
			}

			updates["original_name"] = name
			asset.OriginalName = name
		}

		if asset.MimeType == "" {
			name := asset.OriginalName
			if name == "" {
				name = asset.StorageKey
			}

			mimeType := model.MimeTypeFromName(name)
			updates["mime_type"] = mimeType
			asset.MimeType = mimeType
		}

		if asset.Type == "" ||
			asset.Type == "unknown" {
			updates["type"] = model.AssetTypeFromMIME(asset.MimeType)
		}

		if len(updates) == 0 {
			continue
		}

		if err := db.
			Model(&model.Asset{}).
			Where("id = ?", asset.ID).
			Updates(updates).
			Error; err != nil {
			return fmt.Errorf("failed to backfill asset %d: %w", asset.ID, err)
		}
	}

	return nil
}

// backfillSiteIcon points general_settings.site_icon_id at the asset the
// legacy site_icon URL names. The column is no longer part of the model, so
// the old value is read with a raw scan; it is left in place afterwards,
// unread. An icon that names no asset we manage — an external URL, or an
// upload that is already gone — resolves to no reference at all, and the
// favicon falls back to the other sources.
func backfillSiteIcon(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.GeneralSettings{}) ||
		!db.Migrator().HasColumn(&model.GeneralSettings{}, "site_icon") {
		return nil
	}

	var id uint
	var iconURL string

	err := db.Raw(
		"SELECT id, site_icon FROM general_settings ORDER BY id LIMIT 1",
	).Row().Scan(&id, &iconURL)
	if errors.Is(err, sql.ErrNoRows) || iconURL == "" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read the legacy site icon: %w", err)
	}

	key, err := storageKeyFromURL(iconURL)
	if err != nil {
		slog.Warn("legacy site icon is not an upload, leaving it unset", "url", iconURL)
		return nil
	}

	var asset model.Asset
	if err := db.Where("storage_key = ?", key).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to look up the site icon asset: %w", err)
	}

	if err := db.Model(&model.GeneralSettings{}).
		Where("id = ? AND site_icon_id IS NULL", id).
		Update("site_icon_id", asset.ID).Error; err != nil {
		return fmt.Errorf("failed to backfill the site icon reference: %w", err)
	}

	return nil
}

func storageKeyFromURL(rawURL string) (string, error) {
	const localStoragePrefix = "/uploads/"

	// Local storage.
	if after, ok := strings.CutPrefix(rawURL, localStoragePrefix); ok {
		return strings.Trim(after, "/"), nil
	}

	// S3 storage.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return strings.Trim(parsed.Path, "/"), nil
}

func originalNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		slog.Error("parse raw URL failed",
			"url", rawURL,
			"err", err,
		)
		return ""
	}

	name := path.Base(parsed.Path)
	if name == "." ||
		name == "/" ||
		name == "" {
		return ""
	}

	decoded, err := url.PathUnescape(name)
	if err != nil {
		return name
	}

	return strings.TrimSpace(decoded)
}
