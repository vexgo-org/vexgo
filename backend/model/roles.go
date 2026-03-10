package model

// Role constant definitions
const (
	RoleSuperAdmin  = "super_admin"
	RoleAdmin       = "admin"
	RoleAuthor      = "author"
	RoleContributor = "contributor"
	RoleGuest       = "guest"
)

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
