package auth

import (
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Token fields must be scrubbed for every viewer — even self and admin —
// because they never belong in a rendered object, regardless of role.
func TestFilterUserByPrivacy_ScrubsTokens(t *testing.T) {
	expires := time.Now().Add(5 * time.Minute)
	u := model.User{
		ID:                7,
		Username:          "alice",
		Email:             "alice@example.com",
		VerificationToken: "verify-abc",
		TokenExpiresAt:    &expires,
		PendingEmail:      "new@example.com",
	}

	for _, tc := range []struct {
		name   string
		viewer uint
		role   string
	}{
		{"self", 7, model.RoleGuest},
		{"admin", 99, model.RoleAdmin},
		{"super admin", 99, model.RoleSuperAdmin},
		{"stranger", 99, model.RoleGuest},
	} {
		target := u
		FilterUserByPrivacy(&target, tc.viewer, tc.role)
		if target.VerificationToken != "" || target.PendingEmail != "" || target.TokenExpiresAt != nil {
			t.Errorf("%s: tokens must be scrubbed, got %+v", tc.name, target)
		}
		if target.Email != "alice@example.com" {
			t.Errorf("%s: email handling must not change email here, got %q", tc.name, target.Email)
		}
	}
}
