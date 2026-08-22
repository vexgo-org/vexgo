// Package auth implements authentication, account management and user privacy.
// It currently only hosts user privacy filtering (used by the post and comment
// domains); it grows as the legacy handler package migrates.
package auth

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// FilterUserByPrivacy filters user information based on privacy settings:
// the user themselves and administrators see everything; other viewers see
// information according to the user's privacy settings.
func FilterUserByPrivacy(user *model.User, viewerID uint, viewerRole string) {
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
