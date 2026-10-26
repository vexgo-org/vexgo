package migration

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// backfillSiteIcon maps the legacy general_settings.site_icon URL onto the
// asset reference created by backfillAssetBlob, then drops the dead column.
// It runs after backfillAssetBlob because that is what creates the assets the
// legacy /uploads/ URLs point at.
func (s *Service) backfillSiteIcon() error {
	settings := &model.GeneralSettings{}
	if !s.db.Migrator().HasTable(settings) ||
		!s.db.Migrator().HasColumn(settings, "site_icon") {
		return nil
	}

	// The column is no longer part of the model, so read it with a raw select.
	type legacyRow struct {
		ID       uint
		SiteIcon string
	}
	var rows []legacyRow
	if err := s.db.Table("general_settings").
		Select("id, site_icon").
		Where("site_icon <> ''").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("read legacy site icon: %w", err)
	}

	migrated := 0
	for _, row := range rows {
		key, ok := strings.CutPrefix(row.SiteIcon, "/uploads/")
		if !ok {
			slog.Warn("skip site icon with a non-local URL", "id", row.ID, "url", row.SiteIcon)
			continue
		}

		var blob model.Blob
		if err := s.db.Where("storage_key = ?", key).
			First(&blob).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn("skip site icon without a migrated media file", "id", row.ID, "url", row.SiteIcon)
				continue
			}
			return fmt.Errorf("find site icon blob %q: %w", key, err)
		}

		var asset model.Asset
		if err := s.db.Where("blob_id = ?", blob.ID).
			First(&asset).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn("skip site icon without a migrated asset", "id", row.ID, "url", row.SiteIcon)
				continue
			}
			return fmt.Errorf("find site icon asset (blob=%d): %w", blob.ID, err)
		}

		result := s.db.Model(settings).
			Where("id = ? AND site_icon_id IS NULL", row.ID).
			Update("site_icon_id", asset.ID)
		if result.Error != nil {
			return fmt.Errorf("backfill site icon (id=%d): %w", row.ID, result.Error)
		}
		if result.RowsAffected > 0 {
			migrated++
		}
	}

	// Drop after mapping, so a later startup cannot resurrect an icon the
	// admin cleared in the meantime.
	if err := dropLegacyColumn(s.db, settings, "site_icon"); err != nil {
		return fmt.Errorf("drop legacy site_icon column: %w", err)
	}

	if migrated > 0 {
		slog.Info("migrated legacy site icon to an asset reference", "rows", migrated)
	}
	return nil
}
