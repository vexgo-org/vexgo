// Package auth implements authentication, account management and user privacy.
// It currently only hosts user privacy filtering (used by the post and comment
// domains); it grows as the legacy handler package migrates.
package auth

import (
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// FilterUserByPrivacy filters user information based on privacy settings:
// the user themselves and administrators see everything; other viewers see
// information according to the user's privacy settings.
//
// Emailed-link tokens are always scrubbed, regardless of viewer: they must
// never reach a rendered object (account takeover via leaked links). This is
// defense in depth on top of `json:"-"` in the model, guarding render paths
// that bypass encoding/json.
// // The avatar is normalized for every viewer as well: a value a browser would
// resolve as a scriptable URI is dropped here, so rows written before
// UpdateProfile rejected them cannot reach an <img src> sink.
//
// A viewer who is neither the account owner nor an administrator receives a
// public profile only. Account state — role, verification flag, last login
// time, and the owner's own privacy switches — is cleared, because none of it
// is needed to render a post or a comment and all of it helps an attacker
// pick targets. The email address is personal data: an anonymous caller never
// receives it, and a signed-in caller only while the owner left the address
// public.
func FilterUserByPrivacy(user *model.User, viewerID uint, viewerRole string) {
	// Fields that never belong in a rendered object, whatever the viewer is.
	user.VerificationToken = ""
	user.TokenExpiresAt = nil
	user.PendingEmail = ""
	if !isSafeAvatarURL(user.Avatar) {
		user.Avatar = ""
	}

	// The owner and administrators see the account as it is. A zero user id
	// is not the owner of anything: an author block that failed to preload
	// must not be mistaken for "self" just because the viewer is anonymous.
	if user.ID != 0 && (viewerID == user.ID || model.IsAdmin(viewerRole)) {
		return
	}

	// The owner's switches decide what happens to the profile fields below,
	// so read them before the account state is cleared.
	isPrivate := user.ProfileVisibility == model.ProfileVisibilityPrivate
	hideEmail, hideBirthday, hideBio := user.HideEmail, user.HideBirthday, user.HideBio

	user.Role = ""
	user.EmailVerified = false
	user.LastLoginAt = time.Time{}
	user.ProfileVisibility = ""
	user.HideEmail = false
	user.HideBirthday = false
	user.HideBio = false

	if viewerID == 0 || isPrivate || hideEmail {
		user.Email = ""
	}
	if isPrivate || hideBirthday {
		user.Birthday = ""
	}
	if isPrivate || hideBio {
		user.Bio = ""
	}
}

// isSafeAvatarURL reports whether an avatar value may be handed to a browser.
//
// The stored value ends up in an <img src>: on server-rendered pages, whose
// templates neutralize a scriptable scheme themselves, and — for comment
// authors — in the theme's comment widget, which writes it straight to the
// DOM. Only local upload paths and absolute http(s) URLs are accepted, so
// values like `javascript:` or `data:` can never become a scriptable sink.
// The empty string is safe: it means "no avatar".
func isSafeAvatarURL(raw string) bool {
	if raw == "" {
		return true
	}

	// Control characters have no place in a URL and are a common way to
	// smuggle a scheme past naive prefix checks.
	if strings.ContainsFunc(raw, unicode.IsControl) {
		return false
	}

	if rel, ok := strings.CutPrefix(raw, "/uploads/"); ok {
		return rel != "" && !strings.Contains(rel, "..")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	// A scheme-less value (`relative/path.png`, `//host/x.png`) resolves
	// against the current document and can point off-origin, so it is
	// rejected here rather than guessed at.
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}
