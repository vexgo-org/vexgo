// Package router composes HTTP route registration across all domains.
package router

import (
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/comment"
	"github.com/vexgo-org/vexgo/backend/internal/home"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/notification"
	"github.com/vexgo-org/vexgo/backend/internal/post"
	"github.com/vexgo-org/vexgo/backend/internal/settings"
	"github.com/vexgo-org/vexgo/backend/internal/sso"
	"github.com/vexgo-org/vexgo/backend/internal/upload"
	"github.com/vexgo-org/vexgo/backend/internal/user"
	"github.com/vexgo-org/vexgo/backend/internal/verification"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	Verification verification.Deps
	Auth         auth.Deps
	SSO          sso.Deps
	Home         home.Deps
	Settings     settings.Deps
}

// RegisterAPIRoutes registers all routes under /api.
func RegisterAPIRoutes(r *gin.Engine, deps Deps) {
	api := r.Group("/api")
	api.Use(middleware.NewAuth(deps.DB, deps.JWTSecret).OptionalJWTAuth())

	notification.NewHandler(deps.Notification).RegisterRoutes(api)
	comment.NewHandler(deps.Comment).RegisterRoutes(api)
	post.NewHandler(deps.Post).RegisterRoutes(api)
	upload.NewHandler(deps.Upload).RegisterRoutes(api)
	user.NewHandler(deps.User).RegisterRoutes(api)
	verification.NewHandler(deps.Verification).RegisterRoutes(api)
	auth.NewHandler(deps.Auth).RegisterRoutes(api)
	sso.NewHandler(deps.SSO).RegisterRoutes(api)
	home.NewHandler(deps.Home).RegisterRoutes(api)
	settings.NewHandler(deps.Settings).RegisterRoutes(api)
}
