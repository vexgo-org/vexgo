package migration

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// seedLegacyAvatarUser creates a user and creates the legacy avatar column,
// which AutoMigrate no longer creates, then stores avatar in it.
func seedLegacyAvatarUser(t *testing.T, db *gorm.DB, username, email, avatar string) uint {
	t.Helper()

	user := model.User{Username: username, Email: email, Role: model.RoleGuest}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user %q: %v", username, err)
	}
	if !db.Migrator().HasColumn(&model.User{}, "avatar") {
		if err := db.Exec("ALTER TABLE users ADD COLUMN avatar text").Error; err != nil {
			t.Fatalf("add legacy users.avatar column: %v", err)
		}
	}
	if err := db.Exec("UPDATE users SET avatar = ? WHERE id = ?", avatar, user.ID).Error; err != nil {
		t.Fatalf("seed legacy user avatar: %v", err)
	}

	return user.ID
}

func reloadUser(t *testing.T, db *gorm.DB, id uint) model.User {
	t.Helper()

	var user model.User
	if err := db.First(&user, id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return user
}

func findAssetByOriginalName(t *testing.T, db *gorm.DB, name string) model.Asset {
	t.Helper()

	var asset model.Asset
	if err := db.Where("original_name = ?", name).First(&asset).Error; err != nil {
		t.Fatalf("find migrated asset %q: %v", name, err)
	}
	return asset
}

func TestMigrateDB_BackfillsUserAvatarFromLegacyUpload(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "user-avatar-local.png"
	putObject(t, local, key, pngPayload)

	userID := seedLegacyAvatarUser(t, db, "local", "local@example.com", "/uploads/"+key)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, UserID: userID})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	asset := findAssetByOriginalName(t, db, key)
	user := reloadUser(t, db, userID)
	if user.AvatarID == nil || *user.AvatarID != asset.ID {
		t.Fatalf("user avatar id = %v, want %d", user.AvatarID, asset.ID)
	}
	if db.Migrator().HasColumn(&model.User{}, "avatar") {
		t.Error("legacy users.avatar column still exists after the migration")
	}
}

// S3 media rows store a full URL, so the backfill must match media_files.url
// rather than assuming an /uploads/ prefix.
func TestMigrateDB_BackfillsUserAvatarFromS3URL(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "user-avatar-s3.png"
	const url = "https://cdn.example.com/blog/" + key
	putObject(t, local, key, pngPayload)

	userID := seedLegacyAvatarUser(t, db, "s3", "s3@example.com", url)
	seedMediaFile(t, db, model.MediaFile{
		URL:        url,
		StorageKey: key,
		UserID:     userID,
	})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	asset := findAssetByOriginalName(t, db, key)
	user := reloadUser(t, db, userID)
	if user.AvatarID == nil || *user.AvatarID != asset.ID {
		t.Fatalf("user avatar id = %v, want %d", user.AvatarID, asset.ID)
	}
	if db.Migrator().HasColumn(&model.User{}, "avatar") {
		t.Error("legacy users.avatar column still exists after the migration")
	}
}

func TestMigrateDB_SkipsUnmappableUserAvatar(t *testing.T) {
	s, db, _ := newMigrationService(t)

	userID := seedLegacyAvatarUser(t, db, "external", "external@example.com", "https://example.com/avatar.png")

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if user := reloadUser(t, db, userID); user.AvatarID != nil {
		t.Fatalf("user avatar id = %d, want NULL", *user.AvatarID)
	}
	if db.Migrator().HasColumn(&model.User{}, "avatar") {
		t.Error("legacy users.avatar column still exists after the migration")
	}
}

// A legacy avatar URL must not attach another user's media asset.
func TestMigrateDB_SkipsUserAvatarOwnedByAnotherUser(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "user-avatar-owned.png"
	putObject(t, local, key, pngPayload)

	ownerID := seedLegacyAvatarUser(t, db, "owner", "owner@example.com", "")
	otherID := seedLegacyAvatarUser(t, db, "other", "other@example.com", "/uploads/"+key)
	seedMediaFile(t, db, model.MediaFile{
		URL:    "/uploads/" + key,
		UserID: ownerID,
	})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if user := reloadUser(t, db, otherID); user.AvatarID != nil {
		t.Fatalf("other user avatar id = %d, want NULL", *user.AvatarID)
	}
	if db.Migrator().HasColumn(&model.User{}, "avatar") {
		t.Error("legacy users.avatar column still exists after the migration")
	}
}

// TestMigrateDB_DoesNotResurrectClearedUserAvatar pins the one-shot contract:
// the legacy column is gone after the first start, so clearing the avatar does
// not bring it back on the next one.
func TestMigrateDB_DoesNotResurrectClearedUserAvatar(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "user-avatar-cleared.png"
	putObject(t, local, key, pngPayload)

	userID := seedLegacyAvatarUser(t, db, "cleared", "cleared@example.com", "/uploads/"+key)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, UserID: userID})

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := db.Model(&model.User{}).
		Where("id = ?", userID).
		Update("avatar_id", nil).Error; err != nil {
		t.Fatalf("clear user avatar: %v", err)
	}
	if err := s.MigrateDB(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if user := reloadUser(t, db, userID); user.AvatarID != nil {
		t.Fatalf("user avatar id = %d after clearing, want NULL", *user.AvatarID)
	}
}
