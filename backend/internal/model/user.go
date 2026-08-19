package model

import "time"

// Role constant definitions
const (
	RoleSuperAdmin  = "super_admin"
	RoleAdmin       = "admin"
	RoleAuthor      = "author"
	RoleContributor = "contributor"
	RoleGuest       = "guest"
)

// User is an account with a role, authentication fields and privacy settings.
type User struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	Username          string     `json:"username" binding:"required" gorm:"size:100;uniqueIndex"`
	Email             string     `json:"email" binding:"required,email" gorm:"size:255;uniqueIndex"`
	Password          string     `json:"-"`                                       // not serialized
	Role              string     `json:"role" gorm:"size:50"`                     // super_admin/admin/author/contributor/guest
	Avatar            string     `json:"avatar,omitempty"`                        // avatar URL
	EmailVerified     bool       `json:"email_verified"`                          // whether email is verified
	VerificationToken string     `json:"verification_token" gorm:"size:255"`      // verification token
	TokenExpiresAt    *time.Time `json:"token_expires_at"`                        // token expiration time (can be NULL)
	PendingEmail      string     `json:"pending_email,omitempty" gorm:"size:255"` // new email pending confirmation (for email change)
	PasswordVersion   int        `json:"-" gorm:"default:1"`                      // password version, used to invalidate old tokens after password modification
	LastLoginAt       time.Time  `json:"last_login_at"`                           // last login time to invalidate old tokens
	Birthday          string     `json:"birthday,omitempty"`                      // birthday
	Bio               string     `json:"bio,omitempty"`                           // personal bio
	CreatedAt         time.Time  `json:"createdAt"`                               // registration time
	// Privacy settings
	ProfileVisibility string `json:"profile_visibility,omitempty" gorm:"size:20;default:'public'"` // public/private
	HideEmail         bool   `json:"hide_email,omitempty" gorm:"default:false"`                    // hide email
	HideBirthday      bool   `json:"hide_birthday,omitempty" gorm:"default:false"`                 // hide birthday
	HideBio           bool   `json:"hide_bio,omitempty" gorm:"default:false"`                      // hide bio
}

// IsSuperAdmin checks if user is super admin
func IsSuperAdmin(user User) bool {
	return user.Role == RoleSuperAdmin
}

// IsAdmin checks if user is admin (including super admin)
func IsAdmin(user User) bool {
	return user.Role == RoleAdmin || user.Role == RoleSuperAdmin
}

// IsAuthor checks if user is author (including admin and super admin)
func IsAuthor(user User) bool {
	return user.Role == RoleAuthor || user.Role == RoleAdmin || user.Role == RoleSuperAdmin
}

// IsContributor checks if user is contributor (including higher privilege roles)
func IsContributor(user User) bool {
	return user.Role == RoleContributor || user.Role == RoleAuthor ||
		user.Role == RoleAdmin || user.Role == RoleSuperAdmin
}
