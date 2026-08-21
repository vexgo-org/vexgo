package post

import (
	"context"
	"fmt"
	"strconv"

	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
)

// ToggleLike likes or unlikes a post and notifies the author on like.
// The like insert is conflict-safe (unique post_id+user_id index), so
// concurrent toggles cannot create duplicate rows.
func (s *Service) ToggleLike(ctx context.Context, postID, userID uint) (isLiked bool, count int64, err error) {
	existing, err := s.repo.FindLike(ctx, postID, userID)
	if err == nil {
		// Already liked -> unlike. Deleting by primary key is idempotent,
		// so a concurrent unlike of the same like cannot error.
		if err := s.repo.DeleteLike(ctx, existing); err != nil {
			return false, 0, err
		}
		c, _ := s.repo.CountLikes(ctx, postID)
		return false, c, nil
	}

	// Not liked (yet) -> like. If a concurrent request inserted the same
	// like first, RowsAffected is 0 and we report already-liked instead of
	// failing or inserting a duplicate.
	created, err := s.repo.CreateLikeIfAbsent(ctx, postID, userID)
	if err != nil {
		return false, 0, err
	}
	count, _ = s.repo.CountLikes(ctx, postID)
	if !created {
		return true, count, nil
	}

	// Create notification for post author
	post, err := s.repo.FindByID(ctx, fmt.Sprintf("%d", postID))
	if err == nil && post.AuthorID != userID {
		user, err := s.repo.FindUserByID(ctx, userID)
		if err == nil {
			if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
				UserID:      post.AuthorID,
				Type:        model.NotificationTypeLike,
				Title:       "The post received likes",
				Content:     fmt.Sprintf("User \"%s\" liked your post \"%s\"", user.Username, post.Title),
				RelatedID:   strconv.FormatUint(uint64(postID), 10),
				RelatedType: model.NotificationRelatedTypePost,
			}); err != nil {
				logrus.WithError(err).Warn("failed to create like notification")
			}
		}
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
	count, _ = s.repo.CountLikes(ctx, postID)
	return isLiked, count
}
