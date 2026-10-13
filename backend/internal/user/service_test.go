package user

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fakeNotifier records notification calls instead of touching the DB.
type fakeNotifier struct {
	calls         []model.NotificationType
	notifications []string
}

func (f *fakeNotifier) CreateNotification(_ context.Context, input model.NotificationInput) error {
	f.calls = append(f.calls, input.Type)
	f.notifications = append(f.notifications, input.Content)
	return nil
}

// fakeFiles records deleted URLs.
type fakeFiles struct {
	deleted []string
}

func (f *fakeFiles) Delete(_ context.Context, url string) error {
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
	// Asset shares the media_files table with MediaFile and adds deleted_at,
	// which the cascade queries when it collects the user's uploads.
	if err := db.AutoMigrate(&model.User{}, &model.Post{}, &model.Tag{}, &model.Comment{}, &model.Like{}, &model.MediaFile{}, &model.Asset{}, &model.CreatorApplication{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *fakeNotifier, *fakeFiles, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	notifier := &fakeNotifier{}
	files := &fakeFiles{}
	svc := NewService(Deps{DB: db, Notifier: notifier, Files: files})
	return svc, notifier, files, db
}

func seedUser(t *testing.T, db *gorm.DB, username, role string) model.User {
	t.Helper()
	u := model.User{Username: username, Email: username + "@example.com", Role: role}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

// seedUserAsset migrates the asset model on demand and stores an asset owned by
// userID. Asset and MediaFile share the media_files table, so this is the same
// row DeleteUserCascade already collects — with the difference that Asset
// carries deleted_at, which is what the asset service sets when it retires one.
func seedUserAsset(t *testing.T, db *gorm.DB, userID uint, key string) model.Asset {
	t.Helper()
	asset := model.Asset{
		OriginalName: key,
		StorageKey:   key,
		URL:          "/uploads/" + key,
		MimeType:     model.MimeTypeFromName(key),
		Type:         model.AssetTypeFromMIME(model.MimeTypeFromName(key)),
		UserID:       userID,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("failed to seed asset: %v", err)
	}
	return asset
}

// assetRetired reports whether the asset row is still present but marked
// deleted. Unscoped is what separates a soft delete from a hard one: after a
// hard delete the row is unreadable, and reporting that as "retired" would let
// a destructive implementation pass this test.
func assetRetired(t *testing.T, db *gorm.DB, id uint) bool {
	t.Helper()
	var asset model.Asset
	if err := db.Unscoped().First(&asset, id).Error; err != nil {
		t.Fatalf("asset %d was removed outright instead of retired: %v", id, err)
	}
	return asset.DeletedAt.Valid
}

func TestUpdateRole_NotificationReportsOldRole(t *testing.T) {
	ctx := context.Background()
	svc, notifier, _, db := newTestService(t)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	target := seedUser(t, db, "target", model.RoleGuest)

	if _, err := svc.UpdateRole(ctx, admin, target.ID, model.RoleAuthor); err != nil {
		t.Fatalf("UpdateRole error: %v", err)
	}
	if len(notifier.notifications) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifier.notifications))
	}
	if !strings.Contains(notifier.notifications[0], `from "guest" to "author"`) {
		t.Errorf("expected old role in the notification content, got %q", notifier.notifications[0])
	}

	// Role upgrade path: contributor -> author must report both roles too.
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target2 := seedUser(t, db, "target2", model.RoleContributor)
	notifier.notifications = nil
	if _, err := svc.UpdateRole(ctx, super, target2.ID, model.RoleAuthor); err != nil {
		t.Fatalf("UpdateRole error: %v", err)
	}
	if len(notifier.notifications) != 1 || !strings.Contains(notifier.notifications[0], `from "contributor" to "author"`) {
		t.Errorf("expected contributor->author notification, got %q", notifier.notifications)
	}
}

