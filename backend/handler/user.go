package handler

import (
	"net/http"
	"strconv"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetUserList 获取用户列表
func GetUserList(c *gin.Context) {
	// 分页参数
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

	// 查询用户列表
	query := db.Model(&model.User{})

	// 统计总数
	db.Model(&model.User{}).Count(&total)

	// 分页查询
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

// UpdateUserRole 更新用户角色
func UpdateUserRole(c *gin.Context) {
	// 从上下文中获取当前用户信息
	currentUserInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供用户信息"})
		return
	}

	// 将用户信息从map转换为model.User
	userMap, ok := currentUserInterface.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户信息格式错误"})
		return
	}

	currentUser := model.User{
		ID:       userMap["id"].(uint),
		Username: userMap["username"].(string),
		Role:     userMap["role"].(string),
	}

	// 获取要更新的用户ID
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var user model.User

	// 查找目标用户
	if err := db.First(&user, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	// 不能修改自己的角色
	if user.ID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改自己的角色"})
		return
	}

	// 不能修改超级管理员的角色（除非当前用户也是超级管理员）
	if user.Role == model.RoleSuperAdmin && currentUser.Role != model.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限修改超级管理员角色"})
		return
	}

	// 解析请求参数
	var req struct {
		Role string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证角色是否有效
	validRoles := map[string]bool{
		model.RoleSuperAdmin:  true,
		model.RoleAdmin:       true,
		model.RoleAuthor:      true,
		model.RoleContributor: true,
		model.RoleGuest:       true,
	}

	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色"})
		return
	}

	// 权限检查
	// 超级管理员可以设置任何角色（包括其他用户成为超级管理员）
	// 但不能降级自己的超级管理员权限
	if currentUser.Role == model.RoleSuperAdmin {
		// 如果当前用户是超级管理员，可以设置任何角色
		// 注意：不允许超级管理员将自己的角色降级
		if user.ID == currentUser.ID && req.Role != model.RoleSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "超级管理员不能修改自己的角色"})
			return
		}
		user.Role = req.Role
	} else if currentUser.Role == model.RoleAdmin {
		// 管理员只能将用户角色设置为作者、投稿者或访客（不能设置为管理员或超级管理员）
		if req.Role == model.RoleAuthor || req.Role == model.RoleContributor || req.Role == model.RoleGuest {
			user.Role = req.Role
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "管理员只能将用户角色设置为作者、投稿者或访客"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限修改用户角色"})
		return
	}

	// 保存更新
	if err := db.Model(&user).Select("Role").Updates(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户角色失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "用户角色更新成功",
		"user":    user,
	})
}
