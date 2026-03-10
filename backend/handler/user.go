package handler

import (
	"net/http"
	"strconv"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetUserList gets user list
func GetUserList(c *gin.Context) {
	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var users []model.User
	var total int64

	// Query user list
	query := db.Model(&model.User{})

	// Count total
	db.Model(&model.User{}).Count(&total)

	// Paginated query
	query.Offset((page - 1) * limit).
		Limit(limit).
		Order("id ASC").
		Find(&users)

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": totalPages,
		},
	})
}

// UpdateUserRole updates user role
func UpdateUserRole(c *gin.Context) {
	// Get current user information from context
	currentUserInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No user information provided"})
		return
	}

	// Convert user information from map to model.User
	userMap, ok := currentUserInterface.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User information format error"})
		return
	}

	currentUser := model.User{
		ID:       userMap["id"].(uint),
		Username: userMap["username"].(string),
		Role:     userMap["role"].(string),
	}

	// Get user ID to update
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user model.User

	// Find target user
	if err := db.First(&user, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query user"})
		return
	}

	// Cannot modify own role
	if user.ID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify own role"})
		return
	}

	// Cannot modify super admin's role (unless current user is also super admin)
	if user.Role == model.RoleSuperAdmin && currentUser.Role != model.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to modify super admin role"})
		return
	}

	// Parse request parameters
	var req struct {
		Role string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role is valid
	validRoles := map[string]bool{
		model.RoleSuperAdmin:  true,
		model.RoleAdmin:       true,
		model.RoleAuthor:      true,
		model.RoleContributor: true,
		model.RoleGuest:       true,
	}

	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
		return
	}

	// Permission check
	// Super admin can set any role (including making other users super admin)
	// But cannot downgrade own super admin privileges
	if currentUser.Role == model.RoleSuperAdmin {
		// If current user is super admin, can set any role
		// Note: super admin cannot downgrade own role
		if user.ID == currentUser.ID && req.Role != model.RoleSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Super admin cannot modify own role"})
			return
		}
		user.Role = req.Role
	} else if currentUser.Role == model.RoleAdmin {
		// Admin can only set user roles to author, contributor, or guest (cannot set to admin or super admin)
		if req.Role == model.RoleAuthor || req.Role == model.RoleContributor || req.Role == model.RoleGuest {
			user.Role = req.Role
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin can only set user roles to author, contributor, or guest"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to modify user role"})
		return
	}

	// Save updates
	if err := db.Model(&user).Select("Role").Updates(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User role updated successfully",
		"user":    user,
	})
}
