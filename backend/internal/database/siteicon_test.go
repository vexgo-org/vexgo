package database

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// legacySiteIconSchema is the general_settings table an upgrading install has:
// the site_icon URL column still holding its value, plus the nullable
// site_icon_id column AutoMigrate adds next to it. The table is built by hand
// rather than by rewinding a migrated one, because GORM puts a foreign key on
// the new column and SQLite then refuses to drop it.
const legacySiteIconSchema = `
CREATE TABLE general_settings (
	id                    integer PRIMARY KEY AUTOINCREMENT,
	captcha_enabled       numeric,
	registration_enabled  numeric,
	allow_guest_view_posts numeric,
	site_name             text,
	site_description      text,
	site_icon             text,
	site_icon_id          integer,
	items_per_page        integer,
	site_language         text,
	created_at            datetime,
	updated_at            datetime
)`

// legacyDB opens a database holding the pre-refactor site icon column, seeded
// with iconURL as the configured icon.
func legacyDB(t *testing.T, iconURL string) *gorm.DB {
	t.Helper()
	db := newMigratedDB(t)

	if err := db.Exec("DROP TABLE general_settings").Error; err != nil {
		t.Fatalf("drop settings: %v", err)
	}
	if err := db.Exec(legacySiteIconSchema).Error; err != nil {
		t.Fatalf("create legacy settings: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO general_settings (site_name, site_icon) VALUES (?, ?)",
		"legacy", iconURL,
	).Error; err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	return db
}

// TestBackfillSiteIcon_LinksTheReferencedAsset is the upgrade path: an install
// that configured its icon by URL gets an asset reference pointing at the same
// upload, without the admin touching anything.
func TestBackfillSiteIcon_LinksTheReferencedAsset(t *testing.T) {
	db := legacyDB(t, "/uploads/deadbeef.png")
	asset := model.Asset{StorageKey: "deadbeef.png", URL: "/uploads/deadbeef.png"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	if err := backfillSiteIcon(db); err != nil {
		t.Fatalf("backfillSiteIcon: %v", err)
	}

	var config model.GeneralSettings
	if err := db.Preload("SiteIcon").First(&config).Error; err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if config.SiteIconID == nil || *config.SiteIconID != asset.ID {
		t.Fatalf("site_icon_id = %v, want %d", config.SiteIconID, asset.ID)
	}
	if config.SiteIcon == nil || config.SiteIcon.URL != "/uploads/deadbeef.png" {
		t.Errorf("SiteIcon = %+v, want the asset at /uploads/deadbeef.png", config.SiteIcon)
	}
}

// TestBackfillSiteIcon_Idempotent pins re-runnability: AutoMigrate runs on every
// boot, and the legacy column keeps its stale value forever, so a second pass
// must not move the reference.
func TestBackfillSiteIcon_Idempotent(t *testing.T) {
	db := legacyDB(t, "/uploads/deadbeef.png")
	first := model.Asset{StorageKey: "deadbeef.png", URL: "/uploads/deadbeef.png"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	if err := backfillSiteIcon(db); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	second := model.Asset{StorageKey: "cafe.png", URL: "/uploads/cafe.png"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	if err := backfillSiteIcon(db); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if config.SiteIconID == nil || *config.SiteIconID != first.ID {
		t.Errorf("site_icon_id = %v, want the first asset %d", config.SiteIconID, first.ID)
	}
}

// TestBackfillSiteIcon_LeavesUnresolvableIconsAlone covers the two cases that
// resolve to nothing: an externally hosted icon, and a URL whose upload is no
// longer in the table. Both must fall back to the other favicon sources rather
// than fail the migration.
func TestBackfillSiteIcon_LeavesUnresolvableIconsAlone(t *testing.T) {
	for name, iconURL := range map[string]string{
		"external": "https://example.com/icon.png",
		"missing":  "/uploads/gone.png",
	} {
		t.Run(name, func(t *testing.T) {
			db := legacyDB(t, iconURL)

			if err := backfillSiteIcon(db); err != nil {
				t.Fatalf("backfillSiteIcon: %v", err)
			}

			var config model.GeneralSettings
			if err := db.First(&config).Error; err != nil {
				t.Fatalf("load settings: %v", err)
			}
			if config.SiteIconID != nil {
				t.Errorf("site_icon_id = %d, want no reference", *config.SiteIconID)
			}
		})
	}
}

// TestBackfillSiteIcon_SkipsSoftDeletedAsset guards the soft-delete boundary: a
// pruned icon must not be linked back, or the site would point at an asset that
// is on its way out.
func TestBackfillSiteIcon_SkipsSoftDeletedAsset(t *testing.T) {
	db := legacyDB(t, "/uploads/deadbeef.png")
	asset := model.Asset{StorageKey: "deadbeef.png", URL: "/uploads/deadbeef.png"}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	if err := db.Delete(&asset).Error; err != nil {
		t.Fatalf("soft delete asset: %v", err)
	}

	if err := backfillSiteIcon(db); err != nil {
		t.Fatalf("backfillSiteIcon: %v", err)
	}

	var config model.GeneralSettings
	if err := db.First(&config).Error; err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if config.SiteIconID != nil {
		t.Errorf("site_icon_id = %d, want no reference to a soft-deleted asset", *config.SiteIconID)
	}
}
