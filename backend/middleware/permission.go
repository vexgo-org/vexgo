package middleware

import (
	"net/http"

	"vexgo/backend/handler"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware checks if user has required role
func PermissionMiddleware(requiredRoles ...string) gin.HandlerFunc {
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
		if err := handler.DB().First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
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
