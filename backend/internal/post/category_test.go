package post

import (
	"context"
	"errors"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func TestDeleteCategory_EmptySucceeds(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()

	category, err := svc.CreateCategory(ctx, model.RoleContributor, "tech", "d")
	if err != nil {
		t.Fatalf("CreateCategory error: %v", err)
	}

	if err := svc.DeleteCategory(ctx, model.RoleContributor, category.ID); err != nil {
		t.Fatalf("DeleteCategory error: %v", err)
	}

	var count int64
	db.Model(&model.Category{}).Count(&count)
	if count != 0 {
		t.Errorf("category not deleted")
	}
}

func TestDeleteCategory_InUseRejected(t *testing.T) {
	svc, _, _, db := newTestService(t)
	ctx := context.Background()
	user := seedUser(t, db, "author", model.RoleAuthor)

	category, err := svc.CreateCategory(ctx, model.RoleContributor, "tech", "d")
	if err != nil {
		t.Fatalf("CreateCategory error: %v", err)
	}
	if _, err := svc.Create(ctx, user.Role, user.ID, CreateRequest{Slug: "uses-tech", Title: "t", Content: "c", Category: "tech", Status: model.PostStatusPublished}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	err = svc.DeleteCategory(ctx, model.RoleContributor, category.ID)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("expected InUseError, got %v", err)
	}
	if inUse.Count != 1 {
		t.Errorf("expected usage count 1, got %d", inUse.Count)
	}

	// The category must survive a rejected delete.
	var count int64
	db.Model(&model.Category{}).Where("id = ?", category.ID).Count(&count)
	if count != 1 {
		t.Errorf("in-use category was deleted")
	}
}

func TestDeleteCategory_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	if err := svc.DeleteCategory(ctx, model.RoleContributor, 42); !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestDeleteCategory_ForbiddenRole(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	for _, role := range []string{model.RoleGuest, ""} {
		if err := svc.DeleteCategory(ctx, role, 1); !errors.Is(err, ErrForbidden) {
			t.Errorf("role %q: expected ErrForbidden, got %v", role, err)
		}
	}
}
