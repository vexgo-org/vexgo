package comment

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fakeNotifier records notification calls instead of touching the DB.
type fakeNotifier struct {
	calls []model.NotificationType
}

func (f *fakeNotifier) CreateNotification(_ context.Context, input model.NotificationInput) error {
	f.calls = append(f.calls, input.Type)
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

// TC-CMOD-001: with every moderation switch off (fresh install), a new
// comment is published immediately with no moderation artifacts, and the post
// author is notified.
func TestCreate_AllSwitchesOff_Publishes(t *testing.T) {
	ctx := context.Background()
	svc, notifier, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	comment, count, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "nice post"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusPublished {
		t.Errorf("expected published, got %s", comment.Status)
	}
	if comment.ModerationReason != "" {
		t.Errorf("expected no moderation reason, got %q", comment.ModerationReason)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != model.NotificationTypeComment {
		t.Errorf("expected one comment notification, got %v", notifier.calls)
	}

	// author commenting on own post → no notification
	notifier.calls = nil
	_, _, err = svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: author.ID, Content: "self comment"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected no notification for own post, got %v", notifier.calls)
	}
}

// TC-CMOD-002: with only manual review on, every new comment is held as
// pending and nobody is notified.
func TestCreate_ManualReviewOnly_Pends(t *testing.T) {
	ctx := context.Background()
	svc, notifier, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		ManualReviewEnabled: true,
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	comment, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "needs review"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusPending {
		t.Errorf("expected pending, got %s", comment.Status)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected no notifications for a pending comment, got %v", notifier.calls)
	}
}

// TC-CMOD-003: the keyword filter alone rejects a matching comment with the
// matched keyword persisted as the reason, and nobody is notified.
func TestCreate_KeywordHit_RejectsWithReason(t *testing.T) {
	ctx := context.Background()
	svc, notifier, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		KeywordFilterEnabled: true,
		BlockKeywords:        "spam, ad",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	// Matching is case-insensitive and reports the first matched keyword.
	comment, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "totally SPAM-free"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusRejected {
		t.Errorf("expected rejected, got %s", comment.Status)
	}
	if comment.ModerationReason != "Contains blocked keyword: spam" {
		t.Errorf("expected keyword rejection reason, got %q", comment.ModerationReason)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected no notifications for a rejected comment, got %v", notifier.calls)
	}
}

// TC-CMOD-004: the keyword filter alone publishes a comment without hits.
func TestCreate_KeywordNoHit_Publishes(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		KeywordFilterEnabled: true,
		BlockKeywords:        "spam,ad",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	comment, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "lovely article"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.Status != model.CommentStatusPublished {
		t.Errorf("expected published, got %s", comment.Status)
	}
}

func TestCreate_ReplyNotifiesParentAuthor(t *testing.T) {
	ctx := context.Background()
	svc, notifier, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)
	replier := seedUser(t, db, "replier", model.RoleGuest)

	_, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "first"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var parent model.Comment
	if err := db.First(&parent).Error; err != nil {
		t.Fatalf("failed to load parent: %v", err)
	}
	parentID := parent.ID

	notifier.calls = nil
	_, _, err = svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: replier.ID, Content: "reply", ParentID: &parentID})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(notifier.calls) != 2 {
		t.Errorf("expected post + parent notifications, got %v", notifier.calls)
	}
}

func TestListByPost_PublishedOnlyAndPrivacy(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	commenter.ProfileVisibility = "private"
	db.Save(&commenter)

	if _, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "published one"}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: author.ID, Content: "pending one"}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var pending model.Comment
	if err := db.Where("content = ?", "pending one").First(&pending).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pending.Status = model.CommentStatusPending
	db.Save(&pending)

	comments, err := svc.ListByPost(ctx, "1", 0, "")
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 published comment, got %d", len(comments))
	}
	if comments[0].User.Email != "" {
		t.Errorf("expected private email filtered for anonymous viewer")
	}

	comments, err = svc.ListByPost(ctx, "1", author.ID, author.Role)
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if comments[0].User.Email != "" {
		t.Errorf("expected email hidden from author too (not self), got %q", comments[0].User.Email)
	}

	admin := seedUser(t, db, "admin", model.RoleSuperAdmin)
	comments, err = svc.ListByPost(ctx, "1", admin.ID, admin.Role)
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if comments[0].User.Email == "" {
		t.Errorf("expected admin to see private email")
	}
}

