package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

func TestUpdateProfile_SetsOwnedAvatar(t *testing.T) {
	svc, _, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
	avatar := seedAsset(t, db, u.ID, model.AssetVisibilityPublic, model.BlobTypeImage)

	user, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{
		AvatarPublicID: &avatar.PublicID,
	})
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}
	if user.AvatarID == nil || *user.AvatarID != avatar.ID {
		t.Fatalf("avatar id = %v, want %d", user.AvatarID, avatar.ID)
	}
	if user.Avatar == nil || user.Avatar.PublicID != avatar.PublicID {
		t.Fatalf("avatar was not preloaded after the update: %+v", user.Avatar)
	}
}

// A profile form that omits avatarPublicID must leave the existing avatar
// alone; the normal form never sends the field.
func TestUpdateProfile_PreservesAvatarWhenOmitted(t *testing.T) {
	svc, _, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
	avatar := seedAsset(t, db, u.ID, model.AssetVisibilityPublic, model.BlobTypeImage)
	if err := db.Model(&model.User{}).Where("id = ?", u.ID).
		Update("avatar_id", avatar.ID).Error; err != nil {
		t.Fatalf("failed to set avatar: %v", err)
	}

	bio := "updated bio"
	user, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Bio: &bio})
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}
	if user.AvatarID == nil || *user.AvatarID != avatar.ID {
		t.Fatalf("avatar id = %v, want %d", user.AvatarID, avatar.ID)
	}
	if user.Bio != bio {
		t.Errorf("bio = %q, want %q", user.Bio, bio)
	}
}

// Only a public image asset owned by the acting user may become an avatar.
func TestUpdateProfile_RejectsUnavailableAvatar(t *testing.T) {
	for _, tc := range []struct {
		name     string
		publicID func(t *testing.T, db *gorm.DB, userID uint) uuid.UUID
	}{
		{
			name: "owned by another user",
			publicID: func(t *testing.T, db *gorm.DB, _ uint) uuid.UUID {
				other := seedUser(t, db, "bob@example.com", "password123", model.RoleGuest, true)
				return seedAsset(t, db, other.ID, model.AssetVisibilityPublic, model.BlobTypeImage).PublicID
			},
		},
		{
			name: "private asset",
			publicID: func(t *testing.T, db *gorm.DB, userID uint) uuid.UUID {
				return seedAsset(t, db, userID, model.AssetVisibilityPrivate, model.BlobTypeImage).PublicID
			},
		},
		{
			name: "non-image asset",
			publicID: func(t *testing.T, db *gorm.DB, userID uint) uuid.UUID {
				return seedAsset(t, db, userID, model.AssetVisibilityPublic, model.BlobTypeVideo).PublicID
			},
		},
		{
			name: "unknown asset",
			publicID: func(t *testing.T, _ *gorm.DB, _ uint) uuid.UUID {
				return uuid.New()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, db := newTestService(t)
			u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
			previous := seedAsset(t, db, u.ID, model.AssetVisibilityPublic, model.BlobTypeImage)
			if err := db.Model(&model.User{}).Where("id = ?", u.ID).
				Update("avatar_id", previous.ID).Error; err != nil {
				t.Fatalf("failed to set previous avatar: %v", err)
			}

			candidate := tc.publicID(t, db, u.ID)
			_, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{
				AvatarPublicID: &candidate,
			})
			if !errors.Is(err, ErrInvalidAvatar) {
				t.Fatalf("expected ErrInvalidAvatar, got %v", err)
			}

			user, err := svc.GetCurrentUser(context.Background(), u.ID)
			if err != nil {
				t.Fatalf("failed to reload user: %v", err)
			}
			if user.AvatarID == nil || *user.AvatarID != previous.ID {
				t.Errorf("avatar id = %v, want the previous avatar %d", user.AvatarID, previous.ID)
			}
		})
	}
}

func TestUpdateSettings(t *testing.T) {
	svc, _, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)

	hideEmail := true
	visibility := "private"
	user, err := svc.UpdateSettings(context.Background(), u.ID, UpdateSettingsRequest{
		ProfileVisibility: &visibility,
		HideEmail:         &hideEmail,
	})
	if err != nil {
		t.Fatalf("UpdateSettings error: %v", err)
	}
	if !user.HideEmail || user.ProfileVisibility != "private" {
		t.Errorf("expected settings applied, got %+v", user)
	}

	if _, err := svc.UpdateSettings(context.Background(), 99999, UpdateSettingsRequest{}); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
