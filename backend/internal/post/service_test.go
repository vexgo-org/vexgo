package post

import (
	"errors"
	"strconv"
	"testing"

	"vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeNotifier struct {
	calls []string
}

func (f *fakeNotifier) CreateNotification(userID uint, notificationType, title, content, relatedID, relatedType string) error {
	f.calls = append(f.calls, notificationType)
	return nil
}

type fakeRemover struct {
	deleted []string
}

func (f *fakeRemover) Delete(url string) error {
	f.deleted = append(f.deleted, url)
	return nil
}

func newTestService(t *testing.T) (*Service, *fakeNotifier, *fakeRemover) {
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
	if err := db.AutoMigrate(&model.Post{}, &model.User{}, &model.Tag{}, &model.Like{}, &model.Comment{}, &model.Category{}, &model.GeneralSettings{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	notifier := &fakeNotifier{}
	remover := &fakeRemover{}
	svc := NewService(Deps{DB: db, Notifier: notifier, Files: remover})
	return svc, notifier, remover
}

func seedUser(t *testing.T, db *gorm.DB, username, role string) model.User {
	t.Helper()
	u := model.User{Username: username, Email: username + "@example.com", Role: role}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

func TestCreate_SavesDraftAndPublished(t *testing.T) {
	svc, _, _ := newTestService(t)
	user := seedUser(t, svc.db, "tester", model.RoleContributor)

	post, err := svc.Create(user.Role, user.ID, CreateRequest{
		Title:      "Hello",
		Content:    "world",
		Category:   1, // numeric category simulating frontend behavior
		Tags:       []string{"go", "gin"},
		Excerpt:    "ex",
		CoverImage: "/img.png",
		Status:     model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if post.ID == 0 {
		t.Fatalf("post not saved")
	}

	var stored model.Post
	if err := svc.db.Preload("Tags").First(&stored, post.ID).Error; err != nil {
		t.Fatalf("post not saved: %v", err)
	}
	if stored.Status != model.PostStatusPublished {
		t.Errorf("status expected published, got %s", stored.Status)
	}
	if len(stored.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(stored.Tags))
	}
}

func TestCreate_DerivesStatusByRole(t *testing.T) {
	svc, _, _ := newTestService(t)

	contributor := seedUser(t, svc.db, "contrib", model.RoleContributor)
	post, err := svc.Create(contributor.Role, contributor.ID, CreateRequest{Title: "t", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if post.Status != model.PostStatusPending {
		t.Errorf("contributor post expected pending, got %s", post.Status)
	}

	author := seedUser(t, svc.db, "auth", model.RoleAuthor)
	post, err = svc.Create(author.Role, author.ID, CreateRequest{Title: "t", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if post.Status != model.PostStatusPublished {
		t.Errorf("author post expected published, got %s", post.Status)
	}
}

func TestCreate_ForbidsGuest(t *testing.T) {
	svc, _, _ := newTestService(t)
	guest := seedUser(t, svc.db, "guest", model.RoleGuest)

	if _, err := svc.Create(guest.Role, guest.ID, CreateRequest{Title: "t", Content: "c", Category: "1"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdate_ModifiesFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	user := seedUser(t, svc.db, "tester", model.RoleContributor)

	post, err := svc.Create(user.Role, user.ID, CreateRequest{Title: "A", Content: "B", Category: 1, Status: model.PostStatusDraft})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	updated, err := svc.Update(idString(post.ID), user.ID, UpdateRequest{
		Title:  "New",
		Status: model.PostStatusPublished,
		Tags:   []string{"foo"},
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Title != "New" {
		t.Errorf("title not updated")
	}
	if updated.Status != model.PostStatusPublished {
		t.Errorf("status not updated")
	}
	if len(updated.Tags) != 1 || updated.Tags[0].Name != "foo" {
		t.Errorf("tags not updated: %+v", updated.Tags)
	}

	// other user cannot update
	other := seedUser(t, svc.db, "other", model.RoleGuest)
	if _, err := svc.Update(idString(post.ID), other.ID, UpdateRequest{Title: "hack"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestResolveTags_CreatesIfMissing(t *testing.T) {
	svc, _, _ := newTestService(t)
	if _, err := svc.resolveTags([]string{"x", "y", "x"}); err != nil {
		t.Fatalf("resolveTags error: %v", err)
	}
	var tags []model.Tag
	svc.db.Find(&tags)
	if len(tags) != 2 {
		t.Errorf("expected 2 unique tags, got %d", len(tags))
	}
}

func TestDelete_RemovesFilesAndAssociations(t *testing.T) {
	svc, _, remover := newTestService(t)
	author := seedUser(t, svc.db, "author", model.RoleAuthor)

	post, err := svc.Create(author.Role, author.ID, CreateRequest{
		Title:      "A",
		Content:    "![img](/uploads/a.jpg) and <img src=\"/uploads/b.jpg\">",
		Category:   1,
		CoverImage: "/uploads/cover.jpg",
		Status:     model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// add a like and a comment
	svc.db.Create(&model.Like{PostID: post.ID, UserID: author.ID})
	svc.db.Create(&model.Comment{PostID: post.ID, UserID: author.ID, Content: "c", Status: model.CommentStatusPublished})

	if err := svc.Delete(idString(post.ID), author.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// cover + 2 content images removed
	if len(remover.deleted) != 3 {
		t.Errorf("expected 3 file deletions, got %v", remover.deleted)
	}

	var count int64
	svc.db.Model(&model.Post{}).Count(&count)
	if count != 0 {
		t.Errorf("post not deleted")
	}
	svc.db.Model(&model.Like{}).Count(&count)
	if count != 0 {
		t.Errorf("likes not deleted")
	}
	svc.db.Model(&model.Comment{}).Count(&count)
	if count != 0 {
		t.Errorf("comments not deleted")
	}

	// other user cannot delete
	post2, err := svc.Create(author.Role, author.ID, CreateRequest{Title: "B", Content: "b", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	other := seedUser(t, svc.db, "other", model.RoleGuest)
	if err := svc.Delete(idString(post2.ID), other.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestModeration_ApproveRejectResubmit(t *testing.T) {
	svc, notifier, _ := newTestService(t)
	contributor := seedUser(t, svc.db, "contrib", model.RoleContributor)

	post, err := svc.Create(contributor.Role, contributor.ID, CreateRequest{Title: "t", Content: "c", Category: 1})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// approve
	approved, err := svc.Approve(idString(post.ID))
	if err != nil {
		t.Fatalf("Approve error: %v", err)
	}
	if approved.Status != model.PostStatusPublished {
		t.Errorf("expected published, got %s", approved.Status)
	}
	if len(notifier.calls) == 0 || notifier.calls[0] != "review" {
		t.Errorf("expected review notification, got %v", notifier.calls)
	}

	// reject
	rejected, err := svc.Reject(idString(post.ID), "too short")
	if err != nil {
		t.Fatalf("Reject error: %v", err)
	}
	if rejected.Status != model.PostStatusRejected || rejected.RejectionReason != "too short" {
		t.Errorf("unexpected rejected post: %+v", rejected)
	}

	// resubmit
	resubmitted, err := svc.Resubmit(idString(post.ID))
	if err != nil {
		t.Fatalf("Resubmit error: %v", err)
	}
	if resubmitted.Status != model.PostStatusPending || resubmitted.RejectionReason != "" {
		t.Errorf("unexpected resubmitted post: %+v", resubmitted)
	}

	// resubmit non-rejected → bad request
	if _, err := svc.Resubmit(idString(post.ID)); !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestToggleLike(t *testing.T) {
	svc, notifier, _ := newTestService(t)
	author := seedUser(t, svc.db, "author", model.RoleAuthor)
	liker := seedUser(t, svc.db, "liker", model.RoleGuest)

	post, err := svc.Create(author.Role, author.ID, CreateRequest{Title: "t", Content: "c", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	isLiked, count, err := svc.ToggleLike(post.ID, liker.ID)
	if err != nil {
		t.Fatalf("ToggleLike error: %v", err)
	}
	if !isLiked || count != 1 {
		t.Errorf("expected liked with count 1, got isLiked=%v count=%d", isLiked, count)
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != "like" {
		t.Errorf("expected like notification, got %v", notifier.calls)
	}

	// unlike
	isLiked, count, err = svc.ToggleLike(post.ID, liker.ID)
	if err != nil {
		t.Fatalf("ToggleLike error: %v", err)
	}
	if isLiked || count != 0 {
		t.Errorf("expected unliked with count 0, got isLiked=%v count=%d", isLiked, count)
	}
}

func TestList_RoleVisibility(t *testing.T) {
	svc, _, _ := newTestService(t)
	contributor := seedUser(t, svc.db, "contrib", model.RoleContributor)
	svc.db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	if _, err := svc.Create(contributor.Role, contributor.ID, CreateRequest{Title: "pub", Content: "c", Category: 1, Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := svc.Create(contributor.Role, contributor.ID, CreateRequest{Title: "pend", Content: "c", Category: 1, Status: model.PostStatusPending}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// anonymous: published only
	posts, total, err := svc.List("", 0, 1, 10, "", "", "")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "pub" {
		t.Errorf("expected 1 published post for anonymous, got total=%d posts=%+v", total, posts)
	}

	// contributor: own non-rejected + others' published
	other := seedUser(t, svc.db, "other", model.RoleAuthor)
	if _, err := svc.Create(other.Role, other.ID, CreateRequest{Title: "otherpub", Content: "c", Category: 1, Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	_, total, err = svc.List(contributor.Role, contributor.ID, 1, 10, "", "", "")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3 visible posts, got %d", total)
	}
}

func TestList_GuestViewDenied(t *testing.T) {
	svc, _, _ := newTestService(t)
	// AllowGuestViewPosts used to carry gorm:"default:true", which made GORM
	// omit the zero value on Create and silently store true — now fixed, so
	// false persists directly.
	if err := svc.db.Create(&model.GeneralSettings{AllowGuestViewPosts: false}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	posts, total, err := svc.List("", 0, 1, 10, "", "", "")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 0 || len(posts) != 0 {
		t.Errorf("expected empty result when guest view denied, got total=%d", total)
	}

	// with an existing post, anonymous access is denied
	author := seedUser(t, svc.db, "author", model.RoleAuthor)
	post, err := svc.Create(author.Role, author.ID, CreateRequest{Title: "t", Content: "c", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := svc.Get(idString(post.ID), "", 0); !errors.Is(err, ErrGuestViewDenied) {
		t.Errorf("expected ErrGuestViewDenied, got %v", err)
	}
}

func idString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
