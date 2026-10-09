package post

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// seedAsset migrates the asset table on demand and stores an image asset owned
// by userID, returning it so a test can reference it as a post cover.
func seedAsset(t *testing.T, db *gorm.DB, userID uint, url string) model.Asset {
	t.Helper()
	if err := db.AutoMigrate(&model.Asset{}); err != nil {
		t.Fatalf("failed to migrate asset: %v", err)
	}
	a := model.Asset{
		OriginalName: url,
		StorageKey:   url,
		URL:          url,
		MimeType:     "image/png",
		Type:         model.AssetTypeImage,
		UserID:       userID,
	}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("failed to seed asset: %v", err)
	}
	return a
}

func TestList_RoleVisibility(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	contributor := seedUser(t, db, "contrib", model.RoleContributor)
	// Publishing needs author level or above; the contributor only contributes
	// the pending post below.
	author := seedUser(t, db, "pubauthor", model.RoleAuthor)
	db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	if _, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "pub-post", Title: "pub", Content: "c", Category: "1", Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{Slug: "pend-post", Title: "pend", Content: "c", Category: "1", Status: model.PostStatusPending}); err != nil {
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
	if _, err := svc.Create(ctx, other.Role, other.ID, CreateRequest{Slug: "otherpub", Title: "otherpub", Content: "c", Category: "1", Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// The public List returns every published post regardless of the
	// caller's role (see repository.List: status = published only). The
	// contributor's own pending post is therefore not visible here; the
	// contributor's own pending post is visible in MyPosts.
	_, total, err = svc.List(ctx, ListQuery{UserRole: contributor.Role, UserID: contributor.ID, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 published posts visible to contributor, got %d", total)
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
	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "guest-test", Title: "t", Content: "c", Category: "1", Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := svc.Get(ctx, post.ID, "", 0); !errors.Is(err, ErrGuestViewDenied) {
		t.Errorf("expected ErrGuestViewDenied, got %v", err)
	}
}

func TestLists_IncludePostCounts(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "author", model.RoleAuthor)

	if _, err := svc.CreateCategory(ctx, model.RoleContributor, "tech", ""); err != nil {
		t.Fatalf("CreateCategory error: %v", err)
	}
	if _, err := svc.CreateCategory(ctx, model.RoleContributor, "misc", ""); err != nil {
		t.Fatalf("CreateCategory error: %v", err)
	}
	// Two posts share the category "tech" and the tag "golang"; a third is
	// untagged so the empty tag "lonely" must count 0.
	for _, slug := range []string{"one", "two", "three"} {
		req := CreateRequest{Slug: slug, Title: slug, Content: "c", Category: "tech", Status: model.PostStatusPublished}
		if slug == "three" {
			req.Category = "misc"
		} else {
			req.Tags = []string{"golang"}
		}
		if _, err := svc.Create(ctx, user.Role, user.ID, req); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}
	if _, err := svc.CreateTag(ctx, model.RoleContributor, "lonely"); err != nil {
		t.Fatalf("CreateTag error: %v", err)
	}

	categories, err := svc.Categories(ctx, model.RoleAdmin)
	if err != nil {
		t.Fatalf("Categories error: %v", err)
	}
	counts := map[string]int64{}
	for _, c := range categories {
		counts[c.Name] = c.PostCount
	}
	if counts["tech"] != 2 || counts["misc"] != 1 {
		t.Errorf("expected tech=2 misc=1, got %v", counts)
	}

	tags, err := svc.Tags(ctx, model.RoleAdmin)
	if err != nil {
		t.Fatalf("Tags error: %v", err)
	}
	tagCounts := map[string]int64{}
	for _, tag := range tags {
		tagCounts[tag.Name] = tag.PostCount
	}
	if tagCounts["golang"] != 2 || tagCounts["lonely"] != 0 {
		t.Errorf("expected golang=2 lonely=0, got %v", tagCounts)
	}
}

// TestGet_UnpublishedPostsArePrivate is the regression guard for the single
// post read paths: a draft, pending or rejected post must not be readable by a
// guest or by another user through a guessed slug or id, while its author and
// admins keep full access.
func TestGet_UnpublishedPostsArePrivate(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	other := seedUser(t, db, "other", model.RoleAuthor)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	if err := db.Create(&model.GeneralSettings{AllowGuestViewPosts: true}).Error; err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	unpublished := []model.PostStatus{
		model.PostStatusDraft,
		model.PostStatusPending,
		model.PostStatusRejected,
	}
	for _, status := range unpublished {
		slug := "secret-" + string(status)
		// Seed directly: `rejected` is a moderation outcome the author-facing
		// Create endpoint deliberately refuses (see validateAuthorStatus), and
		// this test exercises the read paths for every unpublished state.
		post := model.Post{
			Slug: slug, Title: "Secret", Content: "x", Category: "1",
			Status: status, AuthorID: author.ID,
		}
		if err := db.Create(&post).Error; err != nil {
			t.Fatalf("seed %s: %v", status, err)
		}
		id := post.ID

		// Guest and an unrelated logged-in user must see "not found".
		for _, tc := range []struct {
			name string
			role string
			uid  uint
		}{
			{"guest", "", 0},
			{"other user", other.Role, other.ID},
		} {
			if _, err := svc.GetBySlug(ctx, slug, tc.role, tc.uid); !errors.Is(err, ErrPostNotFound) {
				t.Errorf("%s GetBySlug(%s) error = %v, want ErrPostNotFound", tc.name, status, err)
			}
			if _, err := svc.Get(ctx, id, tc.role, tc.uid); !errors.Is(err, ErrPostNotFound) {
				t.Errorf("%s Get(%s) error = %v, want ErrPostNotFound", tc.name, status, err)
			}
		}

		// The author and an admin keep access (editing, moderation).
		if _, err := svc.GetBySlug(ctx, slug, author.Role, author.ID); err != nil {
			t.Errorf("author GetBySlug(%s) error = %v, want nil", status, err)
		}
		if _, err := svc.Get(ctx, id, admin.Role, admin.ID); err != nil {
			t.Errorf("admin Get(%s) error = %v, want nil", status, err)
		}
	}

	// Published posts stay readable by everyone.
	if _, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug: "public", Title: "Public", Content: "x", Category: "1", Status: model.PostStatusPublished,
	}); err != nil {
		t.Fatalf("create published: %v", err)
	}
	if _, err := svc.GetBySlug(ctx, "public", "", 0); err != nil {
		t.Errorf("guest GetBySlug(published) error = %v, want nil", err)
	}
}

// TestGetBySlug_IncludesCurrentVisitInViewCount guards the off-by-one that
// made the API report the pre-increment count: the returned post and the
// database row must both reflect the visit that just happened.
func TestGetBySlug_IncludesCurrentVisitInViewCount(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)
	db.Create(&model.Post{
		Slug:      "counted",
		Title:     "Counted",
		Content:   "body",
		Category:  "1",
		AuthorID:  user.ID,
		Status:    model.PostStatusPublished,
		ViewCount: 5,
	})

	post, err := svc.GetBySlug(ctx, "counted", "", 0)
	if err != nil {
		t.Fatalf("GetBySlug error: %v", err)
	}
	if post.ViewCount != 6 {
		t.Errorf("first read ViewCount = %d, want 6", post.ViewCount)
	}

	post, err = svc.GetBySlug(ctx, "counted", "", 0)
	if err != nil {
		t.Fatalf("GetBySlug error: %v", err)
	}
	if post.ViewCount != 7 {
		t.Errorf("second read ViewCount = %d, want 7", post.ViewCount)
	}

	var stored model.Post
	if err := db.Where("slug = ?", "counted").First(&stored).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if stored.ViewCount != 7 {
		t.Errorf("stored ViewCount = %d, want 7", stored.ViewCount)
	}
}

