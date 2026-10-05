package post

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ListModerationQuery carries the moderation status, pagination and search.
type ListModerationQuery struct {
	Status model.PostStatus
	Page   int
	Limit  int
	Search string
}

// ListModeration returns the paginated posts with the given status for the
// moderation queue, with an optional search across title/content/username.
func (s *Service) ListModeration(ctx context.Context, q ListModerationQuery) ([]model.Post, int64, error) {
	offset := (q.Page - 1) * q.Limit
	return s.repo.ListModeration(ctx, q.Status, offset, q.Limit, q.Search)
}

// Approve approves a post and notifies its author.
func (s *Service) Approve(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusPublished
	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:      post.AuthorID,
		Type:        model.NotificationTypeReview,
		Title:       "Post approved",
		Content:     fmt.Sprintf("Your post \"%s\" has been approved", post.Title),
		RelatedID:   id,
		RelatedType: model.NotificationRelatedTypePost,
	}); err != nil {
		slog.Warn("failed to create post approved notification", "err", err)
	}

	return post, nil
}

// Reject rejects a post with a reason and notifies its author.
func (s *Service) Reject(ctx context.Context, id, rejectionReason string) (*model.Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Status = model.PostStatusRejected
	post.RejectionReason = rejectionReason
	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:      post.AuthorID,
		Type:        model.NotificationTypeReview,
		Title:       "Post Rejected",
		Content:     fmt.Sprintf("Your post \"%s\" has been rejected, reason: %s", post.Title, rejectionReason),
		RelatedID:   id,
		RelatedType: model.NotificationRelatedTypePost,
	}); err != nil {
		slog.Warn("failed to create post rejected notification", "err", err)
	}

	return post, nil
}

// Resubmit moves a rejected post back to pending.
func (s *Service) Resubmit(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.FindByIDPreloadTags(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	// Check if post status is rejected
	if post.Status != model.PostStatusRejected {
		return nil, ErrBadRequest
	}

	post.Status = model.PostStatusPending
	post.RejectionReason = ""
	if err := s.repo.Save(ctx, post); err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	return post, nil
}