func TestDelete_Permissions(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "to delete"}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var comment model.Comment
	if err := db.First(&comment).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	id := comment.ID

	other := seedUser(t, db, "other", model.RoleGuest)
	if _, err := svc.Delete(ctx, strconv.FormatUint(uint64(id), 10), other.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}

	count, err := svc.Delete(ctx, strconv.FormatUint(uint64(id), 10), commenter.ID)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 after delete, got %d", count)
	}

	if _, err := svc.Delete(ctx, strconv.FormatUint(uint64(id), 10), commenter.ID); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound, got %v", err)
	}
}

func TestSetStatus(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "moderate me"}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	var comment model.Comment
	if err := db.First(&comment).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}

	updated, err := svc.SetStatus(ctx, strconv.FormatUint(uint64(comment.ID), 10), model.CommentStatusPublished)
	if err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if updated.Status != model.CommentStatusPublished {
		t.Errorf("expected published, got %s", updated.Status)
	}

	if _, err := svc.SetStatus(ctx, "99999", model.CommentStatusPublished); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("expected ErrCommentNotFound, got %v", err)
	}
}

func TestListModeration(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "one"}); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "two"}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	list, total, err := svc.ListModeration(ctx, model.CommentStatusPublished, 1, 1)
	if err != nil {
		t.Fatalf("ListModeration error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 item per page, got %d", len(list))
	}

	list, total, err = svc.ListModeration(ctx, model.CommentStatusPending, 1, 10)
	if err != nil {
		t.Fatalf("ListModeration error: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Errorf("expected empty pending queue, got total=%d len=%d", total, len(list))
	}
}

func TestUpdateModerationConfig_PreservesApiKey(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)

	config, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		ApiKey: "secret-key",
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

	_, err = svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{})
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

// newTestServiceWithCipher builds a comment service wired with a real cipher
// (test passphrase) plus the underlying DB, for encryption-at-rest tests.
func newTestServiceWithCipher(t *testing.T) (*Service, *gorm.DB, *secrets.Cipher) {
	t.Helper()
	db := newTestDB(t)
	cipher, err := secrets.New("test-encryption-key")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	svc := NewService(Deps{DB: db, Notifier: &fakeNotifier{}, Cipher: cipher})
	return svc, db, cipher
}

// TC-ENC-015: with a cipher wired, the moderation API key is stored as
// enc:v1: ciphertext, never as plaintext, while GET responses stay masked.
func TestUpdateModerationConfig_EncryptsApiKeyWithCipher(t *testing.T) {
	ctx := context.Background()
	svc, db, cipher := newTestServiceWithCipher(t)

	resp, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		ApiKey: "secret-key",
	})
	if err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}
	if resp.ApiKey != "" {
		t.Errorf("expected api key masked in response, got %q", resp.ApiKey)
	}

	var stored model.CommentModerationConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !secrets.IsEncrypted(stored.ApiKey) {
		t.Errorf("expected stored api key to carry the encrypted marker, got %q", stored.ApiKey)
	}
	if strings.Contains(stored.ApiKey, "secret-key") {
		t.Errorf("stored api key must not contain the plaintext, got %q", stored.ApiKey)
	}

	decrypted, err := cipher.Decrypt(stored.ApiKey)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "secret-key" {
		t.Errorf("expected stored ciphertext to decrypt to the original, got %q", decrypted)
	}

	masked, err := svc.GetModerationConfig(ctx)
	if err != nil {
		t.Fatalf("GetModerationConfig error: %v", err)
	}
	if masked.ApiKey != "" {
		t.Errorf("expected masked api key from GET, got %q", masked.ApiKey)
	}
}

// TC-ENC-016: without a configured cipher, the moderation API key is stored
// as plaintext exactly as before the feature (no-key fallback).
func TestUpdateModerationConfig_PlaintextWithoutCipher(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := newTestServiceWithCipher(t)
	svc.cipher = nil

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		ApiKey: "secret-key",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	var stored model.CommentModerationConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if stored.ApiKey != "secret-key" {
		t.Errorf("expected plaintext fallback storage, got %q", stored.ApiKey)
	}
}

// TC-ENC-017: an update without an API key keeps the stored ciphertext, and
// the preserved value still decrypts.
func TestUpdateModerationConfig_EmptyApiKeyKeepsStoredValueDecryptable(t *testing.T) {
	ctx := context.Background()
	svc, db, cipher := newTestServiceWithCipher(t)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		ApiKey: "secret-key",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{}); err != nil {
		t.Fatalf("UpdateModerationConfig (no key) error: %v", err)
	}

	var stored model.CommentModerationConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load failed: %v", err)
	}
	decrypted, err := cipher.Decrypt(stored.ApiKey)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != "secret-key" {
		t.Errorf("expected preserved api key to stay decryptable, got %q", decrypted)
	}
}

