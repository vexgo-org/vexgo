package notification

import (
	"context"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestDB opens an isolated in-memory SQLite database with a single
// connection so all queries hit the same database instance.
func newTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.Notification{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, Repository) {
	db := newTestDB(t)
	repo := NewRepository(db)
	return newServiceWithRepo(repo), repo
}

func TestCreateNotification(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	err := svc.CreateNotification(ctx, model.NotificationInput{
		UserID:      1,
		Type:        model.NotificationTypeComment,
		Title:       "New comment",
		Content:     "someone commented",
		RelatedID:   "42",
		RelatedType: model.NotificationRelatedTypePost,
	})
	if err != nil {
		t.Fatalf("CreateNotification error: %v", err)
	}

	list, total, err := repo.List(ctx, 1, 0, 10, "", "")
	if err != nil {
		t.Fatalf("repo.List error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 notification, got %d", total)
	}
	n := list[0]
	if n.UserID != 1 || n.Type != model.NotificationTypeComment || n.Title != "New comment" {
		t.Errorf("unexpected notification: %+v", n)
	}
	if n.IsRead {
		t.Errorf("expected new notification to be unread")
	}
}

func TestList_PaginationAndFilters(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	for range 5 {
		if err := svc.CreateNotification(ctx, model.NotificationInput{UserID: 1, Type: model.NotificationTypeComment, Title: "c", Content: "content"}); err != nil {
			t.Fatalf("failed to seed: %v", err)
		}
	}

	list, total, err := svc.List(ctx, ListQuery{UserID: 1, Page: 1, Limit: 2})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 items on page 1, got %d", len(list))
	}

	list, total, err = svc.List(ctx, ListQuery{UserID: 1, Page: 1, Limit: 10, IsRead: "false"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 unread, got %d", total)
	}
	if len(list) != 5 {
		t.Errorf("expected 5 unread items, got %d", len(list))
	}

	_, total, err = svc.List(ctx, ListQuery{UserID: 1, Page: 1, Limit: 10, NotificationType: "comment"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 comment notifications, got %d", total)
	}
}

func TestMarkAsRead(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	if err := svc.CreateNotification(ctx, model.NotificationInput{
		UserID:  1,
		Type:    model.NotificationTypeComment,
		Title:   "t",
		Content: "c",
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	list, _, err := repo.List(ctx, 1, 0, 10, "", "")
	if err != nil || len(list) == 0 {
		t.Fatalf("load failed: err=%v len=%d", err, len(list))
	}
	n := list[0]

	rows, err := svc.MarkAsRead(ctx, 1, int(n.ID))
	if err != nil {
		t.Fatalf("MarkAsRead error: %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 1 row affected, got %d", rows)
	}

	readList, _, err := repo.List(ctx, 1, 0, 10, "", "true")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(readList) != 1 {
		t.Errorf("expected 1 read notification, got %d", len(readList))
	}

	rows, err = svc.MarkAsRead(ctx, 2, int(n.ID))
	if err != nil {
		t.Fatalf("MarkAsRead error: %v", err)
	}
	if rows != 0 {
		t.Errorf("expected 0 rows for foreign user, got %d", rows)
	}
}

func TestMarkAllAsRead(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	if err := svc.CreateNotification(ctx, model.NotificationInput{
		UserID:  1,
		Type:    model.NotificationTypeComment,
		Title:   "t",
		Content: "c",
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if err := svc.CreateNotification(ctx, model.NotificationInput{
		UserID:  2,
		Type:    model.NotificationTypeComment,
		Title:   "t",
		Content: "c",
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	if err := svc.MarkAllAsRead(ctx, 1); err != nil {
		t.Fatalf("MarkAllAsRead error: %v", err)
	}

	count, err := svc.UnreadCount(ctx, 1)
	if err != nil {
		t.Fatalf("UnreadCount error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 unread for user 1, got %d", count)
	}
	count, err = svc.UnreadCount(ctx, 2)
	if err != nil {
		t.Fatalf("UnreadCount error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 unread for user 2, got %d", count)
	}
}

func TestDelete(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	if err := svc.CreateNotification(ctx, model.NotificationInput{
		UserID:  1,
		Type:    model.NotificationTypeComment,
		Title:   "t",
		Content: "c",
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	list, _, err := repo.List(ctx, 1, 0, 10, "", "")
	if err != nil || len(list) == 0 {
		t.Fatalf("load failed: err=%v len=%d", err, len(list))
	}
	n := list[0]

	rows, err := svc.Delete(ctx, 2, int(n.ID))
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if rows != 0 {
		t.Errorf("expected 0 rows for foreign user, got %d", rows)
	}

	rows, err = svc.Delete(ctx, 1, int(n.ID))
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 1 row affected, got %d", rows)
	}

	count, err := svc.UnreadCount(ctx, 1)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 notifications left, got %d", count)
	}
}
