package model

import (
	"slices"
	"time"
)

// Role constant definitions
const (
	RoleSuperAdmin  = "super_admin"
	RoleAdmin       = "admin"
	RoleAuthor      = "author"
	RoleContributor = "contributor"
	RoleGuest       = "guest"
)

// Profile visibility values.
const (
	ProfileVisibilityPublic  = "public"
	ProfileVisibilityPrivate = "private"
)

// Verification token prefixes. All token kinds share the verification_token
// column, so the prefix distinguishes email verification, password reset and
// email-change tokens and prevents one kind from being used as another.
const (
	TokenPrefixVerify      = "verify-"
	TokenPrefixReset       = "reset-"
	TokenPrefixEmailChange = "email-change-"
)

// User is an account with a role, authentication fields and privacy settings.
type User struct {
	ID            uint   `gorm:"primaryKey"`
	Username      string `binding:"required" gorm:"size:100;uniqueIndex"`
	Email         string `binding:"required,email" gorm:"size:255;uniqueIndex"`
	Password      string // not serialized
	Role          string `gorm:"size:50"` // super_admin/admin/author/contributor/guest
	EmailVerified bool   // whether email is verified
	// VerificationToken / TokenExpiresAt / PendingEmail hold live account
	// tokens and must never serialize: model.User instances are rendered raw
	// by several endpoints, and a leaked emailed-link token allows account
	// takeover. Clients get the pending email via the VerifyEmail response.
	VerificationToken string     `gorm:"size:255"` // verification token (emailed link token)
	TokenExpiresAt    *time.Time // expiry of the verification/reset/email-change token
	PendingEmail      string     `gorm:"size:255"`  // new email pending confirmation (for email change)
	PasswordVersion   int        `gorm:"default:1"` // password version, used to invalidate old tokens after password modification
	LastLoginAt       time.Time  // last login time to invalidate old tokens
	Birthday          string     // birthday
	Bio               string     // personal bio
	CreatedAt         time.Time  // registration time
	// Privacy settings
	ProfileVisibility string `gorm:"size:20;default:'public'"` // public/private
	HideEmail         bool   `gorm:"default:false"`            // hide email
	HideBirthday      bool   `gorm:"default:false"`            // hide birthday
	HideBio           bool   `gorm:"default:false"`            // hide bio

	AvatarID *uint
	Avatar   *Asset `gorm:"foreignKey:AvatarID"`
}

// IsSuperAdmin reports whether the role is super admin.
func IsSuperAdmin(role string) bool {
	return role == RoleSuperAdmin
}

// IsAdmin reports whether the role is admin-level (admin or super admin).
func IsAdmin(role string) bool {
	return role == RoleAdmin || role == RoleSuperAdmin
}

// IsAuthor reports whether the role is author-level (author, admin or super admin).
func IsAuthor(role string) bool {
	return role == RoleAuthor || role == RoleAdmin || role == RoleSuperAdmin
}

// IsContributor reports whether the role is contributor-level (contributor or higher).
func IsContributor(role string) bool {
	return role == RoleContributor || role == RoleAuthor ||
		role == RoleAdmin || role == RoleSuperAdmin
}

// ValidRole checks whether a role is valid.
func ValidRole(role string) bool {
	validRoles := []string{
		RoleSuperAdmin,
		RoleAdmin,
		RoleAuthor,
		RoleContributor,
		RoleGuest,
	}
	return slices.Contains(validRoles, role)
}
