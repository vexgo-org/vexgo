// Package auth implements authentication, account management and user privacy.
// It currently only hosts user privacy filtering (used by the post and comment
// domains); it grows as the legacy handler package migrates.
package auth

import (
	"net/url"
	"strings"
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
//
// The avatar is normalized for every viewer as well: a value a browser would
// resolve as a scriptable URI is dropped here, so rows written before
// UpdateProfile rejected them cannot reach an <img src> sink.
func FilterUserByPrivacy(user *model.User, viewerID uint, viewerRole string) {
	user.VerificationToken = ""
	user.TokenExpiresAt = nil
	user.PendingEmail = ""
	if !isSafeAvatarURL(user.Avatar) {
		user.Avatar = ""
	}

	// Check if viewer is the user themselves or an admin
	isSelf := viewerID == user.ID
	isAdmin := model.IsAdmin(viewerRole)

	// If not self and not admin, filter according to privacy settings
	if !isSelf && !isAdmin {
		// First check profile visibility setting
		if user.ProfileVisibility == model.ProfileVisibilityPrivate {
			// If set to private, hide all personal information
			user.Email = ""
			user.Birthday = ""
			user.Bio = ""
		} else {
			// If public, filter according to individual hide settings
			if user.HideEmail {
				user.Email = ""
			}
			if user.HideBirthday {
				user.Birthday = ""
			}
			if user.HideBio {
				user.Bio = ""
			}
		}
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
