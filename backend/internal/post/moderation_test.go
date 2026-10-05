package post

import (
	"context"
	"errors"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func TestModeration_ApproveRejectResubmit(t *testing.T) {
	svc, notifier, _, db := newTestService(t)
	ctx := context.Background()
	contributor := seedUser(t, db, "contrib", model.RoleContributor)

	post, err := svc.Create(ctx, contributor.Role, contributor.ID, CreateRequest{Slug: "mod-test", Title: "t", Content: "c", Category: "1"})
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
