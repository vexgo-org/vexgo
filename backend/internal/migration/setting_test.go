package migration

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// seedSiteIconSetting creates the legacy site_icon column, which AutoMigrate
// no longer creates, and stores url in the singleton settings row.
func seedSiteIconSetting(t *testing.T, db *gorm.DB, url string) uint {
	t.Helper()

	settings := model.GeneralSettings{SiteName: "VexGo"}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("seed general settings: %v", err)
	}
	if err := db.Exec("ALTER TABLE general_settings ADD COLUMN site_icon text").Error; err != nil {
		t.Fatalf("add legacy site_icon column: %v", err)
	}
	if err := db.Exec("UPDATE general_settings SET site_icon = ?", url).Error; err != nil {
		t.Fatalf("seed legacy site icon: %v", err)
	}

	return settings.ID
}

func reloadSettings(t *testing.T, db *gorm.DB, id uint) model.GeneralSettings {
	t.Helper()

	var settings model.GeneralSettings
	if err := db.First(&settings, id).Error; err != nil {
		t.Fatalf("reload general settings: %v", err)
	}

	return settings
}

func TestMigrateDB_BackfillsSiteIconFromLegacyUpload(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "a1b2c3d4.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key})
	settingsID := seedSiteIconSetting(t, db, "/uploads/"+key)

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var asset model.Asset
	if err := db.Where("original_name = ?", key).First(&asset).Error; err != nil {
		t.Fatalf("find migrated asset: %v", err)
	}
	settings := reloadSettings(t, db, settingsID)
	if settings.SiteIconID == nil || *settings.SiteIconID != asset.ID {
		t.Fatalf("site icon id = %v, want %d", settings.SiteIconID, asset.ID)
	}
	if db.Migrator().HasColumn(&model.GeneralSettings{}, "site_icon") {
		t.Error("legacy site_icon column still exists after the migration")
	}
}

func TestMigrateDB_SkipsUnmappableSiteIcon(t *testing.T) {
	s, db, _ := newMigrationService(t)
	settingsID := seedSiteIconSetting(t, db, "https://example.com/icon.png")

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if settings := reloadSettings(t, db, settingsID); settings.SiteIconID != nil {
		t.Fatalf("site icon id = %d, want NULL", *settings.SiteIconID)
	}
	if db.Migrator().HasColumn(&model.GeneralSettings{}, "site_icon") {
		t.Error("legacy site_icon column still exists after the migration")
	}
}

// TestMigrateDB_DoesNotResurrectClearedSiteIcon pins the one-shot contract:
// the legacy column is gone after the first start, so clearing the icon does
// not bring it back on the next one.
func TestMigrateDB_DoesNotResurrectClearedSiteIcon(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "a1b2c3d4.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key})
	settingsID := seedSiteIconSetting(t, db, "/uploads/"+key)

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := db.Model(&model.GeneralSettings{}).
		Where("id = ?", settingsID).
		Update("site_icon_id", nil).Error; err != nil {
		t.Fatalf("clear site icon: %v", err)
	}
	if err := s.MigrateDB(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if settings := reloadSettings(t, db, settingsID); settings.SiteIconID != nil {
		t.Fatalf("site icon id = %d after clearing, want NULL", *settings.SiteIconID)
	}
}