// TestGet_DoesNotIncrementViewCount locks internal by-ID reads (edit page,
// moderation, notification resolution) to leaving the view count alone:
// opening a post to work on it is not a read.
func TestGet_DoesNotIncrementViewCount(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)
	db.Create(&model.Post{
		Slug:      "internal",
		Title:     "Internal",
		Content:   "body",
		Category:  "1",
		AuthorID:  user.ID,
		Status:    model.PostStatusPublished,
		ViewCount: 5,
	})

	var id model.Post
	if err := db.Where("slug = ?", "internal").First(&id).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}

	post, err := svc.Get(ctx, id.ID, user.Role, user.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if post.ViewCount != 5 {
		t.Errorf("Get ViewCount = %d, want 5 (unchanged)", post.ViewCount)
	}

	var stored model.Post
	if err := db.Where("slug = ?", "internal").First(&stored).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if stored.ViewCount != 5 {
		t.Errorf("stored ViewCount = %d, want 5 (unchanged)", stored.ViewCount)
	}
}

func TestFindBySlug_ReturnsPost(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)
	db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "hello-world", Title: "Hello", Content: "World", Category: "1", Status: model.PostStatusPublished})
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

// TestCreate_SavesFieldsAsAuthor covers the author-level happy path: the
// requested fields persist and an author may publish directly. Contributors
// are covered by TestCreate_PublishRequiresAuthorRole.
func TestCreate_SavesFieldsAsAuthor(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)
	cover := seedAsset(t, db, user.ID, "/img.png")

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{
		Slug:         "hello-world",
		Title:        "Hello",
		Content:      "world",
		Category:     "1",
		Tags:         []string{"go", "gin"},
		Excerpt:      "ex",
		CoverImageID: &cover.ID,
		Status:       model.PostStatusPublished,
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

// TestCreate_ForbidsCoverImageOwnedByAnotherUser is the permission-boundary
// guard for the cover image reference: a post may only point at an asset its
// own author uploaded, so one author cannot attach another user's upload (and
// with it that user's storage) to a post they control.
func TestCreate_ForbidsCoverImageOwnedByAnotherUser(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	other := seedUser(t, db, "other", model.RoleAuthor)
	foreign := seedAsset(t, db, other.ID, "/uploads/other.png")

	_, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "stolen-cover",
		Title:        "t",
		Content:      "c",
		Category:     "1",
		CoverImageID: &foreign.ID,
		Status:       model.PostStatusPublished,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create with another user's asset = %v, want ErrForbidden", err)
	}

	// An id that names no asset at all is rejected the same way: the check is
	// "an asset I own exists", not "the id is well-formed".
	missing := uint(99999)
	_, err = svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "missing-cover",
		Title:        "t",
		Content:      "c",
		Category:     "1",
		CoverImageID: &missing,
		Status:       model.PostStatusPublished,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create with an unknown asset id = %v, want ErrForbidden", err)
	}

	// Neither attempt may leave a post behind.
	var count int64
	db.Model(&model.Post{}).Count(&count)
	if count != 0 {
		t.Errorf("rejected creates stored %d posts, want 0", count)
	}
}

