// Package router composes HTTP route registration across all domains.
//
// The REST API is described by a single huma registry; non-REST routes
// (static files, SSR, the SSO HTML callback) keep registering
// directly with gin. huma is mounted via the gin adapter (`humagin`)
// so the existing gin middleware (auth, rate limit, captcha) keeps
// running unchanged — humagin dispatches gin middleware first, then
// hands control to the huma handler.
package router

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/comment"
	"github.com/vexgo-org/vexgo/backend/internal/home"
	"github.com/vexgo-org/vexgo/backend/internal/humaapi"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/notification"
	"github.com/vexgo-org/vexgo/backend/internal/post"
	"github.com/vexgo-org/vexgo/backend/internal/settings"
	"github.com/vexgo-org/vexgo/backend/internal/sso"
	"github.com/vexgo-org/vexgo/backend/internal/upload"
	"github.com/vexgo-org/vexgo/backend/internal/user"
)

// Deps aggregates the dependencies of every domain package.
type Deps struct {
	DB           *gorm.DB
	JWTSecret    []byte
	Notification notification.Deps
	Comment      comment.Deps
	Post         post.Deps
	Upload       upload.Deps
	User         user.Deps
	Captcha      captcha.Deps
	Auth         auth.Deps
	SSO          sso.Deps
	Home         home.Deps
	Settings     settings.Deps
}

// RegisterAPIRoutes builds the gin engine, mounts huma on the /api
// sub-group, attaches the gin middleware (OptionalJWTAuth on the
// whole /api group) and the huma-side auth context bridge, then
// hands the huma.API to each domain's RegisterRoutes. The returned
// huma.API is exposed so callers (router tests, the openapi-spec
// CLI) can introspect the registered operations.
func RegisterAPIRoutes(r *gin.Engine, deps Deps) huma.API {
	// Build huma first so the auth context bridge can be applied
	// before any domain registers its operations.
	api := humaapi.New(r, humaapi.DefaultConfig("VexGo API", "0.1.0"))

	// Optional auth on the whole /api group: gin middleware
	// populates the gin context with the user (when a Bearer token
	// is present). The huma-side ContextMiddleware below copies
	// that into the request context so handlers can read it via
	// auth.UserFromContext.
	apiGroup := r.Group("/api")
	apiGroup.Use(middleware.NewAuth(deps.DB, deps.JWTSecret).OptionalJWTAuth())
	// Captcha endpoints get an extra per-IP rate limit on top of
	// the optional JWT.
	captchaLimiter := middleware.NewRateLimiter("captcha", deps.Captcha.RateLimitPerMinute, deps.Captcha.RateLimit)
	// Auth credential endpoints (login, register, password reset,
	// resend) get their own rate limit too. The auth domain reads
	// deps.Auth.RateLimitPerMinute to apply it.

	// Huma-side context bridge: copies the user from the gin
	// context into the request context.
	api.UseMiddleware(auth.ContextMiddleware)

	// Register every domain's huma operations.
	notification.NewHandler(deps.Notification).RegisterRoutes(api)
	comment.NewHandler(deps.Comment).RegisterRoutes(api)
	post.NewHandler(deps.Post).RegisterRoutes(api)
	upload.NewHandler(deps.Upload).RegisterRoutes(api)
	user.NewHandler(deps.User).RegisterRoutes(api)
	// Captcha domain: attach the captcha rate limit to a
	// sub-group of the gin /api so it only applies to captcha
	// routes. We do this by creating a huma API view on the
	// sub-group — humagin.NewWithGroup lets the domain register
	// under /captcha while still inheriting the rate limiter.
	if captchaLimiter != nil {
		captchaGroup := apiGroup.Group("/captcha", captchaLimiter)
		captchaAPI := humagin.NewWithGroup(r, captchaGroup, humaapi.DefaultConfig("VexGo API", "0.1.0").OpenAPI)
		// Inherit the auth context middleware from the parent API.
		captchaAPI.UseMiddleware(auth.ContextMiddleware)
		captcha.NewHandler(deps.Captcha).RegisterRoutes(captchaAPI)
	} else {
		captcha.NewHandler(deps.Captcha).RegisterRoutes(api)
	}
	auth.NewHandler(deps.Auth).RegisterRoutes(api)
	home.NewHandler(deps.Home).RegisterRoutes(api)
	settings.NewHandler(deps.Settings).RegisterRoutes(api)
	// SSO and upload keep gin registration for now — the sso
	// callback writes HTML and the upload domain serves multipart
	// forms, both of which are awkward to express as huma I/O
	// types. They will be migrated as a follow-up.
	sso.NewHandler(deps.SSO).RegisterRoutes(apiGroup)
	upload.NewHandler(deps.Upload).RegisterRoutes(apiGroup)

	return api
}
