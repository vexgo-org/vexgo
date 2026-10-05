package post

import (
	"context"
	"errors"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// errLikeRead stands in for a like table that cannot be read.
var errLikeRead = errors.New("like read failed")

// failingFindLikeRepo delegates to a working repository but fails the like
// lookup, so ToggleLike sees a repository error other than "not liked yet".
type failingFindLikeRepo struct {
	Repository
}

func (failingFindLikeRepo) FindLike(context.Context, uint, uint) (*model.Like, error) {
	return nil, errLikeRead
}

func TestToggleLike(t *testing.T) {
	svc, notifier, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	liker := seedUser(t, db, "liker", model.RoleGuest)

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "like-test", Title: "t", Content: "c", Category: "1", Status: model.PostStatusPublished})
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

// TestToggleLike_ReportsRepositoryFailure pins that a failed like lookup is
// reported instead of being read as "not liked yet", which used to insert the
// opposite like and report a fabricated success.
func TestToggleLike_ReportsRepositoryFailure(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	author := seedUser(t, db, "author", model.RoleAuthor)
	liker := seedUser(t, db, "liker", model.RoleGuest)

	post, err := svc.Create(ctx, author.Role, author.ID, CreateRequest{Slug: "like-failure", Title: "t", Content: "c", Category: "1", Status: model.PostStatusPublished})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	svc.repo = failingFindLikeRepo{Repository: svc.repo}

	isLiked, count, err := svc.ToggleLike(ctx, post.ID, liker.ID)
	if !errors.Is(err, errLikeRead) {
		t.Fatalf("expected the lookup failure to be reported, got isLiked=%v count=%d err=%v", isLiked, count, err)
	}

	// The failed lookup must not have written a like as a side effect.
	var likes int64
	db.Model(&model.Like{}).Count(&likes)
	if likes != 0 {
		t.Errorf("expected no like row after a failed lookup, got %d", likes)
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
