package post

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// ToggleLike likes or unlikes a post and notifies the author on like.
// The like insert is conflict-safe (unique post_id+user_id index), so
// concurrent toggles cannot create duplicate rows.
func (s *Service) ToggleLike(ctx context.Context, postID, userID uint) (isLiked bool, count int64, err error) {
	existing, err := s.repo.FindLike(ctx, postID, userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// A repository failure is not "not liked yet": falling through to the
		// like branch would insert a row over an unreadable state and report
		// success, hiding the outage from the caller.
		return false, 0, fmt.Errorf("find like: %w", err)
	}
	if err == nil {
		// Already liked -> unlike. Deleting by primary key is idempotent,
		// so a concurrent unlike of the same like cannot error.
		if err := s.repo.DeleteLike(ctx, existing); err != nil {
			return false, 0, err
		}
		c, countErr := s.repo.CountLikes(ctx, postID)
		if countErr != nil {
			slog.Warn("failed to count post likes", "postID", postID, "err", countErr)
		}
		return false, c, nil
	}

	// Not liked (yet) -> like. If a concurrent request inserted the same
	// like first, RowsAffected is 0 and we report already-liked instead of
	// failing or inserting a duplicate.
	created, err := s.repo.CreateLikeIfAbsent(ctx, postID, userID)
	if err != nil {
		return false, 0, err
	}
	likesCount, err := s.repo.CountLikes(ctx, postID)
	if err != nil {
		// The like itself succeeded; reporting a fabricated zero would tell the
		// user their like did not register.
		slog.Warn("failed to count post likes", "postID", postID, "err", err)
	}
	count = likesCount
	if !created {
		return true, count, nil
	}

	// Notify the post author. The like is already recorded, so a failed
	// lookup is logged instead of failing the request; the caller must still
	// learn their like registered.
	post, err := s.repo.FindByID(ctx, strconv.FormatUint(uint64(postID), 10))
	if err != nil {
		slog.Warn("failed to load the liked post", "postID", postID, "err", err)
		return true, count, nil
	}
	if post.AuthorID == userID {
		return true, count, nil
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		slog.Warn("failed to load the liking user", "userID", userID, "err", err)
		return true, count, nil
	}
	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:      post.AuthorID,
		Type:        model.NotificationTypeLike,
		Title:       "The post received likes",
		Content:     fmt.Sprintf("User \"%s\" liked your post \"%s\"", user.Username, post.Title),
		RelatedID:   strconv.FormatUint(uint64(postID), 10),
		RelatedType: model.NotificationRelatedTypePost,
	}); err != nil {
		slog.Warn("failed to create like notification", "err", err)
	}

	return true, count, nil
}

// LikeStatus returns whether the user liked the post and the total like count.
func (s *Service) LikeStatus(ctx context.Context, postID, userID uint) (isLiked bool, count int64) {
	if userID != 0 {
		if _, err := s.repo.FindLike(ctx, postID, userID); err == nil {
			isLiked = true
		}
	}
	count, countErr := s.repo.CountLikes(ctx, postID)
	if countErr != nil {
		slog.Warn("failed to count post likes", "postID", postID, "err", countErr)
	}
	return isLiked, count
}