// TestCreate_CoverImageResolvesToTheOwnedAsset pins the happy path the
// ownership check protects: the stored value is the id, and the post handed
// back carries the asset so the caller can render its URL without a second
// lookup.
func TestCreate_CoverImageResolvesToTheOwnedAsset(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	cover := seedAsset(t, db, author.ID, "/uploads/cover.png")

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "with-cover",
		Title:        "t",
		Content:      "c",
		Category:     "1",
		CoverImageID: &cover.ID,
		Status:       model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if post.CoverImageID == nil || *post.CoverImageID != cover.ID {
		t.Fatalf("CoverImageID = %v, want %d", post.CoverImageID, cover.ID)
	}
	if post.CoverImage == nil {
		t.Fatal("CoverImage is nil, want the referenced asset")
	}
	if post.CoverImage.URL != "/uploads/cover.png" {
		t.Errorf("CoverImage.URL = %q, want /uploads/cover.png", post.CoverImage.URL)
	}
}

// TestUpdate_ForbidsCoverImageOwnedByAnotherUser mirrors the create guard on
// the update path, and pins that a rejected swap leaves the existing cover in
// place rather than half-applied.
func TestUpdate_ForbidsCoverImageOwnedByAnotherUser(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	other := seedUser(t, db, "other", model.RoleAuthor)
	own := seedAsset(t, db, author.ID, "/uploads/own.png")
	foreign := seedAsset(t, db, other.ID, "/uploads/other.png")

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "swap-cover",
		Title:        "t",
		Content:      "c",
		Category:     "1",
		CoverImageID: &own.ID,
		Status:       model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if _, err := svc.Update(ctx, post.ID, author.ID, UpdateRequest{
		Title:        "renamed",
		CoverImageID: &foreign.ID,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update with another user's asset = %v, want ErrForbidden", err)
	}

	var stored model.Post
	if err := db.First(&stored, post.ID).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if stored.CoverImageID == nil || *stored.CoverImageID != own.ID {
		t.Errorf("cover_image_id = %v, want the original %d kept", stored.CoverImageID, own.ID)
	}
	if stored.Title != "t" {
		t.Errorf("title = %q, want the rejected update to change nothing", stored.Title)
	}

	// The author's own asset still swaps in, so the guard rejects the foreign
	// id rather than every id.
	if _, err := svc.Update(ctx, post.ID, author.ID, UpdateRequest{
		CoverImageID: &own.ID,
	}); err != nil {
		t.Errorf("Update with an owned asset error: %v", err)
	}
}

// TestCreate_DanglingCoverImageResolvesToNothing covers the upgrade boundary:
// the asset behind a stored cover_image_id can be deleted afterwards (or was
// never ours to begin with), and a post pointing at nothing must still read
// back cleanly. The id is kept so the reference is recoverable if the asset
// returns, mirroring the site icon reference.
func TestCreate_DanglingCoverImageResolvesToNothing(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	cover := seedAsset(t, db, author.ID, "/uploads/cover.png")

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "dangling",
		Title:        "t",
		Content:      "c",
		Category:     "1",
		CoverImageID: &cover.ID,
		Status:       model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if err := db.Delete(&cover).Error; err != nil {
		t.Fatalf("soft delete asset: %v", err)
	}

	loaded, err := svc.Get(ctx, post.ID, author.Role, author.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if loaded.CoverImageID == nil || *loaded.CoverImageID != cover.ID {
		t.Fatalf("CoverImageID = %v, want the stored %d kept", loaded.CoverImageID, cover.ID)
	}
	if loaded.CoverImage != nil {
		t.Errorf("CoverImage = %+v, want nil for a soft-deleted asset", loaded.CoverImage)
	}

	// The reference survives an unrelated edit, because cover_image_id is
	// written from CoverImageID rather than from the association that failed to
	// load.
	if _, err := svc.Update(ctx, post.ID, author.ID, UpdateRequest{Title: "renamed"}); err != nil {
		t.Fatalf("Update after asset delete error: %v", err)
	}
	var stored model.Post
	if err := db.First(&stored, post.ID).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if stored.CoverImageID == nil || *stored.CoverImageID != cover.ID {
		t.Errorf("cover_image_id = %v, want the dangling reference %d kept", stored.CoverImageID, cover.ID)
	}
}

// TestDelete_DanglingCoverImageSkipsTheMissingFile pins the cleanup path: the
// cover's file is collected from the asset, so a dangling reference must not
// queue a deletion for a file whose asset is gone.
func TestDelete_DanglingCoverImageSkipsTheMissingFile(t *testing.T) {
	svc, _, remover, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	cover := seedAsset(t, db, author.ID, "/uploads/cover.png")

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "dangling-delete",
		Title:        "t",
		Content:      "c",
		Category:     "1",
		CoverImageID: &cover.ID,
		Status:       model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if err := db.Delete(&cover).Error; err != nil {
		t.Fatalf("soft delete asset: %v", err)
	}

	if err := svc.Delete(ctx, post.ID, author.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if len(remover.deleted) != 0 {
		t.Errorf("deleted files = %v, want none for a post whose cover asset is gone", remover.deleted)
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

// TestCreate_PublishRequiresAuthorRole is the permission-boundary guard for
// the status field on create: a client-supplied "published" must not let a
// role below author level bypass the moderation queue, while author level and
// above keep publishing directly.
func TestCreate_PublishRequiresAuthorRole(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()

	denied := []struct {
		name string
		role string
	}{
		{"anonymous", ""},
		{"guest", model.RoleGuest},
		{"contributor", model.RoleContributor},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			slug := "denied-" + tc.name
			user := seedUser(t, db, slug, tc.role)

			if _, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{
				Slug: slug, Title: "t", Content: "c", Category: "1",
				Status: model.PostStatusPublished,
			}); !errors.Is(err, ErrForbidden) {
				t.Fatalf("Create(published) = %v, want ErrForbidden", err)
			}

			// A rejected request must not persist a row.
			var count int64
			db.Model(&model.Post{}).Where("slug = ?", slug).Count(&count)
			if count != 0 {
				t.Fatalf("post persisted despite rejection")
			}
		})
	}

	// Author level and above publish directly.
	for _, role := range []string{model.RoleAuthor, model.RoleAdmin, model.RoleSuperAdmin} {
		t.Run(role, func(t *testing.T) {
			slug := "allowed-" + strings.ReplaceAll(role, "_", "-")
			user := seedUser(t, db, slug, role)
			post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{
				Slug: slug, Title: "t", Content: "c", Category: "1",
				Status: model.PostStatusPublished,
			})
			if err != nil {
				t.Fatalf("Create(published) error = %v", err)
			}
			if post.Status != model.PostStatusPublished {
				t.Fatalf("status = %s, want published", post.Status)
			}
		})
	}
}

// TestCreate_RejectsInvalidStatus guards the enum: an unknown string and the
// moderation-only `rejected` state are rejected and never persisted, for every
// role.
func TestCreate_RejectsInvalidStatus(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "author", model.RoleAuthor)

	for _, status := range []model.PostStatus{"garbage", "PUBLISHED", model.PostStatusRejected} {
		slug := "bad-" + strings.ToLower(string(status))
		_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{
			Slug: slug, Title: "t", Content: "c", Category: "1", Status: status,
		})
		if !errors.Is(err, ErrInvalidStatus) {
			t.Errorf("Create(status=%q) error = %v, want ErrInvalidStatus", status, err)
		}
		var count int64
		db.Model(&model.Post{}).Where("slug = ?", slug).Count(&count)
		if count != 0 {
			t.Errorf("status %q persisted despite rejection", status)
		}
	}
}

