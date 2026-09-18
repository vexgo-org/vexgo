package auth

import (
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Avatar values a browser would resolve as a scriptable URI must be dropped
// before they reach a rendered object (the theme's comment widget writes the
// stored value straight to the DOM), for every viewer.
func TestFilterUserByPrivacy_ScrubsUnsafeAvatar(t *testing.T) {
	unsafe := []string{
		`javascript:document.title="xss"`,
		`data:text/html,<script>alert(1)</script>`,
		`vbscript:msgbox(1)`,
		`JaVaScRiPt:alert(1)`,
		`//evil.example.com/avatar.png`,
		`/uploads/../../etc/passwd`,
		`/uploads/`,
		`relative/avatar.png`,
		"javascript:\n:alert(1)",
	}

	for _, avatar := range unsafe {
		target := model.User{ID: 7, Username: "alice", Avatar: avatar}
		// Self and admin viewers must be scrubbed too: the value is not safe
		// for anyone's browser.
		FilterUserByPrivacy(&target, 7, model.RoleSuperAdmin)
		if target.Avatar != "" {
			t.Errorf("unsafe avatar %q must be scrubbed, got %q", avatar, target.Avatar)
		}
	}
}

// Legitimate avatars — upload paths and absolute http(s) URLs — must survive
// the scrub unchanged.
func TestFilterUserByPrivacy_KeepsSafeAvatar(t *testing.T) {
	safe := []string{
		"",
		"/uploads/avatar-1.png",
		"https://cdn.example.com/a/b.png",
		"http://127.0.0.1:38081/uploads/a.png",
	}

	for _, avatar := range safe {
		target := model.User{ID: 7, Username: "alice", Avatar: avatar}
		FilterUserByPrivacy(&target, 99, model.RoleGuest)
		if target.Avatar != avatar {
			t.Errorf("safe avatar %q must be preserved, got %q", avatar, target.Avatar)
		}
	}
}

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
