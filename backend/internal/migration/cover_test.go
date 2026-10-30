package migration

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// mp4Payload sniffs as video/mp4, so backfillAssetBlob stores a non-image blob.
const mp4Payload = "\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"

// seedLegacyPostCover creates a post, creates the legacy cover_image column,
// which AutoMigrate no longer creates, then stores coverImage in it.
func seedLegacyPostCover(t *testing.T, db *gorm.DB, slug, coverImage string) uint {
	t.Helper()

	post := model.Post{Slug: slug, Title: slug, Content: "content"}
	if err := db.Omit("Author").Create(&post).Error; err != nil {
		t.Fatalf("seed post %q: %v", slug, err)
	}
	if !db.Migrator().HasColumn(&model.Post{}, "cover_image") {
		if err := db.Exec("ALTER TABLE posts ADD COLUMN cover_image text").Error; err != nil {
			t.Fatalf("add legacy posts.cover_image column: %v", err)
		}
	}
	if err := db.Exec("UPDATE posts SET cover_image = ? WHERE id = ?", coverImage, post.ID).Error; err != nil {
		t.Fatalf("seed legacy post cover image: %v", err)
	}

	return post.ID
}

func reloadPost(t *testing.T, db *gorm.DB, id uint) model.Post {
	t.Helper()

	var post model.Post
	if err := db.First(&post, id).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	return post
}

func TestMigrateDB_BackfillsPostCoverFromLegacyUpload(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "post-cover-local.png"
	putObject(t, local, key, pngPayload)

	postID := seedLegacyPostCover(t, db, "cover-local", "/uploads/"+key)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	asset := findAssetByOriginalName(t, db, key)
	post := reloadPost(t, db, postID)
	if post.CoverImageID == nil || *post.CoverImageID != asset.ID {
		t.Fatalf("post cover image id = %v, want %d", post.CoverImageID, asset.ID)
	}
	if db.Migrator().HasColumn(&model.Post{}, "cover_image") {
		t.Error("legacy posts.cover_image column still exists after the migration")
	}
}

// S3 media rows store a full URL, so the backfill must match media_files.url
// rather than assuming an /uploads/ prefix.
func TestMigrateDB_BackfillsPostCoverFromS3URL(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "post-cover-s3.png"
	const url = "https://cdn.example.com/blog/" + key
	putObject(t, local, key, pngPayload)

	postID := seedLegacyPostCover(t, db, "cover-s3", url)
	seedMediaFile(t, db, model.MediaFile{URL: url, StorageKey: key})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	asset := findAssetByOriginalName(t, db, key)
	post := reloadPost(t, db, postID)
	if post.CoverImageID == nil || *post.CoverImageID != asset.ID {
		t.Fatalf("post cover image id = %v, want %d", post.CoverImageID, asset.ID)
	}
	if db.Migrator().HasColumn(&model.Post{}, "cover_image") {
		t.Error("legacy posts.cover_image column still exists after the migration")
	}
}

// A cover set through the current asset API must win over the legacy URL.
func TestMigrateDB_KeepsExistingPostCover(t *testing.T) {
	s, db, local := newMigrationService(t)

	const legacyKey = "post-cover-legacy.png"
	const currentKey = "post-cover-current.png"
	putObject(t, local, legacyKey, pngPayload)
	putObject(t, local, currentKey, pngPayload)

	postID := seedLegacyPostCover(t, db, "cover-existing", "/uploads/"+legacyKey)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + legacyKey})
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + currentKey})

	if err := s.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill assets: %v", err)
	}
	current := findAssetByOriginalName(t, db, currentKey)
	if err := db.Model(&model.Post{}).
		Where("id = ?", postID).
		Update("cover_image_id", current.ID).Error; err != nil {
		t.Fatalf("set current post cover: %v", err)
	}

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	post := reloadPost(t, db, postID)
	if post.CoverImageID == nil || *post.CoverImageID != current.ID {
		t.Fatalf("post cover image id = %v, want existing %d", post.CoverImageID, current.ID)
	}
	if db.Migrator().HasColumn(&model.Post{}, "cover_image") {
		t.Error("legacy posts.cover_image column still exists after the migration")
	}
}

func TestMigrateDB_SkipsNonImagePostCover(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "post-cover-video.mp4"
	putObject(t, local, key, mp4Payload)

	postID := seedLegacyPostCover(t, db, "cover-video", "/uploads/"+key)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if post := reloadPost(t, db, postID); post.CoverImageID != nil {
		t.Fatalf("post cover image id = %d, want NULL", *post.CoverImageID)
	}
	if db.Migrator().HasColumn(&model.Post{}, "cover_image") {
		t.Error("legacy posts.cover_image column still exists after the migration")
	}
}

func TestMigrateDB_SkipsUnmappablePostCover(t *testing.T) {
	s, db, _ := newMigrationService(t)

	postID := seedLegacyPostCover(t, db, "cover-external", "https://example.com/cover.png")

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if post := reloadPost(t, db, postID); post.CoverImageID != nil {
		t.Fatalf("post cover image id = %d, want NULL", *post.CoverImageID)
	}
	if db.Migrator().HasColumn(&model.Post{}, "cover_image") {
		t.Error("legacy posts.cover_image column still exists after the migration")
	}
}

// TestMigrateDB_DoesNotResurrectClearedPostCover pins the one-shot contract:
// the legacy column is gone after the first start, so clearing the cover does
// not bring it back on the next one.
func TestMigrateDB_DoesNotResurrectClearedPostCover(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "post-cover-cleared.png"
	putObject(t, local, key, pngPayload)

	postID := seedLegacyPostCover(t, db, "cover-cleared", "/uploads/"+key)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := db.Model(&model.Post{}).
		Where("id = ?", postID).
		Update("cover_image_id", nil).Error; err != nil {
		t.Fatalf("clear post cover: %v", err)
	}
	if err := s.MigrateDB(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if post := reloadPost(t, db, postID); post.CoverImageID != nil {
		t.Fatalf("post cover image id = %d after clearing, want NULL", *post.CoverImageID)
	}
}
