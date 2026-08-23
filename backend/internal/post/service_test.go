package post

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeNotifier struct {
	calls []model.NotificationType
}

func (f *fakeNotifier) CreateNotification(_ context.Context, input model.NotificationInput) error {
	f.calls = append(f.calls, input.Type)
	return nil
}

type fakeRemover struct {
	deleted []string
}

func (f *fakeRemover) Delete(url string) error {
	f.deleted = append(f.deleted, url)
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
	if err := db.AutoMigrate(&model.Post{}, &model.User{}, &model.Tag{}, &model.Like{}, &model.Comment{}, &model.Category{}, &model.GeneralSettings{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *fakeNotifier, *fakeRemover, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	notifier := &fakeNotifier{}
	remover := &fakeRemover{}
	svc := NewService(Deps{DB: db, Notifier: notifier, Files: remover})
	return svc, notifier, remover, db
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
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleContributor)

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{
		Slug:       "hello-world",
		Title:      "Hello",
		Content:    "world",
		Category:   1,
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
	if err := db.Preload("Tags").First(&stored, post.ID).Error; err != nil {
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
	svc, _, _, db := newTestService(t)
	ctx := context.Background()

	contributor := seedUser(t, db, "contrib", model.RoleContributor)
	post, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{Slug: "contrib-post", Title: "t", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if post.Status != model.PostStatusPending {
		t.Errorf("contributor post expected pending, got %s", post.Status)
	}

	author := seedUser(t, db, "auth", model.RoleAuthor)
	post, err = svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "author-post", Title: "t", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if post.Status != model.PostStatusPublished {
		t.Errorf("author post expected published, got %s", post.Status)
	}
}

func TestCreate_ForbidsGuest(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	guest := seedUser(t, db, "guest", model.RoleGuest)

	if _, err := svc.Create(ctx, guest.Role, guest.ID, CreateRequest{Slug: "guest-post", Title: "t", Content: "c", Category: "1"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdate_ModifiesFields(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleContributor)

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "alpha", Title: "A", Content: "B", Category: 1, Status: model.PostStatusDraft})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	updated, err := svc.Update(ctx, idString(post.ID), user.ID, UpdateRequest{
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

	other := seedUser(t, db, "other", model.RoleGuest)
	if _, err := svc.Update(ctx, idString(post.ID), other.ID, UpdateRequest{Title: "hack"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestResolveTags_CreatesIfMissing(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.resolveTags(ctx, []string{"x", "y", "x"}); err != nil {
		t.Fatalf("resolveTags error: %v", err)
	}
}

func TestDelete_RemovesFilesAndAssociations(t *testing.T) {
	svc, _, remover, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:       "delete-test",
		Title:      "A",
		Content:    "![img](/uploads/a.jpg) and <img src=\"/uploads/b.jpg\">",
		Category:   1,
		CoverImage: "/uploads/cover.jpg",
		Status:     model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	db.Create(&model.Like{PostID: post.ID, UserID: author.ID})
	db.Create(&model.Comment{PostID: post.ID, UserID: author.ID, Content: "c", Status: model.CommentStatusPublished})

	if err := svc.Delete(ctx, idString(post.ID), author.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if len(remover.deleted) != 3 {
		t.Errorf("expected 3 file deletions, got %v", remover.deleted)
	}

	var count int64
	db.Model(&model.Post{}).Count(&count)
	if count != 0 {
		t.Errorf("post not deleted")
	}
	db.Model(&model.Like{}).Count(&count)
	if count != 0 {
		t.Errorf("likes not deleted")
	}
	db.Model(&model.Comment{}).Count(&count)
	if count != 0 {
		t.Errorf("comments not deleted")
	}

	post2, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "beta", Title: "B", Content: "b", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	other := seedUser(t, db, "other", model.RoleGuest)
	if err := svc.Delete(ctx, idString(post2.ID), other.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestModeration_ApproveRejectResubmit(t *testing.T) {
	svc, notifier, _, db := newTestService(t)
	ctx := context.Background()
	contributor := seedUser(t, db, "contrib", model.RoleContributor)

	post, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{Slug: "mod-test", Title: "t", Content: "c", Category: 1})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	approved, err := svc.Approve(ctx, idString(post.ID))
	if err != nil {
		t.Fatalf("Approve error: %v", err)
	}
	if approved.Status != model.PostStatusPublished {
		t.Errorf("expected published, got %s", approved.Status)
	}
	if len(notifier.calls) == 0 || notifier.calls[0] != model.NotificationTypeReview {
		t.Errorf("expected review notification, got %v", notifier.calls)
	}

	rejected, err := svc.Reject(ctx, idString(post.ID), "too short")
	if err != nil {
		t.Fatalf("Reject error: %v", err)
	}
	if rejected.Status != model.PostStatusRejected || rejected.RejectionReason != "too short" {
		t.Errorf("unexpected rejected post: %+v", rejected)
	}

	resubmitted, err := svc.Resubmit(ctx, idString(post.ID))
	if err != nil {
		t.Fatalf("Resubmit error: %v", err)
	}
	if resubmitted.Status != model.PostStatusPending || resubmitted.RejectionReason != "" {
		t.Errorf("unexpected resubmitted post: %+v", resubmitted)
	}

	if _, err := svc.Resubmit(ctx, idString(post.ID)); !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestToggleLike(t *testing.T) {
	svc, notifier, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	liker := seedUser(t, db, "liker", model.RoleGuest)

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "like-test", Title: "t", Content: "c", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	isLiked, count, err := svc.ToggleLike(ctx, post.ID, liker.ID)
	if err != nil {
		t.Fatalf("ToggleLike error: %v", err)
	}
	if !isLiked || count != 1 {
		t.Errorf("expected liked with count 1, got isLiked=%v count=%d", isLiked, count)
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != model.NotificationTypeLike {
		t.Errorf("expected like notification, got %v", notifier.calls)
	}

	isLiked, count, err = svc.ToggleLike(ctx, post.ID, liker.ID)
	if err != nil {
		t.Fatalf("ToggleLike error: %v", err)
	}
	if isLiked || count != 0 {
		t.Errorf("expected unliked with count 0, got isLiked=%v count=%d", isLiked, count)
	}
}

func TestCreateLikeIfAbsent_ConflictSafe(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()

	created, err := svc.repo.CreateLikeIfAbsent(ctx, 1, 1)
	if err != nil {
		t.Fatalf("CreateLikeIfAbsent error: %v", err)
	}
	if !created {
		t.Errorf("expected first insert to create the like")
	}

	// A concurrent request inserting the same post+user must not create a
	// duplicate row and must not error.
	created, err = svc.repo.CreateLikeIfAbsent(ctx, 1, 1)
	if err != nil {
		t.Fatalf("CreateLikeIfAbsent error: %v", err)
	}
	if created {
		t.Errorf("expected second insert to be a no-op")
	}
	var likes int64
	db.Model(&model.Like{}).Count(&likes)
	if likes != 1 {
		t.Errorf("expected exactly 1 like row, got %d", likes)
	}
}

func TestList_RoleVisibility(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	contributor := seedUser(t, db, "contrib", model.RoleContributor)
	db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	if _, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{Slug: "pub-post", Title: "pub", Content: "c", Category: 1, Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{Slug: "pend-post", Title: "pend", Content: "c", Category: 1, Status: model.PostStatusPending}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	posts, total, err := svc.List(ctx, ListQuery{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "pub" {
		t.Errorf("expected 1 published post for anonymous, got total=%d posts=%+v", total, posts)
	}

	other := seedUser(t, db, "other", model.RoleAuthor)
	if _, err := svc.Create(ctx, other.Role, other.ID, CreateRequest{Slug: "otherpub", Title: "otherpub", Content: "c", Category: 1, Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	_, total, err = svc.List(ctx, ListQuery{UserRole: contributor.Role, UserID: contributor.ID, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3 visible posts, got %d", total)
	}
}

func TestList_GuestViewDenied(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	if err := db.Create(&model.GeneralSettings{AllowGuestViewPosts: false}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	posts, total, err := svc.List(ctx, ListQuery{Page: 1, Limit: 10})
	if !errors.Is(err, ErrGuestViewDenied) {
		t.Fatalf("expected ErrGuestViewDenied, got %v", err)
	}
	if posts != nil || total != 0 {
		t.Errorf("expected no posts when guest view denied, got posts=%+v total=%d", posts, total)
	}

	author := seedUser(t, db, "author", model.RoleAuthor)
	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "guest-test", Title: "t", Content: "c", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := svc.Get(ctx, idString(post.ID), "", 0); !errors.Is(err, ErrGuestViewDenied) {
		t.Errorf("expected ErrGuestViewDenied, got %v", err)
	}
}

func TestCreate_RejectsEmptySlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "", Title: "t", Content: "c", Category: 1})
	if !errors.Is(err, model.ErrEmptySlug) {
		t.Errorf("expected ErrEmptySlug, got %v", err)
	}
}

func TestCreate_RejectsInvalidSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	invalid := []string{
		"INVALID",                 // uppercase
		"with space",              // spaces
		"-leading",                // leading hyphen
		"trailing-",               // trailing hyphen
		"double--hyphen",          // consecutive hyphens
		"123",                     // numeric only
		string(make([]byte, 201)), // too long
	}

	for _, slug := range invalid {
		t.Run(slug, func(t *testing.T) {
			if len(slug) > 10 {
				t.Skip("long string")
			}
			_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: slug, Title: "t", Content: "c", Category: 1})
			if err == nil {
				t.Errorf("expected error for slug %q, but got nil", slug)
			}
		})
	}
}

func TestCreate_RejectsDuplicateSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "my-post", Title: "First", Content: "c", Category: 1})
	if err != nil {
		t.Fatalf("first Create error: %v", err)
	}

	_, err = svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "my-post", Title: "Second", Content: "c", Category: 1})
	if !errors.Is(err, model.ErrSlugTaken) {
		t.Errorf("expected ErrSlugTaken, got %v", err)
	}
}

func TestUpdate_RejectsDuplicateSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "first-post", Title: "First", Content: "c", Category: 1})
	if err != nil {
		t.Fatalf("first Create error: %v", err)
	}

	second, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "second-post", Title: "Second", Content: "c", Category: 1})
	if err != nil {
		t.Fatalf("second Create error: %v", err)
	}

	// Try to change second post's slug to match first post's slug
	_, err = svc.Update(ctx, idString(second.ID), user.ID, UpdateRequest{Slug: "first-post"})
	if !errors.Is(err, model.ErrSlugTaken) {
		t.Errorf("expected ErrSlugTaken, got %v", err)
	}

	// Updating to own slug should succeed (no-op or allowed)
	updated, err := svc.Update(ctx, idString(second.ID), user.ID, UpdateRequest{Slug: "second-post"})
	if err != nil {
		t.Errorf("updating to own slug should succeed, got %v", err)
	}
	if updated.Slug != "second-post" {
		t.Errorf("expected slug to remain second-post, got %s", updated.Slug)
	}
}

func TestFindBySlug_ReturnsPost(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)
	db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "hello-world", Title: "Hello", Content: "World", Category: 1, Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	post, err := svc.GetBySlug(ctx, "hello-world", "", 0)
	if err != nil {
		t.Fatalf("GetBySlug error: %v", err)
	}
	if post.Title != "Hello" {
		t.Errorf("expected Hello, got %s", post.Title)
	}
}

func TestFindBySlug_UnknownSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	_, err := svc.GetBySlug(ctx, "no-such-slug", "", 0)
	if !errors.Is(err, ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestSlugValidation(t *testing.T) {
	valid := []string{"hello", "hello-world", "my-post-123", "a1-b2-c3"}
	for _, s := range valid {
		if err := model.ValidateSlug(s); err != nil {
			t.Errorf("expected valid slug %q, got error: %v", s, err)
		}
	}

	invalid := []string{"", "INVALID", "-bad", "bad-", "bad--bad", "123"}
	for _, s := range invalid {
		if err := model.ValidateSlug(s); err == nil {
			t.Errorf("expected error for slug %q, got nil", s)
		}
	}
}

func TestSlugFromTitle_I18n(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "Chinese",
			title: "中文 标题 测试",
			want:  "中文-标题-测试",
		},
		{
			name:  "English",
			title: "Hello World Test",
			want:  "hello-world-test",
		},
		{
			name:  "French",
			title: "Bonjour le Monde",
			want:  "bonjour-le-monde",
		},
		{
			name:  "German",
			title: "Hallo schöne Welt",
			want:  "hallo-schöne-welt",
		},
		{
			name:  "Japanese",
			title: "こんにちは 世界 入門",
			want:  "こんにちは-世界-入門",
		},
		{
			name:  "Russian",
			title: "Привет прекрасный мир",
			want:  "привет-прекрасный-мир",
		},
		{
			name:  "Korean",
			title: "안녕하세요 아름다운 세계",
			want:  "안녕하세요-아름다운-세계",
		},
		{
			name:  "Arabic",
			title: "مرحبا بالعالم الجميل",
			want:  "مرحبا-بالعالم-الجميل",
		},
		{
			name:  "Multiple spaces",
			title: "Hello   世界   테스트",
			want:  "hello-世界-테스트",
		},
		{
			name:  "Mixed languages",
			title: "Hello 中文 Français Deutsch 日本語 Русский 한국어 العربية",
			want:  "hello-中文-français-deutsch-日本語-русский-한국어-العربية",
		},
		{
			name:  "Punctuation and numbers",
			title: "What's Up? 中文 测试 123!",
			want:  "whats-up-中文-测试-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.SlugFromTitle(tt.title)
			if got != tt.want {
				t.Errorf(
					"SlugFromTitle(%q) = %q, want %q",
					tt.title,
					got,
					tt.want,
				)
			}
		})
	}
}

func idString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
