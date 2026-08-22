package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newMigratedDB(t *testing.T) *gorm.DB {
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
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestOpen_DefaultSQLite(t *testing.T) {
	t.Setenv("DB_TYPE", "")
	dataDir := t.TempDir()

	db, err := Open(&config.Config{}, dataDir)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	// Default SQLite path is dataDir/blog.db
	if _, err := os.Stat(filepath.Join(dataDir, "blog.db")); err != nil {
		t.Errorf("expected blog.db created in dataDir: %v", err)
	}
}

func TestOpen_CreatesMissingDataDir(t *testing.T) {
	t.Setenv("DB_TYPE", "")
	dataDir := filepath.Join(t.TempDir(), "nested", "dir")

	if _, err := Open(&config.Config{}, dataDir); err != nil {
		t.Fatalf("Open error: %v", err)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("expected dataDir created: %v", err)
	}
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
	if err := AutoMigrate(db); err != nil {
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

	// Re-running AutoMigrate must deduplicate and recreate the index.
	if err := AutoMigrate(db); err != nil {
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
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate error: %v", err)
	}

	// Every model registered in AutoMigrate must produce a table.
	for _, m := range []any{
		&model.Post{},
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

func TestSeed_CreatesDefaults(t *testing.T) {
	db := newMigratedDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("Seed error: %v", err)
	}

	// Default super admin with a bcrypt-hashed password.
	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("expected admin user seeded: %v", err)
	}
	if admin.Role != model.RoleSuperAdmin || !admin.EmailVerified {
		t.Errorf("expected super admin + verified, got role=%q verified=%v", admin.Role, admin.EmailVerified)
	}
	if admin.Password == "" {
		t.Errorf("expected hashed admin password")
	}

	// Default SMTP config.
	var smtp model.SMTPConfig
	if err := db.First(&smtp).Error; err != nil {
		t.Errorf("expected SMTP config seeded: %v", err)
	}
	if smtp.Port != 587 || smtp.FromName != "VexGo" {
		t.Errorf("unexpected default SMTP config: %+v", smtp)
	}

	// Default general settings.
	var general model.GeneralSettings
	if err := db.First(&general).Error; err != nil {
		t.Errorf("expected general settings seeded: %v", err)
	}
	if !general.RegistrationEnabled || !general.AllowGuestViewPosts {
		t.Errorf("expected registration + guest viewing enabled by default: %+v", general)
	}

	// Default AI config.
	var ai model.AIConfig
	if err := db.First(&ai).Error; err != nil {
		t.Errorf("expected AI config seeded: %v", err)
	}
	if ai.Provider != "openai" || ai.ModelName != "gpt-3.5-turbo" {
		t.Errorf("unexpected default AI config: %+v", ai)
	}

	// Default theme config.
	var theme model.ThemeConfig
	if err := db.First(&theme).Error; err != nil {
		t.Errorf("expected theme config seeded: %v", err)
	}
	if theme.ActiveTheme != "default" {
		t.Errorf("expected default theme, got %q", theme.ActiveTheme)
	}

	// Default category.
	var cat model.Category
	if err := db.Where("name = ?", "Default").First(&cat).Error; err != nil {
		t.Errorf("expected default category seeded: %v", err)
	}
}

func TestSeed_Idempotent(t *testing.T) {
	db := newMigratedDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("first Seed error: %v", err)
	}
	if err := Seed(db); err != nil {
		t.Fatalf("second Seed error: %v", err)
	}

	var admins int64
	if err := db.Model(&model.User{}).Where("username = ?", "admin").Count(&admins).Error; err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if admins != 1 {
		t.Errorf("expected exactly 1 admin after double seed, got %d", admins)
	}

	var cats int64
	if err := db.Model(&model.Category{}).Count(&cats).Error; err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if cats != 1 {
		t.Errorf("expected exactly 1 category after double seed, got %d", cats)
	}
}

func TestSeed_PreservesExistingAdmin(t *testing.T) {
	db := newMigratedDB(t)
	// Pre-existing admin with a custom email must not be overwritten.
	existing := model.User{
		Username:      "admin",
		Email:         "custom-admin@example.com",
		Password:      "hash",
		Role:          model.RoleSuperAdmin,
		EmailVerified: true,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed existing admin: %v", err)
	}

	if err := Seed(db); err != nil {
		t.Fatalf("Seed error: %v", err)
	}
	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("reload admin: %v", err)
	}
	if admin.Email != "custom-admin@example.com" {
		t.Errorf("expected existing admin email preserved, got %q", admin.Email)
	}
}
