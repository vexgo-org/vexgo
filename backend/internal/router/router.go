// Package router composes HTTP route registration across all domains.
//
// The REST API is split into two layers:
//
//   - huma-typed operations (the domains that have been migrated to
//     huma: captcha, home, notification, user). They register on a
//     huma.API mounted on /api via humagin; the gin adapter
//     (humagin) preserves the existing gin middleware (auth, rate
//     limit, captcha) by running it before the huma handler.
//
//   - gin-only operations (auth, comment, post, settings, upload,
//     sso, verify-email). They register on the underlying gin
//     group until each domain is migrated. They are documented in
//     the route-surface test (router_test.go) and continue to
//     appear in `r.Routes()`.
//
// When all domains are migrated the huma branch becomes the only
// one. The dual registration is a transitional state.
package router

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/comment"
	"github.com/vexgo-org/vexgo/backend/internal/humaapi"
	"github.com/vexgo-org/vexgo/backend/internal/home"
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

// RegisterHumaRoutes wires only the huma-typed domains onto the
// given huma.API. It is used by the openapi-spec CLI to emit a
// spec without standing up the full runtime dependency tree. The
// server uses RegisterAPIRoutes which also wires gin-only domains.
func RegisterHumaRoutes(r *gin.Engine, api huma.API) {
	api.UseMiddleware(auth.ContextMiddleware)
	captcha.NewHandler(captcha.Deps{}).RegisterRoutes(api)
	home.NewHandler(home.Deps{}).RegisterRoutes(api)
	notification.NewHandler(notification.Deps{}).RegisterRoutes(api)
	user.NewHandler(user.Deps{}).RegisterRoutes(api)
	upload.NewHandler(upload.Deps{}).RegisterRoutes(api)
	comment.NewHandler(comment.Deps{}).RegisterRoutes(api)
	post.NewHandler(post.Deps{}).RegisterRoutes(api)
}

// RegisterAPIRoutes wires every domain. The huma-typed domains
// receive a huma.API mounted on /api. The gin-typed domains continue
// to register on the gin /api group directly until they are
// migrated.
//
// Returns the huma API so callers (openapi-spec CLI, router tests)
// can introspect the registered operations.
func RegisterAPIRoutes(r *gin.Engine, deps Deps) huma.API {
	apiGroup := r.Group("/api")
	jwtAuth := middleware.NewAuth(deps.DB, deps.JWTSecret)
	apiGroup.Use(jwtAuth.OptionalJWTAuth())

	api := humaapi.New(r, humaapi.DefaultConfig("VexGo API", "0.1.0"))
	api.UseMiddleware(auth.ContextMiddleware)

	// MIGRATED: captcha, home, notification, user, upload, comment, post
	captcha.NewHandler(deps.Captcha).RegisterRoutes(api)
	home.NewHandler(deps.Home).RegisterRoutes(api)
	notification.NewHandler(deps.Notification).RegisterRoutes(api)
	user.NewHandler(deps.User).RegisterRoutes(api)
	upload.NewHandler(deps.Upload).RegisterRoutes(api)
	comment.NewHandler(deps.Comment).RegisterRoutes(api)
	post.NewHandler(deps.Post).RegisterRoutes(api)

	// GIN-ONLY (not yet migrated): auth, settings, sso, verify-email
	g := r.Group("/api", jwtAuth.OptionalJWTAuth())
	auth.NewHandler(deps.Auth).RegisterRoutes(g)
	settings.NewHandler(deps.Settings).RegisterRoutes(g)
	sso.NewHandler(deps.SSO).RegisterRoutes(g)

	return api
}