func TestUpdateRole_Permissions(t *testing.T) {
	ctx := context.Background()
	svc, notifier, _, db := newTestService(t)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target := seedUser(t, db, "target", model.RoleGuest)

	updated, err := svc.UpdateRole(ctx, admin, target.ID, model.RoleAuthor)
	if err != nil {
		t.Fatalf("UpdateRole error: %v", err)
	}
	if updated.Role != model.RoleAuthor {
		t.Errorf("expected author, got %s", updated.Role)
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != model.NotificationTypeRole {
		t.Errorf("expected one role notification, got %v", notifier.calls)
	}

	if _, err := svc.UpdateRole(ctx, admin, target.ID, model.RoleAdmin); !errors.Is(err, ErrAdminRoleRestricted) {
		t.Errorf("expected ErrAdminRoleRestricted, got %v", err)
	}

	if _, err := svc.UpdateRole(ctx, admin, admin.ID, model.RoleAuthor); !errors.Is(err, ErrCannotModifySelf) {
		t.Errorf("expected ErrCannotModifySelf, got %v", err)
	}

	guest := seedUser(t, db, "guest", model.RoleGuest)
	if _, err := svc.UpdateRole(ctx, guest, target.ID, model.RoleAuthor); !errors.Is(err, ErrNoPermission) {
		t.Errorf("expected ErrNoPermission, got %v", err)
	}

	if _, err := svc.UpdateRole(ctx, admin, super.ID, model.RoleAuthor); !errors.Is(err, ErrModifySuperAdmin) {
		t.Errorf("expected ErrModifySuperAdmin, got %v", err)
	}

	if _, err := svc.UpdateRole(ctx, super, target.ID, model.RoleSuperAdmin); !errors.Is(err, ErrSuperAdminRestricted) {
		t.Errorf("expected ErrSuperAdminRestricted, got %v", err)
	}

	if _, err := svc.UpdateRole(ctx, super, target.ID, "not-a-role"); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}

	if _, err := svc.UpdateRole(ctx, super, 99999, model.RoleAuthor); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestDeleteUser_CascadeAndPermissions(t *testing.T) {
	ctx := context.Background()
	svc, _, files, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	target := seedUser(t, db, "target", model.RoleAuthor)
	otherAdmin := seedUser(t, db, "other-admin", model.RoleAdmin)

	if err := db.AutoMigrate(&model.Asset{}); err != nil {
		t.Fatalf("failed to migrate asset: %v", err)
	}
	cover := model.Asset{URL: "/uploads/cover.jpg", StorageKey: "cover.jpg", MimeType: "image/jpeg", Type: model.AssetTypeImage, UserID: target.ID}
	if err := db.Create(&cover).Error; err != nil {
		t.Fatalf("failed to seed asset: %v", err)
	}
	post := model.Post{Title: "p", Content: "c", Category: "1", AuthorID: target.ID, Status: model.PostStatusPublished, CoverImageID: &cover.ID, CoverImage: &cover}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}
	if err := db.Create(&model.Comment{PostID: post.ID, UserID: target.ID, Content: "c"}).Error; err != nil {
		t.Fatalf("failed to seed comment: %v", err)
	}
	if err := db.Create(&model.Like{PostID: post.ID, UserID: target.ID}).Error; err != nil {
		t.Fatalf("failed to seed like: %v", err)
	}
	if err := db.Create(&model.MediaFile{UserID: target.ID, URL: "/uploads/media.jpg"}).Error; err != nil {
		t.Fatalf("failed to seed media file: %v", err)
	}

	if err := svc.DeleteUser(ctx, admin, otherAdmin.ID); !errors.Is(err, ErrAdminDeleteRestricted) {
		t.Errorf("expected ErrAdminDeleteRestricted, got %v", err)
	}

	guest := seedUser(t, db, "guest", model.RoleGuest)
	if err := svc.DeleteUser(ctx, guest, target.ID); !errors.Is(err, ErrNoPermissionToDelete) {
		t.Errorf("expected ErrNoPermissionToDelete, got %v", err)
	}

	if err := svc.DeleteUser(ctx, target, target.ID); !errors.Is(err, ErrCannotDeleteSelf) {
		t.Errorf("expected ErrCannotDeleteSelf, got %v", err)
	}

	if err := svc.DeleteUser(ctx, super, 99999); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}

	if err := svc.DeleteUser(ctx, super, target.ID); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}
	var count int64
	db.Model(&model.User{}).Where("id = ?", target.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected user deleted")
	}
	db.Model(&model.Post{}).Where("author_id = ?", target.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected posts deleted")
	}
	db.Model(&model.Comment{}).Where("user_id = ?", target.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected comments deleted")
	}
	db.Model(&model.Like{}).Where("user_id = ?", target.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected likes deleted")
	}
	// The uploads go through the asset service, which retires the rows rather
	// than destroying them. model.MediaFile carries no deleted_at, so it counts
	// retired rows too: the right assertion is that they are marked deleted.
	var asset model.Asset
	if err := db.Unscoped().First(&asset, cover.ID).Error; err != nil {
		t.Errorf("cover asset was removed outright instead of retired: %v", err)
	} else if !asset.DeletedAt.Valid {
		t.Errorf("cover asset %d is not soft-deleted", cover.ID)
	}
	var media model.MediaFile
	if err := db.Unscoped().First(&media, "user_id = ?", target.ID).Error; err != nil {
		t.Errorf("user media file was removed outright instead of retired: %v", err)
	}
	if len(files.deleted) != 0 {
		t.Errorf("files bypassed the asset service: %v", files.deleted)
	}
}

