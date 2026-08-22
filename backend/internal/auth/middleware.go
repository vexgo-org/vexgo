package auth

import (
	"net/http"

	"github.com/vexgo-org/vexgo/backend/internal/config"

	"github.com/gin-gonic/gin"
)

// LocalLoginGuard returns a middleware that rejects password-based login
// when local login is disabled in the SSO configuration.
func LocalLoginGuard(sso *config.SSOConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sso.AllowLocalLogin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "local login is disabled, please use SSO",
			})
			return
		}
		c.Next()
	}
}
