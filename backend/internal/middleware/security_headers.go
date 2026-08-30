package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets baseline browser defenses on every response. It is
// deliberately conservative (no CSP, which needs per-deployment tuning): these
// headers mainly stop uploaded or proxied content from being sniffed into
// executable documents and from being framed by third parties.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}
