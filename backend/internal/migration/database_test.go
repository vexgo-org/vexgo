package migration

import (
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newAutoMigratedDB opens an in-memory SQLite database with the schema created
// by autoMigrate. The pool is capped at one connection so the in-memory
// database is not lost between statements.
func newAutoMigratedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := autoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestAutoMigrate_DeduplicatesExistingLikes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := autoMigrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Simulate legacy data: drop the unique index, insert duplicate likes.
	if err := db.Migrator().DropIndex(&model.Like{}, "idx_likes_post_user"); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if err := db.Create(&model.Like{PostID: 1, UserID: 1}).Error; err != nil {
		t.Fatalf("seed like 1: %v", err)
	}
	if err := db.Create(&model.Like{PostID: 1, UserID: 1}).Error; err != nil {
		t.Fatalf("seed duplicate like: %v", err)
	}
	if err := db.Create(&model.Like{PostID: 1, UserID: 2}).Error; err != nil {
		t.Fatalf("seed like 2: %v", err)
	}

	// Re-running autoMigrate must deduplicate and recreate the index.
	if err := autoMigrate(db); err != nil {
		t.Fatalf("second migrate with duplicates: %v", err)
	}
	var count int64
	if err := db.Model(&model.Like{}).Count(&count).Error; err != nil {
		t.Fatalf("count likes: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 likes after dedupe, got %d", count)
	}
}

func TestAutoMigrate_CreatesAllTables(t *testing.T) {
	db := newAutoMigratedDB(t)

	// Every model registered in autoMigrate must produce a table.
	for _, m := range []any{
		&model.Post{},
		&model.Page{},
		&model.User{},
		&model.Tag{},
		&model.Category{},
		&model.Comment{},
		&model.Like{},
		&model.MediaFile{},
		&model.SMTPConfig{},
		&model.Captcha{},
		&model.GeneralSettings{},
		&model.CommentModerationConfig{},
		&model.AIConfig{},
		&model.SSOBinding{},
		&model.ThemeConfig{},
		&model.Notification{},
		&model.CreatorApplication{},
	} {
		if !db.Migrator().HasTable(m) {
			t.Errorf("expected table for %T", m)
		}
	}
}

func TestMigrateDB_RunsOnFreshAndExistingSchema(t *testing.T) {
	// MigrateDB must be safe on a fresh database and idempotent afterwards.
	svc, _, _ := newMigrationService(t)

	if err := svc.MigrateDB(); err != nil {
		t.Fatalf("first MigrateDB: %v", err)
	}
	if err := svc.MigrateDB(); err != nil {
		t.Fatalf("second MigrateDB: %v", err)
	}
}
