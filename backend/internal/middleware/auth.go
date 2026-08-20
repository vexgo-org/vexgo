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
// It returns the user and true when the context holds a complete user
// record (id, username and role); otherwise it returns a zero-value User
// and false.
func CurrentUser(c *gin.Context) (model.User, bool) {
	userContext, exists := c.Get(CtxUserKey)
	if !exists {
		return model.User{}, false
	}
	userMap, ok := userContext.(map[string]any)
	if !ok {
		return model.User{}, false
	}
	id, ok := userMap["id"].(uint)
	if !ok {
		return model.User{}, false
	}
	username, ok := userMap["username"].(string)
	if !ok {
		return model.User{}, false
	}
	role, ok := userMap["role"].(string)
	if !ok {
		return model.User{}, false
	}
	return model.User{ID: id, Username: username, Role: role}, true
}

// claimsUserID extracts the user_id claim as a uint, returning 0 when the
// claim is missing or not numeric.
func claimsUserID(claims jwt.MapClaims) uint {
	if v, ok := claims["user_id"].(float64); ok {
		return uint(v)
	}
	return 0
}

// claimsUsername extracts the username claim, returning "" when missing.
func claimsUsername(claims jwt.MapClaims) string {
	if v, ok := claims["username"].(string); ok {
		return v
	}
	return ""
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

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}
		userID := claimsUserID(claims)
		if userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Verify password version and get latest role
		var dbRole string
		if a.db != nil {
			var user model.User
			if err := a.db.First(&user, userID).Error; err != nil {
				// The user was deleted after the token was issued; do not
				// fall back to the token's role claim.
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User no longer exists"})
				return
			}
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

		c.Set(CtxUserIDKey, userID)

		// Get complete user information and set in context
		userInfo := map[string]any{
			"id":       userID,
			"username": claimsUsername(claims),
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

// OptionalJWTAuth attempts to parse a JWT from the Authorization header and
// write the user info to the context. If the token is missing or invalid the
// request is not blocked (used for public endpoints that can detect a
// logged-in user without requiring authentication).
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

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			// Do not interrupt request, only ignore unparseable token
			c.Next()
			return
		}
		userID := claimsUserID(claims)
		validToken := true

		// Verify password version and get latest role. A user that no longer
		// exists makes the token invalid (treated as anonymous).
		var dbRole string
		if a.db != nil && userID != 0 {
			var user model.User
			if err := a.db.First(&user, userID).Error; err != nil {
				validToken = false
			} else {
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

		if !validToken {
			// Do not interrupt request, only ignore invalid token
			c.Next()
			return
		}

		// Safely set userID/username/role
		if userID != 0 {
			c.Set(CtxUserIDKey, userID)
		}
		userInfo := map[string]any{
			"id":       userID,
			"username": claimsUsername(claims),
			"role":     "",
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
