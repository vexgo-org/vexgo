// backend/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"

	"vexgo/backend/config"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Global database connection, needs to be set during initialization
var db *gorm.DB

// SetDB sets database connection
func SetDB(database *gorm.DB) {
	db = database
}

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No authentication information provided"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication format error"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			// Ensure using HS256 signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return config.JWTSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := uint(claims["user_id"].(float64))

		// Verify password version
		if db != nil {
			var user model.User
			if err := db.First(&user, userID).Error; err == nil {
				// Check if password version in token matches current user's password version
				if tokenPasswordVersion, ok := claims["password_version"].(float64); ok {
					if int(tokenPasswordVersion) != user.PasswordVersion {
						c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Password has been changed, please log in again"})
						return
					}
				}
			}
		}

		c.Set("userID", userID)
		c.Set("username", claims["username"].(string))

		// Get complete user information and set in context
		userInfo := map[string]interface{}{
			"id":       userID,
			"username": claims["username"].(string),
		}

		// Safely get role information
		if role, ok := claims["role"].(string); ok {
			userInfo["role"] = role
		} else {
			userInfo["role"] = ""
		}

		c.Set("user", userInfo)

		c.Next()
	}
}

// OptionalJWTAuth attempts to parse JWT from Authorization header and write user info to context,
// If not provided or parsing fails, do not block the request (used for public endpoints that can sense logged-in user but don't require authentication).
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
			// Do not interrupt request, only ignore invalid token
			c.Next()
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := uint(0)
		validToken := true

		// Verify password version
		if db != nil {
			if uid, ok := claims["user_id"].(float64); ok {
				userID = uint(uid)
				var user model.User
				if err := db.First(&user, userID).Error; err == nil {
					// Check if password version in token matches current user's password version
					if tokenPasswordVersion, ok := claims["password_version"].(float64); ok {
						if int(tokenPasswordVersion) != user.PasswordVersion {
							validToken = false
						}
					}
				}
			}
		}

		if !validToken {
			// Do not interrupt request, only ignore invalid token
			c.Next()
			return
		}

		// Safely set userID/username/role
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
