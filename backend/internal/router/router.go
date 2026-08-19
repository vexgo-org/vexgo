// Package router composes HTTP route registration across all domains.
package router

import (
	"vexgo/backend/internal/auth"
	"vexgo/backend/internal/comment"
	"vexgo/backend/internal/home"
	"vexgo/backend/internal/message"
	"vexgo/backend/internal/middleware"
	"vexgo/backend/internal/post"
	"vexgo/backend/internal/settings"
	"vexgo/backend/internal/sso"
	"vexgo/backend/internal/upload"
	"vexgo/backend/internal/user"
	"vexgo/backend/internal/verification"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Deps aggregates the dependencies of every domain package.
type Deps struct {
	DB           *gorm.DB
	JWTSecret    []byte
	Message      message.Deps
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

	message.NewHandler(deps.Message).RegisterRoutes(api)
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
