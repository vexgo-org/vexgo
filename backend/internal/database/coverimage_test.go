package database

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// legacyPostRow is one pre-refactor post: the only cover it recorded was the
// cover_image URL, and cover_image_id is empty for the backfill to fill.
type legacyPostRow struct {
	slug       string
	coverImage string
}

// legacyPostDB opens a migrated database whose posts table still has the
// legacy cover_image column, seeded with one post per row. Every post gets its
// own slug so a failure names the row that broke.
func legacyPostDB(t *testing.T, rows ...legacyPostRow) *gorm.DB {
	t.Helper()
	db := newMigratedDB(t)

	// An upgrading install keeps its cover_image URL column and gains the
	// cover_image_id reference beside it, so adding the column to the migrated
	// table reproduces the upgrade without rebuilding the table by hand.
	if err := db.Exec("ALTER TABLE posts ADD COLUMN cover_image text").Error; err != nil {
		t.Fatalf("add legacy cover column: %v", err)
	}

	for _, row := range rows {
		if err := db.Exec(
			"INSERT INTO posts (slug, title, cover_image) VALUES (?, ?, ?)",
			row.slug, row.slug, row.coverImage,
		).Error; err != nil {
			t.Fatalf("seed post %q: %v", row.slug, err)
		}
	}

	return db
}

// seedCoverAsset inserts the asset a legacy cover URL resolves to. Asset reuses
// the media_files table, so an upgraded installation already has the current
// columns and nothing here needs rewinding.
func seedCoverAsset(t *testing.T, db *gorm.DB, key string) model.Asset {
	t.Helper()
	user := model.User{Username: "legacy-" + key, Email: "legacy-" + key + "@example.com", Role: model.RoleContributor}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	asset := model.Asset{
		StorageKey: key,
		URL:        "/uploads/" + key,
		UserID:     user.ID,
		MimeType:   model.MimeTypeFromName(key),
		Type:       model.AssetTypeFromMIME(model.MimeTypeFromName(key)),
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return asset
}

// loadPost reads one post back, failing the test if its cover is gone.
func loadPost(t *testing.T, db *gorm.DB, slug string) model.Post {
	t.Helper()
	var post model.Post
	if err := db.Preload("CoverImage").Where("slug = ?", slug).First(&post).Error; err != nil {
		t.Fatalf("load post %q: %v", slug, err)
	}
	return post
}

// assertNoCover pins the no-reference outcome. One helper for every case that
// has to leave a post uncovered, so each reads as the single claim it makes.
func assertNoCover(t *testing.T, post model.Post) {
	t.Helper()
	if post.CoverImageID != nil {
		t.Errorf("cover_image_id = %d, want no reference", *post.CoverImageID)
	}
	if post.CoverImage != nil {
		t.Errorf("cover image = %+v, want no asset", post.CoverImage)
	}
}

// TestBackfillCoverImage_LinksTheReferencedAsset is the upgrade path: a post
// whose cover was a URL gets cover_image_id pointing at the same upload,
// without the author touching anything.
func TestBackfillCoverImage_LinksTheReferencedAsset(t *testing.T) {
	db := legacyPostDB(t, legacyPostRow{slug: "first", coverImage: "/uploads/deadbeef.png"})
	asset := seedCoverAsset(t, db, "deadbeef.png")

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("backfillCoverImage: %v", err)
	}

	post := loadPost(t, db, "first")
	if post.CoverImageID == nil || *post.CoverImageID != asset.ID {
		t.Fatalf("cover_image_id = %v, want %d", post.CoverImageID, asset.ID)
	}
	if post.CoverImage == nil || post.CoverImage.URL != "/uploads/deadbeef.png" {
		t.Errorf("cover image = %+v, want the asset at /uploads/deadbeef.png", post.CoverImage)
	}
}

// TestBackfillCoverImage_LinksS3Covers pins the other storage backend: an S3
// cover is a full URL, and matching has to resolve the same asset a local
// cover would instead of only recognising the /uploads/ prefix.
func TestBackfillCoverImage_LinksS3Covers(t *testing.T) {
	const coverURL = "https://bucket.s3.example.com/2026/deadbeef.png"

	db := legacyPostDB(t, legacyPostRow{slug: "s3", coverImage: coverURL})
	asset := seedCoverAsset(t, db, "2026/deadbeef.png")

	// Give the asset the S3 URL it would really be served from, so the only
	// thing the two share is the storage key the cover URL resolves to.
	if err := db.Model(&asset).Update("url", coverURL).Error; err != nil {
		t.Fatalf("set asset url: %v", err)
	}

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("backfillCoverImage: %v", err)
	}

	post := loadPost(t, db, "s3")
	if post.CoverImageID == nil || *post.CoverImageID != asset.ID {
		t.Errorf("cover_image_id = %v, want %d", post.CoverImageID, asset.ID)
	}
}