func TestDeleteUser_ClearsPostTagAssociations(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target := seedUser(t, db, "target", model.RoleAuthor)

	post := model.Post{Title: "p", Content: "c", Category: "1", AuthorID: target.ID, Status: model.PostStatusPublished}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}
	tags := []model.Tag{{Name: "go"}, {Name: "gin"}}
	if err := db.Create(&tags).Error; err != nil {
		t.Fatalf("failed to seed tags: %v", err)
	}
	if err := db.Model(&post).Association("Tags").Append(tags); err != nil {
		t.Fatalf("failed to attach tags: %v", err)
	}

	if err := svc.DeleteUser(ctx, super, target.ID); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}

	// The many-to-many join rows must not survive as orphans.
	var joinCount int64
	if err := db.Table("post_tags").Count(&joinCount).Error; err != nil {
		t.Fatalf("count post_tags: %v", err)
	}
	if joinCount != 0 {
		t.Errorf("expected post_tags join rows deleted, got %d", joinCount)
	}

	// Tags themselves are global and must survive the deletion.
	var tagCount int64
	if err := db.Model(&model.Tag{}).Count(&tagCount).Error; err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tagCount != 2 {
		t.Errorf("expected tags preserved, got %d", tagCount)
	}
}

// TestDeleteUser_RetiresAssetsOfDeletedUser pins the user half of the TODO: a
// deleted user's uploads go through the asset service, which retires the row
// rather than destroying it. Today the cascade issues a hard DELETE against
// media_files, so the row disappears and this test fails at the Unscoped read.
func TestDeleteUser_RetiresAssetsOfDeletedUser(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target := seedUser(t, db, "target", model.RoleAuthor)
	avatar := seedUserAsset(t, db, target.ID, "avatar.png")
	inline := seedUserAsset(t, db, target.ID, "inline.png")

	if err := svc.DeleteUser(ctx, super, target.ID); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}

	for _, asset := range []model.Asset{avatar, inline} {
		if !assetRetired(t, db, asset.ID) {
			t.Errorf("asset %d (%s) is not retired", asset.ID, asset.StorageKey)
		}
	}
}

// TestDeleteUser_LeavesOtherUsersAssetsAlone is the permission boundary: a
// cascade must not sweep assets it does not own, or deleting one account would
// silently destroy every other account's uploads.
func TestDeleteUser_LeavesOtherUsersAssetsAlone(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target := seedUser(t, db, "target", model.RoleAuthor)
	innocent := seedUser(t, db, "innocent", model.RoleAuthor)
	mine := seedUserAsset(t, db, target.ID, "mine.png")
	theirs := seedUserAsset(t, db, innocent.ID, "theirs.png")

	if err := svc.DeleteUser(ctx, super, target.ID); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}

	if !assetRetired(t, db, mine.ID) {
		t.Errorf("deleted user's own asset %d is not retired", mine.ID)
	}
	if assetRetired(t, db, theirs.ID) {
		t.Errorf("asset %d belongs to another user and must survive", theirs.ID)
	}
}