// ---------------------------------------------------------------------------
// Slug validation and generation tests
// ---------------------------------------------------------------------------

func TestCreate_RejectsEmptySlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "", Title: "t", Content: "c", Category: "1"})
	if !errors.Is(err, model.ErrEmptySlug) {
		t.Errorf("expected ErrEmptySlug, got %v", err)
	}
}

func TestCreate_RejectsInvalidSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	invalid := []string{
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
			_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: slug, Title: "t", Content: "c", Category: "1"})
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

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "my-post", Title: "First", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("first Create error: %v", err)
	}

	_, err = svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "my-post", Title: "Second", Content: "c", Category: "1"})
	if !errors.Is(err, model.ErrSlugTaken) {
		t.Errorf("expected ErrSlugTaken, got %v", err)
	}
}

// TestCreate_NormalizesUppercaseSlug verifies the service layer normalizes
// uppercase input to lowercase before persisting.
func TestCreate_NormalizesUppercaseSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "HELLO-WORLD", Title: "t", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("expected uppercase slug to be normalized, got error: %v", err)
	}
	if post.Slug != "hello-world" {
		t.Errorf("expected slug to be normalized to lowercase, got %q", post.Slug)
	}
}

// TestCreate_SupportsInternationalSlugs verifies the full Create → GetBySlug
// lifecycle works with non-ASCII slug content.
func TestCreate_SupportsInternationalSlugs(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)
	db.Create(&model.GeneralSettings{AllowGuestViewPosts: true})

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{
		Slug:     "中文-标题-测试",
		Title:    "中文标题",
		Content:  "内容",
		Category: "1",
		Status:   model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create with Chinese slug should succeed, got: %v", err)
	}
	if post.Slug != "中文-标题-测试" {
		t.Errorf("expected slug 中文-标题-测试, got %s", post.Slug)
	}

	// Lookup by slug works.
	found, err := svc.GetBySlug(ctx, "中文-标题-测试", "", 0)
	if err != nil {
		t.Fatalf("GetBySlug with Chinese slug should succeed, got: %v", err)
	}
	if found.Title != "中文标题" {
		t.Errorf("expected title 中文标题, got %s", found.Title)
	}
}