// TestBackfillCoverImage_LinksEveryPost guards the batch: an install has many
// posts, and a backfill that only links the first row leaves the rest of the
// archive without covers.
func TestBackfillCoverImage_LinksEveryPost(t *testing.T) {
	db := legacyPostDB(t,
		legacyPostRow{slug: "alpha", coverImage: "/uploads/alpha.png"},
		legacyPostRow{slug: "beta", coverImage: "/uploads/beta.png"},
		legacyPostRow{slug: "gamma", coverImage: "/uploads/gamma.png"},
	)
	wants := map[string]uint{
		"alpha": seedCoverAsset(t, db, "alpha.png").ID,
		"beta":  seedCoverAsset(t, db, "beta.png").ID,
		"gamma": seedCoverAsset(t, db, "gamma.png").ID,
	}

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("backfillCoverImage: %v", err)
	}

	for slug, wantID := range wants {
		post := loadPost(t, db, slug)
		if post.CoverImageID == nil || *post.CoverImageID != wantID {
			t.Errorf("%s: cover_image_id = %v, want %d", slug, post.CoverImageID, wantID)
		}
	}
}

// TestBackfillCoverImage_Idempotent pins re-runnability: AutoMigrate runs on
// every boot, and the legacy column keeps its stale value forever, so a second
// pass must not move a reference the author already changed.
func TestBackfillCoverImage_Idempotent(t *testing.T) {
	db := legacyPostDB(t, legacyPostRow{slug: "first", coverImage: "/uploads/deadbeef.png"})
	first := seedCoverAsset(t, db, "deadbeef.png")
	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("first backfill: %v", err)
	}

	// The admin swaps the cover after the upgrade. The legacy column still
	// names the original upload, so a backfill that trusted it would revert
	// the change on the next boot.
	replacement := seedCoverAsset(t, db, "cafe.png")
	if err := db.Model(&model.Post{}).
		Where("slug = ?", "first").
		Update("cover_image_id", replacement.ID).Error; err != nil {
		t.Fatalf("swap cover: %v", err)
	}
	second := seedCoverAsset(t, db, "deadbeef-alt.png")

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	post := loadPost(t, db, "first")
	if post.CoverImageID == nil || *post.CoverImageID != replacement.ID {
		t.Errorf("cover_image_id = %v, want the replacement %d (first asset %d, second %d)",
			post.CoverImageID, replacement.ID, first.ID, second.ID)
	}
}

// TestBackfillCoverImage_LeavesUnresolvableCoversAlone covers the cases that
// resolve to nothing: an externally hosted cover, a URL whose upload is gone,
// and a post that never had one. All three must end up with no reference rather
// than fail the migration.
func TestBackfillCoverImage_LeavesUnresolvableCoversAlone(t *testing.T) {
	for name, cover := range map[string]string{
		"external": "https://example.com/cover.png",
		"missing":  "/uploads/gone.png",
		"empty":    "",
	} {
		t.Run(name, func(t *testing.T) {
			db := legacyPostDB(t, legacyPostRow{slug: "unresolved", coverImage: cover})

			if err := backfillCoverImage(db); err != nil {
				t.Fatalf("backfillCoverImage: %v", err)
			}

			assertNoCover(t, loadPost(t, db, "unresolved"))
		})
	}
}

// TestBackfillCoverImage_LeavesNullCoverAlone pins the other empty shape: the
// legacy column is nullable, and GORM writes NULL rather than ” for a post
// saved without a cover. Reading NULL into a scan target fails, so the backfill
// has to treat it as no cover rather than abort the whole migration.
func TestBackfillCoverImage_LeavesNullCoverAlone(t *testing.T) {
	db := legacyPostDB(t, legacyPostRow{slug: "nullish", coverImage: ""})
	if err := db.Exec("UPDATE posts SET cover_image = NULL WHERE slug = ?", "nullish").Error; err != nil {
		t.Fatalf("null out legacy cover: %v", err)
	}

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("backfillCoverImage: %v", err)
	}

	assertNoCover(t, loadPost(t, db, "nullish"))
}

// TestBackfillCoverImage_SkipsSoftDeletedAsset guards the soft-delete boundary:
// a pruned cover must not be linked back, or the post would point at an asset
// that is on its way out.
func TestBackfillCoverImage_SkipsSoftDeletedAsset(t *testing.T) {
	db := legacyPostDB(t, legacyPostRow{slug: "pruned", coverImage: "/uploads/deadbeef.png"})
	asset := seedCoverAsset(t, db, "deadbeef.png")
	if err := db.Delete(&asset).Error; err != nil {
		t.Fatalf("soft delete asset: %v", err)
	}

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("backfillCoverImage: %v", err)
	}

	assertNoCover(t, loadPost(t, db, "pruned"))
}

// TestBackfillCoverImage_NoLegacyColumn pins the fresh-install path: a database
// that never had the URL column must pass through untouched rather than fail on
// a column that was never there.
func TestBackfillCoverImage_NoLegacyColumn(t *testing.T) {
	db := newMigratedDB(t)
	user := model.User{Username: "fresh", Email: "fresh@example.com", Role: model.RoleAuthor}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	asset := seedCoverAsset(t, db, "fresh.png")
	post := model.Post{
		Slug:         "fresh",
		Title:        "fresh",
		Content:      "body",
		AuthorID:     user.ID,
		CoverImageID: &asset.ID,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if err := backfillCoverImage(db); err != nil {
		t.Fatalf("backfillCoverImage: %v", err)
	}

	got := loadPost(t, db, "fresh")
	if got.CoverImageID == nil || *got.CoverImageID != asset.ID {
		t.Errorf("cover_image_id = %v, want the untouched %d", got.CoverImageID, asset.ID)
	}
}
