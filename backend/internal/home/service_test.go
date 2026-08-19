package home

import (
	"context"
	"testing"

	"vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Post{}, &model.User{}, &model.Category{}, &model.Tag{}, &model.Comment{}, &model.GeneralSettings{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return NewService(Deps{DB: db}), db
}

func TestStats_Counts(t *testing.T) {
	svc, db := newTestService(t)
	u := model.User{Username: "alice", Email: "alice@example.com", Role: model.RoleGuest}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&model.Post{Title: "p", Content: "c", AuthorID: u.ID, Status: model.PostStatusPublished}).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}
	if err := db.Create(&model.Category{Name: "cat"}).Error; err != nil {
		t.Fatalf("failed to seed category: %v", err)
	}
	if err := db.Create(&model.Tag{Name: "tag"}).Error; err != nil {
		t.Fatalf("failed to seed tag: %v", err)
	}
	if err := db.Create(&model.Comment{PostID: 1, UserID: u.ID, Content: "c"}).Error; err != nil {
		t.Fatalf("failed to seed comment: %v", err)
	}

	stats := svc.Stats(context.Background(), "")
	if stats.Posts != 1 || stats.Users != 1 || stats.Categories != 1 || stats.Tags != 1 || stats.Comments != 1 {
		t.Errorf("expected all counts 1, got %+v", stats)
	}
}

func TestStats_GuestViewDisabled(t *testing.T) {
	svc, db := newTestService(t)
	u := model.User{Username: "alice", Email: "alice@example.com", Role: model.RoleGuest}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&model.Post{Title: "p", Content: "c", AuthorID: u.ID, Status: model.PostStatusPublished}).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	// Disable guest viewing directly via Create. AllowGuestViewPosts used to
	// carry gorm:"default:true", which made GORM omit the zero value on Create
	// and silently store true — that bug is now fixed.
	if err := db.Create(&model.GeneralSettings{AllowGuestViewPosts: false}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	// anonymous → empty stats
	stats := svc.Stats(context.Background(), "")
	if stats.Posts != 0 {
		t.Errorf("expected zero posts for anonymous viewer, got %+v", stats)
	}

	// logged-in user sees stats
	stats = svc.Stats(context.Background(), model.RoleGuest)
	if stats.Posts != 1 {
		t.Errorf("expected 1 post for logged-in viewer, got %+v", stats)
	}
}

func TestStats_NoSettingsDefaultsToAllowed(t *testing.T) {
	svc, _ := newTestService(t)
	// no GeneralSettings row → guest viewing allowed by default
	stats := svc.Stats(context.Background(), "")
	if stats.Posts != 0 {
		t.Errorf("expected empty stats, got %+v", stats)
	}
}