// TestDeleteUser_RetiresCoverAssetOfDeletedPosts pins the cross-domain case: an
// upload that is a deleted user's post cover is covered by the ownership sweep
// above, and deleting the account must retire it rather than leaving the post
// row's reference dangling on a live asset.
func TestDeleteUser_RetiresCoverAssetOfDeletedPosts(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target := seedUser(t, db, "target", model.RoleAuthor)
	cover := seedUserAsset(t, db, target.ID, "cover.png")

	post := model.Post{
		Title:        "p",
		Content:      "c",
		Category:     "1",
		AuthorID:     target.ID,
		Status:       model.PostStatusPublished,
		CoverImageID: &cover.ID,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	if err := svc.DeleteUser(ctx, super, target.ID); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}

	if !assetRetired(t, db, cover.ID) {
		t.Errorf("cover asset %d is not retired", cover.ID)
	}

	var count int64
	if err := db.Model(&model.Post{}).Where("author_id = ?", target.ID).Count(&count).Error; err != nil {
		t.Fatalf("count posts: %v", err)
	}
	if count != 0 {
		t.Errorf("posts of the deleted user remain: %d", count)
	}
}

// TestDeleteUser_RetiresAlreadyRetiredAssets pins that the cascade tolerates a
// row it cannot see. An asset the user retired themselves is already gone from
// every scoped query before the account goes, so a lookup miss is an ordinary
// outcome and not an error that aborts the account deletion. The assertion that
// carries the weight is therefore the last one: the user row really went away.
func TestDeleteUser_RetiresAlreadyRetiredAssets(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	target := seedUser(t, db, "target", model.RoleAuthor)
	asset := seedUserAsset(t, db, target.ID, "already.png")

	if err := db.Delete(&asset).Error; err != nil {
		t.Fatalf("retire asset up front: %v", err)
	}

	if err := svc.DeleteUser(ctx, super, target.ID); err != nil {
		t.Fatalf("DeleteUser error: %v", err)
	}

	if !assetRetired(t, db, asset.ID) {
		t.Errorf("already-retired asset %d was resurrected", asset.ID)
	}

	var count int64
	if err := db.Model(&model.User{}).Where("id = ?", target.ID).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Errorf("user was not deleted: %d rows remain", count)
	}
}

