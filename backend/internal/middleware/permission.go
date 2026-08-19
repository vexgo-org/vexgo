package middleware

import (
	"net/http"
	"slices"

	"vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// Permission checks if the authenticated user has one of the required roles.
// The role is taken from the gin context, where JWTAuth already verified the
// user against the database on this request; when only a user ID is present
// (middleware chains without JWTAuth, e.g. tests), the user is loaded from
// the database as a fallback.
func (a *Auth) Permission(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userIDInterface, exists := c.Get(CtxUserIDKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
			return
		}

		userID := userIDInterface.(uint)

		// Prefer the role already resolved by JWTAuth (fresh per request),
		// falling back to a DB lookup when the context only carries the ID.
		role, ok := contextRole(c)
		if !ok {
			if a.db == nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No user information provided"})
				return
			}
			var user model.User
			if err := a.db.First(&user, userID).Error; err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No user information provided"})
				return
			}
			role = user.Role
			// Store user information in context for later use
			c.Set(CtxUserKey, map[string]any{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			})
		}

		// Check if user role meets requirements
		hasPermission := slices.Contains(requiredRoles, role)

		// Super admin has all permissions
		if model.IsSuperAdmin(role) {
			hasPermission = true
		}

		if !hasPermission {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			return
		}

		c.Next()
	}
}

// contextRole returns the role stored in the gin context by JWTAuth.
func contextRole(c *gin.Context) (string, bool) {
	userContext, exists := c.Get(CtxUserKey)
	if !exists {
		return "", false
	}
	userMap, ok := userContext.(map[string]any)
	if !ok {
		return "", false
	}
	role, ok := userMap["role"].(string)
	if !ok || role == "" {
		return "", false
	}
	return role, true
}