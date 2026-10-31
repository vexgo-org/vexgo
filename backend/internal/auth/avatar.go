package auth

import (
	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// userAvatarURL returns the original public URL of user's avatar.
func UserAvatarURL(user *model.User) string {
	if user == nil ||
		user.Avatar == nil {
		return ""
	}

	return asset.OriginalURL(user.Avatar.PublicID)
}
