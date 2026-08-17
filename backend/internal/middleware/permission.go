package middleware

import (
	"net/http"

	"vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// Permission checks if the authenticated user has one of the required roles.
func (a *Auth) Permission(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userIDInterface, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
			return
		}

		userID := userIDInterface.(uint)

		// Query user information
		var user model.User
		if err := a.db.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No user information provided"})
			return
		}

		// Check if user role meets requirements
		hasPermission := false
		for _, requiredRole := range requiredRoles {
			if user.Role == requiredRole {
				hasPermission = true
				break
			}
		}

		// Super admin has all permissions
		if user.Role == model.RoleSuperAdmin {
			hasPermission = true
		}

		if !hasPermission {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			return
		}

		// Store user information in context for later use
		c.Set("user", map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		})
		c.Next()
	}
}
