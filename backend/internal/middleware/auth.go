// Package middleware provides HTTP middleware for authentication,
// authorization and request logging.
package middleware

import (
	"net/http"
	"strings"

	"vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Context keys used to carry the authenticated user through the gin context.
const (
	CtxUserKey   = "user"
	CtxUserIDKey = "userID"
)

// Auth holds the database connection and JWT secret used to validate tokens.
type Auth struct {
	db        *gorm.DB
	jwtSecret []byte
}

// NewAuth creates the authentication middleware with the given database and JWT secret.
func NewAuth(db *gorm.DB, jwtSecret []byte) *Auth {
	return &Auth{db: db, jwtSecret: jwtSecret}
}

// CurrentUser extracts the authenticated user from the gin context.
// It returns the user and true when a valid JWT was parsed; otherwise
// it returns a zero-value User and false.
func CurrentUser(c *gin.Context) (model.User, bool) {
	userContext, exists := c.Get(CtxUserKey)
	if !exists {
		return model.User{}, false
	}
	userMap, ok := userContext.(map[string]any)
	if !ok {
		return model.User{}, false
	}
	id, _ := userMap["id"].(uint)
	username, _ := userMap["username"].(string)
	role, _ := userMap["role"].(string)
	return model.User{ID: id, Username: username, Role: role}, true
}

// CurrentUserID extracts only the user ID from the context.
// Returns 0 when no user is authenticated.
func CurrentUserID(c *gin.Context) uint {
	if uid, exists := c.Get(CtxUserIDKey); exists {
		switch v := uid.(type) {
		case uint:
			return v
		case int:
			return uint(v)
		case float64:
			return uint(v)
		}
	}
	return 0
}

// JWTAuth authenticates requests via a Bearer JWT and writes the user info
// into the gin context.
func (a *Auth) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No authentication information provided"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication format error"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (any, error) {
			// Ensure using HS256 signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return a.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := uint(claims["user_id"].(float64))

		// Verify password version and get latest role
		var dbRole string
		if a.db != nil {
			var user model.User
			if err := a.db.First(&user, userID).Error; err == nil {
				// Check if password version in token matches current user's password version
				if tokenPasswordVersion, ok := claims["password_version"].(float64); ok {
					if int(tokenPasswordVersion) != user.PasswordVersion {
						c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Password has been changed, please log in again"})
						return
					}
				}
				// Check if token was issued before last login to prevent token reuse
				if tokenIat, ok := claims["iat"].(float64); ok {
					if int64(tokenIat) < user.LastLoginAt.Unix() {
						c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token is invalid, please log in again"})
						return
					}
				}
				dbRole = user.Role
			}
		}

		c.Set(CtxUserIDKey, userID)

		// Get complete user information and set in context
		userInfo := map[string]any{
			"id":       userID,
			"username": claims["username"].(string),
		}

		// Safely get role information, prefer database role
		if dbRole != "" {
			userInfo["role"] = dbRole
		} else if role, ok := claims["role"].(string); ok {
			userInfo["role"] = role
		} else {
			userInfo["role"] = ""
		}

		c.Set(CtxUserKey, userInfo)

		c.Next()
	}
}

// OptionalJWTAuth attempts to parse JWT from Authorization header and write user info to context,
// If not provided or parsing fails, do not block the request (used for public endpoints that can sense logged-in user but don't require authentication).
func (a *Auth) OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return a.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			// Do not interrupt request, only ignore invalid token
			c.Next()
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := uint(0)
		validToken := true

		// Verify password version and get latest role
		var dbRole string
		if a.db != nil {
			if uid, ok := claims["user_id"].(float64); ok {
				userID = uint(uid)
				var user model.User
				if err := a.db.First(&user, userID).Error; err == nil {
					// Check if password version in token matches current user's password version
					if tokenPasswordVersion, ok := claims["password_version"].(float64); ok {
						if int(tokenPasswordVersion) != user.PasswordVersion {
							validToken = false
						}
					}
					// Check if token was issued before last login to prevent token reuse
					if tokenIat, ok := claims["iat"].(float64); ok {
						if int64(tokenIat) < user.LastLoginAt.Unix() {
							validToken = false
						}
					}
					dbRole = user.Role
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
			c.Set(CtxUserIDKey, uint(uid))
		}
		userInfo := map[string]any{
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

		if dbRole != "" {
			userInfo["role"] = dbRole
		} else if role, ok := claims["role"].(string); ok {
			userInfo["role"] = role
		}
		c.Set(CtxUserKey, userInfo)

		c.Next()
	}
}