func TestApplyForCreator(t *testing.T) {
	ctx := context.Background()
	svc, notifier, _, db := newTestService(t)
	guest := seedUser(t, db, "guest", model.RoleGuest)
	admin := seedUser(t, db, "admin", model.RoleAdmin)

	appID, err := svc.ApplyForCreator(ctx, guest, "I want to write")
	if err != nil {
		t.Fatalf("ApplyForCreator error: %v", err)
	}
	if appID == 0 {
		t.Errorf("expected non-zero application id")
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != model.NotificationTypeRole {
		t.Errorf("expected one admin notification, got %v", notifier.calls)
	}

	if _, err := svc.ApplyForCreator(ctx, guest, "again"); !errors.Is(err, ErrAlreadyPending) {
		t.Errorf("expected ErrAlreadyPending, got %v", err)
	}

	author := seedUser(t, db, "author", model.RoleAuthor)
	if _, err := svc.ApplyForCreator(ctx, author, "nope"); !errors.Is(err, ErrRoleNotEligible) {
		t.Errorf("expected ErrRoleNotEligible, got %v", err)
	}
	_ = admin
}

func TestListCreatorApplications_PermissionAndFilter(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	guest := seedUser(t, db, "guest", model.RoleGuest)
	other := seedUser(t, db, "other", model.RoleGuest)

	if _, err := svc.ApplyForCreator(ctx, guest, "apply 1"); err != nil {
		t.Fatalf("ApplyForCreator error: %v", err)
	}
	if _, err := svc.ApplyForCreator(ctx, other, "apply 2"); err != nil {
		t.Fatalf("ApplyForCreator error: %v", err)
	}

	if _, _, err := svc.ListCreatorApplications(ctx, ListCreatorApplicationsQuery{ActorRole: guest.Role, Status: model.CreatorApplicationStatusPending, Page: 1, Limit: 10}); !errors.Is(err, ErrNoPermissionAccessApps) {
		t.Errorf("expected ErrNoPermissionAccessApps, got %v", err)
	}

	apps, total, err := svc.ListCreatorApplications(ctx, ListCreatorApplicationsQuery{ActorRole: admin.Role, Status: model.CreatorApplicationStatusPending, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListCreatorApplications error: %v", err)
	}
	if total != 2 || len(apps) != 2 {
		t.Errorf("expected 2 pending applications, got total=%d len=%d", total, len(apps))
	}
	if apps[0].User.Username == "" {
		t.Errorf("expected applicant preloaded")
	}

	apps, total, err = svc.ListCreatorApplications(ctx, ListCreatorApplicationsQuery{ActorRole: admin.Role, Status: model.CreatorApplicationStatusApproved, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListCreatorApplications error: %v", err)
	}
	if total != 0 || len(apps) != 0 {
		t.Errorf("expected no approved applications, got total=%d len=%d", total, len(apps))
	}
}

func TestReviewCreatorApplication(t *testing.T) {
	ctx := context.Background()
	svc, notifier, _, db := newTestService(t)
	super := seedUser(t, db, "super", model.RoleSuperAdmin)
	guest := seedUser(t, db, "guest", model.RoleGuest)
	other := seedUser(t, db, "other", model.RoleGuest)

	appID, err := svc.ApplyForCreator(ctx, guest, "promote me")
	if err != nil {
		t.Fatalf("ApplyForCreator error: %v", err)
	}

	if err := svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{Actor: other, AppID: appID, Action: "approve"}); !errors.Is(err, ErrNoPermissionReviewApps) {
		t.Errorf("expected ErrNoPermissionReviewApps, got %v", err)
	}

	if err := svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{Actor: super, AppID: appID, Action: "maybe"}); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("expected ErrInvalidAction, got %v", err)
	}

	if err := svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{Actor: super, AppID: 99999, Action: "approve"}); !errors.Is(err, ErrApplicationNotFound) {
		t.Errorf("expected ErrApplicationNotFound, got %v", err)
	}

	notifier.calls = nil
	if err := svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{Actor: super, AppID: appID, Action: "approve"}); err != nil {
		t.Fatalf("ReviewCreatorApplication error: %v", err)
	}
	var u model.User
	if err := db.First(&u, guest.ID).Error; err != nil {
		t.Fatalf("failed to load user: %v", err)
	}
	if u.Role != model.RoleContributor {
		t.Errorf("expected contributor after approval, got %s", u.Role)
	}
	if len(notifier.calls) != 1 {
		t.Errorf("expected applicant notification, got %v", notifier.calls)
	}

	if err := svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{Actor: super, AppID: appID, Action: "reject"}); !errors.Is(err, ErrApplicationProcessed) {
		t.Errorf("expected ErrApplicationProcessed, got %v", err)
	}

	appID2, err := svc.ApplyForCreator(ctx, other, "promote me too")
	if err != nil {
		t.Fatalf("ApplyForCreator error: %v", err)
	}
	if err := svc.ReviewCreatorApplication(ctx, ReviewCreatorApplicationRequest{Actor: super, AppID: appID2, Action: "reject", Reason: "not enough posts"}); err != nil {
		t.Fatalf("ReviewCreatorApplication error: %v", err)
	}
	var app model.CreatorApplication
	if err := db.First(&app, appID2).Error; err != nil {
		t.Fatalf("failed to load application: %v", err)
	}
	if app.Status != model.CreatorApplicationStatusRejected {
		t.Errorf("expected rejected status, got %s", app.Status)
	}
	if app.ReviewReason != "not enough posts" {
		t.Errorf("expected review reason, got %q", app.ReviewReason)
	}
}

func TestListUsers_SearchAndPagination(t *testing.T) {
	ctx := context.Background()
	svc, _, _, db := newTestService(t)
	seedUser(t, db, "alice", model.RoleGuest)
	seedUser(t, db, "bob", model.RoleAuthor)
	seedUser(t, db, "carol", model.RoleAdmin)

	users, total, err := svc.ListUsers(ctx, "", 1, 10)
	if err != nil {
		t.Fatalf("ListUsers error: %v", err)
	}
	if total != 3 || len(users) != 3 {
		t.Errorf("expected 3 users, got total=%d len=%d", total, len(users))
	}

	users, total, err = svc.ListUsers(ctx, "ali", 1, 10)
	if err != nil {
		t.Fatalf("ListUsers error: %v", err)
	}
	if total != 1 || len(users) != 1 || users[0].Username != "alice" {
		t.Errorf("expected only alice, got total=%d users=%v", total, users)
	}

	users, total, err = svc.ListUsers(ctx, "", 2, 2)
	if err != nil {
		t.Fatalf("ListUsers error: %v", err)
	}
	if total != 3 || len(users) != 1 {
		t.Errorf("expected 1 user on page 2, got total=%d len=%d", total, len(users))
	}
}