// TC-ENC-018: an undecryptable stored api key (wrong/rotated key) must not
// crash the server; the key is treated as unset on read.
func TestModerationConfig_UndecryptableKeyTreatedAsUnset(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := newTestServiceWithCipher(t)

	other, err := secrets.New("other-key")
	if err != nil {
		t.Fatalf("failed to create other cipher: %v", err)
	}
	encrypted, err := other.Encrypt("rotated-key")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if err := db.Create(&model.CommentModerationConfig{
		LLMReviewEnabled: true,
		ApiKey:           encrypted,
	}).Error; err != nil {
		t.Fatalf("failed to seed moderation config: %v", err)
	}

	config, err := svc.GetModerationConfig(ctx)
	if err != nil {
		t.Fatalf("GetModerationConfig error: %v", err)
	}
	if config.ApiKey != "" {
		t.Errorf("expected undecryptable key to be treated as unset, got %q", config.ApiKey)
	}

	// The internal read path must also degrade gracefully, not panic.
	if _, err := svc.moderationConfig(ctx); err != nil {
		t.Fatalf("moderationConfig error: %v", err)
	}
}

// flakyModerationRepo fails every moderation-config read to simulate a
// transient persistence error. It embeds the real Repository so only the
// read needs overriding.
type flakyModerationRepo struct {
	Repository
}

func (f *flakyModerationRepo) GetModerationConfig(_ context.Context) (model.CommentModerationConfig, error) {
	return model.CommentModerationConfig{}, errors.New("transient persistence failure")
}

// Regression: when the read that decides create-vs-update fails with a real
// (non-"not found") error, the update must abort instead of wiping the
// stored API key with a zero value.
func TestUpdateModerationConfig_RepoErrorDoesNotWipeStoredKey(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := newTestServiceWithCipher(t)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		ApiKey: "secret-key",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	var before model.CommentModerationConfig
	if err := db.First(&before).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}

	svc.repo = &flakyModerationRepo{Repository: svc.repo}

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{}); err == nil {
		t.Fatal("expected an error when the moderation-config read fails, got nil")
	}

	var after model.CommentModerationConfig
	if err := db.First(&after).Error; err != nil {
		t.Fatalf("failed to load stored config: %v", err)
	}
	if after.ApiKey != before.ApiKey {
		t.Errorf("stored api key must not change when the update fails: before %q, after %q", before.ApiKey, after.ApiKey)
	}
}

// Approving a rejected comment must clear its moderation reason: the comment
// becomes publicly visible and its internal moderation data must not leak
// through the public comment API.
func TestSetStatus_ApproveClearsModerationReason(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestService(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)

	if _, err := svc.UpdateModerationConfig(ctx, UpdateModerationConfigRequest{
		KeywordFilterEnabled: true,
		BlockKeywords:        "spam",
	}); err != nil {
		t.Fatalf("UpdateModerationConfig error: %v", err)
	}

	comment, _, err := svc.Create(ctx, CreateRequest{PostID: post.ID, UserID: commenter.ID, Content: "buy spam"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if comment.ModerationReason == "" {
		t.Fatalf("expected a moderation reason on the rejected comment")
	}

	approved, err := svc.SetStatus(ctx, strconv.FormatUint(uint64(comment.ID), 10), model.CommentStatusPublished)
	if err != nil {
		t.Fatalf("SetStatus error: %v", err)
	}
	if approved.Status != model.CommentStatusPublished {
		t.Errorf("expected published, got %s", approved.Status)
	}
	if approved.ModerationReason != "" {
		t.Errorf("expected moderation reason cleared on approval, got %q", approved.ModerationReason)
	}

	var stored model.Comment
	if err := db.First(&stored, comment.ID).Error; err != nil {
		t.Fatalf("load stored comment: %v", err)
	}
	if stored.ModerationReason != "" {
		t.Errorf("expected stored moderation reason cleared, got %q", stored.ModerationReason)
	}

	// The public read path must not expose the internal reason.
	public, err := svc.ListByPost(ctx, strconv.FormatUint(uint64(post.ID), 10), 0, "")
	if err != nil {
		t.Fatalf("ListByPost error: %v", err)
	}
	if len(public) != 1 || public[0].ModerationReason != "" {
		t.Errorf("expected public list without moderation reason, got %+v", public)
	}
}
