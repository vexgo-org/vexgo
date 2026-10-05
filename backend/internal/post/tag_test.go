package post

import (
	"context"
	"errors"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func TestDeleteTag_EmptySucceeds(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()

	tag, err := svc.CreateTag(ctx, model.RoleContributor, "golang")
	if err != nil {
		t.Fatalf("CreateTag error: %v", err)
	}

	if err := svc.DeleteTag(ctx, model.RoleContributor, tag.ID); err != nil {
		t.Fatalf("DeleteTag error: %v", err)
	}

	var count int64
	db.Model(&model.Tag{}).Count(&count)
	if count != 0 {
		t.Errorf("tag not deleted")
	}
}

func TestDeleteTag_InUseRejected(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "author", model.RoleAuthor)

	tag, err := svc.CreateTag(ctx, model.RoleContributor, "golang")
	if err != nil {
		t.Fatalf("CreateTag error: %v", err)
	}
	if _, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "tagged", Title: "t", Content: "c", Category: "tech", Tags: []string{"golang"}, Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	err = svc.DeleteTag(ctx, model.RoleContributor, tag.ID)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("expected InUseError, got %v", err)
	}
	if inUse.Count != 1 {
		t.Errorf("expected usage count 1, got %d", inUse.Count)
	}

	// The tag row and its join row must survive a rejected delete.
	var count int64
	db.Model(&model.Tag{}).Where("id = ?", tag.ID).Count(&count)
	if count != 1 {
		t.Errorf("in-use tag was deleted")
	}
	db.Table("post_tags").Where("tag_id = ?", tag.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected the join row to remain, got %d", count)
	}
}

func TestDeleteTag_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	if err := svc.DeleteTag(ctx, model.RoleContributor, 42); !errors.Is(err, ErrTagNotFound) {
		t.Errorf("expected ErrTagNotFound, got %v", err)
	}
}

func TestDeleteTag_ForbiddenRole(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	for _, role := range []string{model.RoleGuest, ""} {
		if err := svc.DeleteTag(ctx, role, 1); !errors.Is(err, ErrForbidden) {
			t.Errorf("role %q: expected ErrForbidden, got %v", role, err)
		}
	}
}