func TestUpdate_ModifiesFields(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "alpha", Title: "A", Content: "B", Category: "1", Status: model.PostStatusDraft})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	updated, err := svc.Update(ctx, post.ID, user.ID, UpdateRequest{
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
	if _, err := svc.Update(ctx, post.ID, other.ID, UpdateRequest{Title: "hack"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// TestUpdate_PublishRequiresAuthorRole verifies the update path enforces the
// same boundary as create: a contributor cannot move their own post into the
// published state, while an admin may publish any post and an author may
// publish their own.
func TestUpdate_PublishRequiresAuthorRole(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	contributor := seedUser(t, db, "contrib", model.RoleContributor)

	post, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{
		Slug: "pending-post", Title: "t", Content: "c", Category: "1",
		Status: model.PostStatusPending,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if _, err := svc.Update(ctx, post.ID, contributor.ID, UpdateRequest{Status: model.PostStatusPublished}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("contributor self-publish error = %v, want ErrForbidden", err)
	}
	if got := reloadPostStatus(t, db, post.ID); got != model.PostStatusPending {
		t.Fatalf("status = %s after rejected publish, want pending", got)
	}

	// An admin may publish it; afterwards the contributor editing content
	// without a status change stays allowed.
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	if _, err := svc.Update(ctx, post.ID, admin.ID, UpdateRequest{Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("admin publish error = %v", err)
	}
	if got := reloadPostStatus(t, db, post.ID); got != model.PostStatusPublished {
		t.Fatalf("status = %s, want published", got)
	}
	if _, err := svc.Update(ctx, post.ID, contributor.ID, UpdateRequest{Title: "edited"}); err != nil {
		t.Fatalf("contributor edit of published post error = %v", err)
	}

	// An author publishes their own draft directly.
	author := seedUser(t, db, "author", model.RoleAuthor)
	own, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug: "author-draft", Title: "t", Content: "c", Category: "1", Status: model.PostStatusDraft,
	})
	if err != nil {
		t.Fatalf("Create author draft error: %v", err)
	}
	if _, err := svc.Update(ctx, own.ID, author.ID, UpdateRequest{Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("author publish error = %v", err)
	}
	if got := reloadPostStatus(t, db, own.ID); got != model.PostStatusPublished {
		t.Fatalf("author post status = %s, want published", got)
	}
}

// TestUpdate_RejectedPostStatusRules covers the guards around a rejected post:
// its author may requeue it (pending) but not republish it, and the
// moderation-only `rejected` value cannot be set through Update by anyone.
func TestUpdate_RejectedPostStatusRules(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	admin := seedUser(t, db, "admin", model.RoleAdmin)

	// Rejection is a moderation outcome, so seed the rejected rows directly.
	rejected := model.Post{
		Slug: "rejected-post", Title: "t", Content: "c", Category: "1",
		Status: model.PostStatusRejected, AuthorID: author.ID,
	}
	if err := db.Create(&rejected).Error; err != nil {
		t.Fatalf("seed rejected post: %v", err)
	}

	// The author cannot override the rejection by publishing.
	if _, err := svc.Update(ctx, rejected.ID, author.ID, UpdateRequest{Status: model.PostStatusPublished}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("author republish error = %v, want ErrForbidden", err)
	}
	if got := reloadPostStatus(t, db, rejected.ID); got != model.PostStatusRejected {
		t.Fatalf("status = %s, want rejected", got)
	}

	// `rejected` is never settable through Update.
	if _, err := svc.Update(ctx, rejected.ID, author.ID, UpdateRequest{Status: model.PostStatusRejected}); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("Update(status=rejected) error = %v, want ErrInvalidStatus", err)
	}

	// Requeueing for review stays allowed for the author.
	if _, err := svc.Update(ctx, rejected.ID, author.ID, UpdateRequest{Status: model.PostStatusPending}); err != nil {
		t.Fatalf("author requeue error = %v", err)
	}
	if got := reloadPostStatus(t, db, rejected.ID); got != model.PostStatusPending {
		t.Fatalf("status = %s, want pending", got)
	}

	// An admin keeps the override path for a post they own.
	adminPost := model.Post{
		Slug: "admin-rejected", Title: "t", Content: "c", Category: "1",
		Status: model.PostStatusRejected, AuthorID: admin.ID,
	}
	if err := db.Create(&adminPost).Error; err != nil {
		t.Fatalf("seed admin post: %v", err)
	}
	if _, err := svc.Update(ctx, adminPost.ID, admin.ID, UpdateRequest{Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("admin republish error = %v", err)
	}
	if got := reloadPostStatus(t, db, adminPost.ID); got != model.PostStatusPublished {
		t.Fatalf("admin status = %s, want published", got)
	}
}

func TestResolveTags_CreatesIfMissing(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.resolveTags(ctx, []string{"x", "y", "x"}); err != nil {
		t.Fatalf("resolveTags error: %v", err)
	}
}

func TestUpdate_RejectsDuplicateSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	_, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "first-post", Title: "First", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("first Create error: %v", err)
	}

	second, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "second-post", Title: "Second", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("second Create error: %v", err)
	}

	// Try to change second post's slug to match first post's slug
	_, err = svc.Update(ctx, second.ID, user.ID, UpdateRequest{Slug: "first-post"})
	if !errors.Is(err, model.ErrSlugTaken) {
		t.Errorf("expected ErrSlugTaken, got %v", err)
	}

	// Updating to own slug should succeed (no-op or allowed)
	updated, err := svc.Update(ctx, second.ID, user.ID, UpdateRequest{Slug: "second-post"})
	if err != nil {
		t.Errorf("updating to own slug should succeed, got %v", err)
	}
	if updated.Slug != "second-post" {
		t.Errorf("expected slug to remain second-post, got %s", updated.Slug)
	}
}

// TestUpdate_NormalizesUppercaseSlug verifies the service layer normalizes
// uppercase slug input in Update, and a duplicate that only differs in case
// is treated as a no-op (keeping the same slug).
func TestUpdate_NormalizesUppercaseSlug(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "tester", model.RoleAuthor)

	post, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "my-slug", Title: "t", Content: "c", Category: "1"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Case-only change should be a no-op.
	updated, err := svc.Update(ctx, post.ID, user.ID, UpdateRequest{Slug: "MY-SLUG"})
	if err != nil {
		t.Fatalf("Update with case-only change should succeed, got: %v", err)
	}
	if updated.Slug != "my-slug" {
		t.Errorf("expected slug to stay my-slug, got %s", updated.Slug)
	}
}

func TestDelete_RemovesFilesAndAssociations(t *testing.T) {
	svc, _, remover, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	cover := seedAsset(t, db, author.ID, "/uploads/cover.jpg")

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
		Slug:         "delete-test",
		Title:        "A",
		Content:      "![img](/uploads/a.jpg) and <img src=\"/uploads/b.jpg\">",
		Category:     "1",
		CoverImageID: &cover.ID,
		Status:       model.PostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	db.Create(&model.Like{PostID: post.ID, UserID: author.ID})
	db.Create(&model.Comment{PostID: post.ID, UserID: author.ID, Content: "c", Status: model.CommentStatusPublished})

	if err := svc.Delete(ctx, post.ID, author.ID); err != nil {
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

	post2, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "beta", Title: "B", Content: "b", Category: "1", Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	other := seedUser(t, db, "other", model.RoleGuest)
	if err := svc.Delete(ctx, post2.ID, other.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// TestUserPosts_HidesOtherUsersUnpublished covers the profile listing path: a
// logged-in author browsing another user must not see that user's drafts or
// pending posts.
func TestUserPosts_HidesOtherUsersUnpublished(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	viewer := seedUser(t, db, "viewer", model.RoleAuthor)

	for _, spec := range []struct {
		slug   string
		status model.PostStatus
	}{
		{"pub", model.PostStatusPublished},
		{"draft", model.PostStatusDraft},
		{"pend", model.PostStatusPending},
	} {
		if _, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
			Slug: spec.slug, Title: spec.slug, Content: "x", Category: "1", Status: spec.status,
		}); err != nil {
			t.Fatalf("create %s: %v", spec.slug, err)
		}
	}

	posts, _, err := svc.UserPosts(ctx, UserPostsQuery{
		UserID:          author.ID,
		CurrentUserRole: viewer.Role,
		CurrentUserID:   viewer.ID,
		Page:            1,
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("UserPosts error: %v", err)
	}
	if len(posts) != 1 || posts[0].Slug != "pub" {
		t.Errorf("other user must see only published posts, got %+v", posts)
	}

	// The author still sees their own draft and pending posts.
	own, _, err := svc.UserPosts(ctx, UserPostsQuery{
		UserID:          author.ID,
		CurrentUserRole: author.Role,
		CurrentUserID:   author.ID,
		Page:            1,
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("own UserPosts error: %v", err)
	}
	if len(own) != 3 {
		t.Errorf("author must see all own non-rejected posts, got %d", len(own))
	}
}

// A list request for an author id that is not in use is a missing resource,
// not an empty page; an existing author without posts still gets an empty
// page and no error.
func TestUserPosts_UnknownAuthor(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)

	_, _, err := svc.UserPosts(ctx, UserPostsQuery{
		UserID:          author.ID,
		CurrentUserRole: model.RoleGuest,
		Page:            1,
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("existing author without posts must not fail: %v", err)
	}

	_, _, err = svc.UserPosts(ctx, UserPostsQuery{
		UserID:          99999,
		CurrentUserRole: model.RoleGuest,
		Page:            1,
		Limit:           10,
	})
	if !errors.Is(err, ErrAuthorNotFound) {
		t.Fatalf("expected ErrAuthorNotFound, got %v", err)
	}
}

// The author block is filtered for the viewer on every public list path —
// including the two that used to hand out the raw row: the latest and popular
// post feeds.
func TestPublicPostFeedsRedactAuthor(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T, hideEmail bool) (*Service, model.User) {
		t.Helper()
		svc, _, _, db := newTestService(t)
		author := seedUser(t, db, "author", model.RoleAuthor)
		author.Email = "author@example.com"
		author.EmailVerified = true
		author.HideEmail = hideEmail
		author.ProfileVisibility = model.ProfileVisibilityPublic
		author.LastLoginAt = time.Now()
		if err := db.Save(&author).Error; err != nil {
			t.Fatalf("failed to save author: %v", err)
		}
		if _, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{
			Slug:     "published-post",
			Title:    "Published",
			Content:  "body",
			Category: "1",
			Status:   model.PostStatusPublished,
		}); err != nil {
			t.Fatalf("Create error: %v", err)
		}
		return svc, author
	}

	check := func(t *testing.T, label string, posts []model.Post, wantEmail string) {
		t.Helper()
		if len(posts) != 1 {
			t.Fatalf("%s: expected 1 post, got %d", label, len(posts))
		}
		author := posts[0].Author
		if author.Username == "" {
			t.Fatalf("%s: test setup lost the author block", label)
		}
		if author.Email != wantEmail {
			t.Errorf("%s: expected email %q, got %q", label, wantEmail, author.Email)
		}
		if author.Role != "" || author.EmailVerified || !author.LastLoginAt.IsZero() || author.ProfileVisibility != "" {
			t.Errorf("%s: account state leaked in the author block: %+v", label, author)
		}
	}

	t.Run("anonymous viewer", func(t *testing.T) {
		svc, _ := seed(t, false)

		latest, err := svc.Latest(ctx, 0, "", 5)
		if err != nil {
			t.Fatalf("Latest error: %v", err)
		}
		check(t, "Latest", latest, "")

		popular, err := svc.Popular(ctx, 0, "", 5)
		if err != nil {
			t.Fatalf("Popular error: %v", err)
		}
		check(t, "Popular", popular, "")
	})

	t.Run("signed-in reader", func(t *testing.T) {
		svc, _ := seed(t, false)

		latest, err := svc.Latest(ctx, 999, model.RoleGuest, 5)
		if err != nil {
			t.Fatalf("Latest error: %v", err)
		}
		check(t, "Latest", latest, "author@example.com")
	})

	t.Run("signed-in reader, address hidden", func(t *testing.T) {
		svc, _ := seed(t, true)

		popular, err := svc.Popular(ctx, 999, model.RoleGuest, 5)
		if err != nil {
			t.Fatalf("Popular error: %v", err)
		}
		check(t, "Popular", popular, "")
	})

	t.Run("owner keeps their own account", func(t *testing.T) {
		svc, author := seed(t, false)

		latest, err := svc.Latest(ctx, author.ID, author.Role, 5)
		if err != nil {
			t.Fatalf("Latest error: %v", err)
		}
		if latest[0].Author.Role != model.RoleAuthor {
			t.Errorf("the owner must see their own role, got %+v", latest[0].Author)
		}
	})
}
