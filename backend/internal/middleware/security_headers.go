package middleware

import "github.com/gin-gonic/gin"

// ContentSecurityPolicy is intentionally limited to directives that never
// change how a theme or the admin SPA renders: fetching is left unrestricted
// because themes legitimately load their own assets, fonts and inline scripts.
// It still forbids plugins/objects, pins <base> to this origin (blocking
// base-tag injection) and pins framing to same-origin, mirroring
// X-Frame-Options for browsers that prefer the CSP directive.
const ContentSecurityPolicy = "base-uri 'self'; object-src 'none'; frame-ancestors 'self'"

// SecurityHeaders sets baseline browser defenses on every response. It stops
// uploaded or proxied content from being sniffed into executable documents and
// from being framed by third parties, and applies the baseline CSP above.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", ContentSecurityPolicy)
		c.Next()
	}
}
