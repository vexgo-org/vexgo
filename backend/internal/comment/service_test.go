package comment

import (
	"errors"
	"strconv"
	"testing"

	"vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fakeNotifier records notification calls instead of touching the DB.
type fakeNotifier struct {
	calls []string
}

func (f *fakeNotifier) CreateNotification(userID uint, notificationType, title, content, relatedID, relatedType string) error {
	f.calls = append(f.calls, notificationType)
	return nil
}

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
	if err := db.AutoMigrate(&model.Comment{}, &model.Post{}, &model.User{}, &model.CommentModerationConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *fakeNotifier, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	notifier := &fakeNotifier{}
	svc := NewService(Deps{DB: db, Notifier: notifier})
	return svc, notifier, db
}

func seedUser(t *testing.T, db *gorm.DB, username, role string) model.User {
	t.Helper()
	u := model.User{Username: username, Email: username + "@example.com", Role: role}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

func seedPost(t *testing.T, db *gorm.DB, authorID uint) model.Post {
	t.Helper()
	p := model.Post{Title: "Post", Content: "body", Category: "1", AuthorID: authorID, Status: model.PostStatusPublished}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}
	return p
}

func TestCreate_AutoApproved(t *testing.T) {
	svc, notifier, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	comment, count, err := svc.Create(post.ID, commenter.ID, "nice post", nil)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusPublished {
		t.Errorf("expected published, got %s", comment.Status)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != "comment" {
		t.Errorf("expected one comment notification, got %v", notifier.calls)
	}

	// author commenting on own post → no notification
	notifier.calls = nil
	_, _, err = svc.Create(post.ID, author.ID, "self comment", nil)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected no notification for own post, got %v", notifier.calls)
	}
}

func TestCreate_ModerationDisabledManualApproval(t *testing.T) {
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, err := svc.UpdateModerationConfig(UpdateModerationConfigRequest{
		Enabled:            false,
		AutoApproveEnabled: false,
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	comment, _, err := svc.Create(post.ID, commenter.ID, "needs review", nil)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusPending {
		t.Errorf("expected pending, got %s", comment.Status)
	}
}

func TestCreate_ModerationRejectsBlockedKeyword(t *testing.T) {
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, err := svc.UpdateModerationConfig(UpdateModerationConfigRequest{
		Enabled:       true,
		BlockKeywords: "spam,ad",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	comment, _, err := svc.Create(post.ID, commenter.ID, "buy now spam", nil)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusRejected {
		t.Errorf("expected rejected, got %s", comment.Status)
	}
}

func TestCreate_ReplyNotifiesParentAuthor(t *testing.T) {
	svc, notifier, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)
	replier := seedUser(t, db, "replier", model.RoleGuest)

	parentID := uint(0)
	_, _, err := svc.Create(post.ID, commenter.ID, "first", nil)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var parent model.Comment
	if err := db.First(&parent).Error; err != nil {
		t.Fatalf("failed to load parent: %v", err)
	}
	parentID = parent.ID

	notifier.calls = nil
	_, _, err = svc.Create(post.ID, replier.ID, "reply", &parentID)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(notifier.calls) != 2 {
		t.Errorf("expected post + parent notifications, got %v", notifier.calls)
	}
}

func TestListByPost_PublishedOnlyAndPrivacy(t *testing.T) {
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	commenter.ProfileVisibility = "private"
	db.Save(&commenter)

	if _, _, err := svc.Create(post.ID, commenter.ID, "published one", nil); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, _, err := svc.Create(post.ID, author.ID, "pending one", nil); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var pending model.Comment
	if err := db.Where("content = ?", "pending one").First(&pending).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pending.Status = model.CommentStatusPending
	db.Save(&pending)

	comments, err := svc.ListByPost("1", 0, "")
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 published comment, got %d", len(comments))
	}
	if comments[0].User.Email != "" {
		t.Errorf("expected private email filtered for anonymous viewer")
	}

	comments, err = svc.ListByPost("1", author.ID, author.Role)
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if comments[0].User.Email != "" {
		t.Errorf("expected email hidden from author too (not self), got %q", comments[0].User.Email)
	}

	admin := seedUser(t, db, "admin", model.RoleSuperAdmin)
	comments, err = svc.ListByPost("1", admin.ID, admin.Role)
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if comments[0].User.Email == "" {
		t.Errorf("expected admin to see private email")
	}
}

func TestDelete_Permissions(t *testing.T) {
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(post.ID, commenter.ID, "to delete", nil); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var comment model.Comment
	if err := db.First(&comment).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	id := comment.ID

	other := seedUser(t, db, "other", model.RoleGuest)
	if _, err := svc.Delete(strconv.FormatUint(uint64(id), 10), other.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}

	count, err := svc.Delete(strconv.FormatUint(uint64(id), 10), commenter.ID)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 after delete, got %d", count)
	}

	if _, err := svc.Delete(strconv.FormatUint(uint64(id), 10), commenter.ID); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound, got %v", err)
	}
}

func TestSetStatus(t *testing.T) {
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(post.ID, commenter.ID, "moderate me", nil); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var comment model.Comment
	if err := db.First(&comment).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}

	updated, err := svc.SetStatus(strconv.FormatUint(uint64(comment.ID), 10), model.CommentStatusPublished)
	if err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if updated.Status != model.CommentStatusPublished {
		t.Errorf("expected published, got %s", updated.Status)
	}

	if _, err := svc.SetStatus("99999", model.CommentStatusPublished); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound, got %v", err)
	}
}

func TestListModeration(t *testing.T) {
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(post.ID, commenter.ID, "one", nil); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, _, err := svc.Create(post.ID, commenter.ID, "two", nil); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	list, total, err := svc.ListModeration(model.CommentStatusPublished, 1, 1)
	if err != nil {
		t.Fatalf("ListModeration error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 item per page, got %d", len(list))
	}

	list, total, err = svc.ListModeration(model.CommentStatusPending, 1, 10)
	if err != nil {
		t.Fatalf("ListModeration error: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Errorf("expected empty pending queue, got total=%d len=%d", total, len(list))
	}
}

func TestUpdateModerationConfig_PreservesApiKey(t *testing.T) {
	svc, _, db := newTestService(t)

	config, err := svc.UpdateModerationConfig(UpdateModerationConfigRequest{
		Enabled: true,
		ApiKey:  "secret-key",
	})
	if err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}
	if config.ApiKey != "" {
		t.Errorf("expected api key masked in response")
	}

	var stored model.CommentModerationConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if stored.ApiKey != "secret-key" {
		t.Errorf("expected stored api key, got %q", stored.ApiKey)
	}

	_, err = svc.UpdateModerationConfig(UpdateModerationConfigRequest{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if stored.ApiKey != "secret-key" {
		t.Errorf("expected api key preserved, got %q", stored.ApiKey)
	}
}
