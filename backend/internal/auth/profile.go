package auth

import (
	"context"
	"errors"

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

	if req.AvatarPublicID != nil {
		id, err := s.repo.FindAvatarIDByPublicID(ctx, userID, *req.AvatarPublicID)
		if err != nil {
			return nil, err
		}

		user.AvatarID = &id
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

	return s.repo.FindUserByID(ctx, userID)
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
