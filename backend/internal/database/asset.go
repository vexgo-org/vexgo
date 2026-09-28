package database

import (
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
