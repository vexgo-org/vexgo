// backend/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"

	"blog-system/backend/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未提供认证信息"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "认证格式错误"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			// 确保使用 HS256 签名方法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return config.JWTSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		c.Set("userID", uint(claims["user_id"].(float64)))
		c.Set("username", claims["username"].(string))

		// 获取用户完整信息并设置到上下文中
		userInfo := map[string]interface{}{
			"id":       uint(claims["user_id"].(float64)),
			"username": claims["username"].(string),
		}

		// 安全地获取角色信息
		if role, ok := claims["role"].(string); ok {
			userInfo["role"] = role
		} else {
			userInfo["role"] = ""
		}

		c.Set("user", userInfo)

		c.Next()
	}
}

// OptionalJWTAuth 尝试解析 Authorization header 中的 JWT 并将用户信息写入上下文，
// 如果没有提供或解析失败则不阻止请求（用于公开接口可以感知登录用户但不强制认证）。
func OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.Next()
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return config.JWTSecret, nil
		})

		if err != nil || !token.Valid {
			// 不中断请求，仅忽略无效 token
			c.Next()
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		// 安全地设置 userID/username/role
		if uid, ok := claims["user_id"].(float64); ok {
			c.Set("userID", uint(uid))
		}
		if uname, ok := claims["username"].(string); ok {
			c.Set("username", uname)
		}
		userInfo := map[string]interface{}{
			"id":       uint(0),
			"username": "",
			"role":     "",
		}
		if uid, ok := claims["user_id"].(float64); ok {
			userInfo["id"] = uint(uid)
		}
		if uname, ok := claims["username"].(string); ok {
			userInfo["username"] = uname
		}
		if role, ok := claims["role"].(string); ok {
			userInfo["role"] = role
		}
		c.Set("user", userInfo)

		c.Next()
	}
}
