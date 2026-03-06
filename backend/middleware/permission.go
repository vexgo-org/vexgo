package middleware

import (
	"net/http"

	"vexgo/backend/handler"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware 权限控制中间件
func PermissionMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取用户ID
		userIDInterface, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未提供用户信息"})
			return
		}

		userID := userIDInterface.(uint)

		// 查询用户信息
		var user model.User
		if err := handler.DB().First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}

		// 检查用户角色是否符合要求
		hasPermission := false
		for _, requiredRole := range requiredRoles {
			if user.Role == requiredRole {
				hasPermission = true
				break
			}
		}

		// 超级管理员拥有所有权限
		if user.Role == model.RoleSuperAdmin {
			hasPermission = true
		}

		if !hasPermission {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			return
		}

		// 将用户信息存储到上下文中供后续使用
		c.Set("user", map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		})
		c.Next()
	}
}
