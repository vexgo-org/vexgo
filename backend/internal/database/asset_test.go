package database

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newAssetDB opens an in-memory SQLite database holding only the tables the
// asset backfill touches. The legacy shape lives in the row data rather than
// the schema, because Asset reuses the existing media_files table, so an
// upgraded installation already has the current columns.
func newAssetDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedLegacyAsset inserts one pre-Asset media row: the only field the old
// MediaFile model recorded was the URL, so storage key, original name, MIME
// type and asset type are all empty for the backfill to derive.
func seedLegacyAsset(t *testing.T, db *gorm.DB, url string) model.Asset {
	t.Helper()
	user := model.User{Username: "legacy", Email: "legacy@example.com", Role: model.RoleContributor}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	row := model.Asset{URL: url, UserID: user.ID}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed legacy asset: %v", err)
	}
	return row
}

// assertBackfilled checks the fields backFillAssets derives from the URL alone.
func assertBackfilled(t *testing.T, asset model.Asset, wantKey string) {
	t.Helper()
	if asset.StorageKey != wantKey {
		t.Errorf("storage key = %q, want %q", asset.StorageKey, wantKey)
	}
	if asset.OriginalName != wantKey {
		t.Errorf("original name = %q, want %q", asset.OriginalName, wantKey)
	}
	if asset.MimeType != "image/png" {
		t.Errorf("mime type = %q, want image/png", asset.MimeType)
	}
	if asset.Type != model.AssetTypeImage {
		t.Errorf("asset type = %q, want %q", asset.Type, model.AssetTypeImage)
	}
}

// TestBackFillAssets_BackfillsSoftDeletedRow guards the soft-deleted case.
// backFillAssets reads Unscoped() so it deliberately covers rows an admin
// deleted before the upgrade, and the update has to be Unscoped() as well:
// GORM scopes an update with deleted_at IS NULL, so a scoped write silently
// matches nothing and the deleted row keeps an empty storage_key. That column
// is unique, so several such rows collide on the next migration.
func TestBackFillAssets_BackfillsSoftDeletedRow(t *testing.T) {
	db := newAssetDB(t)
	row := seedLegacyAsset(t, db, "/uploads/legacy-photo.png")

	if err := db.Delete(&model.Asset{}, row.ID).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if err := backFillAssets(db); err != nil {
		t.Fatalf("backFillAssets: %v", err)
	}

	var after model.Asset
	if err := db.Unscoped().First(&after, row.ID).Error; err != nil {
		t.Fatalf("reload soft-deleted row: %v", err)
	}
	assertBackfilled(t, after, "legacy-photo.png")
}

// TestBackFillAssets_BackfillsLiveRow pins the ordinary path, so the
// soft-deleted case above cannot be satisfied by weakening the backfill.
func TestBackFillAssets_BackfillsLiveRow(t *testing.T) {
	db := newAssetDB(t)
	row := seedLegacyAsset(t, db, "/uploads/legacy-photo.png")

	if err := backFillAssets(db); err != nil {
		t.Fatalf("backFillAssets: %v", err)
	}

	var after model.Asset
	if err := db.First(&after, row.ID).Error; err != nil {
		t.Fatalf("reload row: %v", err)
	}
	assertBackfilled(t, after, "legacy-photo.png")
}

// TestBackFillAssets_LeavesCompleteRowsAlone checks the backfill only fills
// gaps: it runs on every startup, so a row that already has every field must
// not be rewritten.
func TestBackFillAssets_LeavesCompleteRowsAlone(t *testing.T) {
	db := newAssetDB(t)
	row := model.Asset{
		OriginalName: "photo.png",
		StorageKey:   "already-keyed.png",
		URL:          "/uploads/already-keyed.png",
		MimeType:     "image/png",
		Type:         model.AssetTypeImage,
	}
	user := model.User{Username: "u", Email: "u@example.com", Role: model.RoleContributor}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	row.UserID = user.ID
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}
	before := row.UpdatedAt

	if err := backFillAssets(db); err != nil {
		t.Fatalf("backFillAssets: %v", err)
	}

	var after model.Asset
	if err := db.First(&after, row.ID).Error; err != nil {
		t.Fatalf("reload row: %v", err)
	}
	// Equal, not ==: the SQLite round-trip normalises the time location, so
	// comparing the structs directly would report a rewrite that never happened.
	if !after.UpdatedAt.Equal(before) {
		t.Errorf("complete row was rewritten: updated_at %v -> %v", before, after.UpdatedAt)
	}
	if after.StorageKey != "already-keyed.png" {
		t.Errorf("storage key = %q, want already-keyed.png", after.StorageKey)
	}
}
