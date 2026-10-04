package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// GetCurrentUser loads a user by ID.
func (s *Service) GetCurrentUser(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateProfile updates the optional profile fields, deleting the old avatar
// file when the avatar changes.
func (s *Service) UpdateProfile(ctx context.Context, userID uint, req UpdateProfileRequest) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if req.Avatar != nil {
		// The value is rendered as an <img src> by the public pages and by
		// the theme's comment widget, which writes it straight to the DOM;
		// anything the browser would resolve as a script is refused here
		// (see isSafeAvatarURL) instead of being stored for those sinks.
		if !isSafeAvatarURL(*req.Avatar) {
			return nil, ErrInvalidAvatar
		}

		// Delete the old avatar file — but only when it maps to a media
		// record owned by this user. The stored URL is client-controlled
		// (set by a previous profile update), and Storage.Delete resolves it
		// back to a storage key (for S3, any key in the bucket), so deleting
		// on faith would let a user wipe arbitrary objects by first pointing
		// their avatar at them.
		if *req.Avatar != user.Avatar && user.Avatar != "" {
			s.deleteOldAvatar(ctx, userID, user.Avatar)
		}
		user.Avatar = *req.Avatar
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Birthday != nil {
		user.Birthday = *req.Birthday
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if err := s.repo.SaveUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// deleteOldAvatar removes the file behind a replaced avatar when — and only
// when — it is a media file owned by the acting user. Anything else
// (external URLs, records that no longer exist, other users' files, DB
// errors) is logged and skipped: the cleanup is best-effort and must never
// widen into deleting unmanaged or third-party storage objects.
func (s *Service) deleteOldAvatar(ctx context.Context, userID uint, url string) {
	media, err := s.repo.FindMediaByURL(ctx, url)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		slog.Warn("old avatar has no media record, skipping deletion", "userID", userID, "url", url)
		return
	case err != nil:
		slog.Warn("failed to look up old avatar media record, skipping deletion", "userID", userID, "url", url, "err", err)
		return
	}
	if media.UserID != userID {
		slog.Warn("old avatar media record belongs to another user, skipping deletion", "userID", userID, "ownerID", media.UserID, "url", url)
		return
	}
	if err := s.files.Delete(ctx, url); err != nil {
		slog.Warn("failed to delete old avatar", "url", url, "err", err)
	}
}

// UpdateSettings updates the user's privacy settings.
func (s *Service) UpdateSettings(ctx context.Context, userID uint, req UpdateSettingsRequest) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if req.ProfileVisibility != nil {
		user.ProfileVisibility = *req.ProfileVisibility
	}
	if req.HideEmail != nil {
		user.HideEmail = *req.HideEmail
	}
	if req.HideBirthday != nil {
		user.HideBirthday = *req.HideBirthday
	}
	if req.HideBio != nil {
		user.HideBio = *req.HideBio
	}

	if err := s.repo.SaveUser(ctx, user); err != nil {
		return nil, ErrSaveSettings
	}

	return user, nil
}
