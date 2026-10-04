package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// The stored avatar is rendered as an <img src> by the public pages and by the
// theme's comment widget, so a value the browser would execute as a script
// must be refused instead of stored.
func TestUpdateProfile_RejectsUnsafeAvatar(t *testing.T) {
	for _, tc := range []struct {
		name   string
		avatar string
	}{
		{"javascript scheme", `javascript:document.title="xss"`},
		{"data scheme", "data:text/html,<script>alert(1)</script>"},
		{"vbscript scheme", "vbscript:msgbox(1)"},
		{"mixed-case scheme", "JaVaScRiPt:alert(1)"},
		{"protocol-relative host", "//evil.example.com/avatar.png"},
		{"relative path", "avatar.png"},
		{"path traversal", "/uploads/../../secret.png"},
		{"uploads directory itself", "/uploads/"},
		{"theme asset path", "/theme-assets/evil.svg"},
		{"embedded newline", "java\nscript:alert(1)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, files, db := newTestService(t)
			u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
			u.Avatar = "/uploads/original.png"
			if err := db.Save(&u).Error; err != nil {
				t.Fatalf("failed to set avatar: %v", err)
			}

			avatar := tc.avatar
			_, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &avatar})
			if !errors.Is(err, ErrInvalidAvatar) {
				t.Fatalf("expected ErrInvalidAvatar, got %v", err)
			}

			// The rejected value must not be stored, and the previous avatar
			// file must not have been deleted on the way.
			stored, err := svc.GetCurrentUser(context.Background(), u.ID)
			if err != nil {
				t.Fatalf("failed to reload user: %v", err)
			}
			if stored.Avatar != "/uploads/original.png" {
				t.Errorf("expected the previous avatar to survive, got %q", stored.Avatar)
			}
			if len(files.deleted) != 0 {
				t.Errorf("expected no deletions, got %v", files.deleted)
			}
		})
	}
}

// Upload paths and absolute http(s) URLs stay accepted: the admin SPA sends
// the URL returned by the upload endpoint, which is either form.
func TestUpdateProfile_AcceptsSafeAvatar(t *testing.T) {
	for _, avatar := range []string{
		"",
		"/uploads/new-avatar.png",
		"https://cdn.example.com/avatars/alice.png",
	} {
		svc, _, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)

		value := avatar
		user, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &value})
		if err != nil {
			t.Fatalf("UpdateProfile(%q) error: %v", avatar, err)
		}
		if user.Avatar != avatar {
			t.Errorf("expected avatar %q, got %q", avatar, user.Avatar)
		}
	}
}

func TestUpdateProfile_DeletesOldAvatar(t *testing.T) {
	t.Run("owned media record is deleted", func(t *testing.T) {
		svc, files, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
		u.Avatar = "/uploads/old-avatar.png"
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("failed to set avatar: %v", err)
		}
		seedMedia(t, db, u.ID, "/uploads/old-avatar.png")

		newAvatar := "/uploads/new-avatar.png"
		user, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &newAvatar})
		if err != nil {
			t.Fatalf("UpdateProfile error: %v", err)
		}
		if user.Avatar != newAvatar {
			t.Errorf("expected new avatar, got %s", user.Avatar)
		}
		if len(files.deleted) != 1 || files.deleted[0] != "/uploads/old-avatar.png" {
			t.Errorf("expected old avatar deleted, got %v", files.deleted)
		}
	})

	// security: the stored avatar URL is client-controlled; pointing it at
	// someone else's media (or an arbitrary S3 URL) must never trigger a
	// delete of that object when the avatar changes again.
	t.Run("media owned by another user is not deleted", func(t *testing.T) {
		svc, files, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
		other := seedUser(t, db, "bob@example.com", "password123", model.RoleGuest, true)
		u.Avatar = "/uploads/victim.png"
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("failed to set avatar: %v", err)
		}
		seedMedia(t, db, other.ID, "/uploads/victim.png")

		newAvatar := "/uploads/new-avatar.png"
		if _, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &newAvatar}); err != nil {
			t.Fatalf("UpdateProfile error: %v", err)
		}
		if len(files.deleted) != 0 {
			t.Errorf("expected no deletions, got %v", files.deleted)
		}
	})

	t.Run("unknown URL is not deleted", func(t *testing.T) {
		svc, files, db := newTestService(t)
		u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)
		u.Avatar = "https://bucket.s3.amazonaws.com/anything/secret.png"
		if err := db.Save(&u).Error; err != nil {
			t.Fatalf("failed to set avatar: %v", err)
		}

		newAvatar := "/uploads/new-avatar.png"
		if _, err := svc.UpdateProfile(context.Background(), u.ID, UpdateProfileRequest{Avatar: &newAvatar}); err != nil {
			t.Fatalf("UpdateProfile error: %v", err)
		}
		if len(files.deleted) != 0 {
			t.Errorf("expected no deletions, got %v", files.deleted)
		}
	})
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
