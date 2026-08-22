package user

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the user domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	admin := h.mw.Permission(model.RoleAdmin, model.RoleSuperAdmin)

	api.GET("/users", h.mw.JWTAuth(), admin, h.GetUserList)
	api.PUT("/users/:id/role", h.mw.JWTAuth(), admin, h.UpdateUserRole)
	api.DELETE("/users/:id", h.mw.JWTAuth(), admin, h.DeleteUser)

	// Creator application routes
	api.POST("/users/apply-creator", h.mw.JWTAuth(), h.ApplyForCreator)
	api.GET("/users/creator-applications", h.mw.JWTAuth(), admin, h.GetCreatorApplications)
	api.PUT("/users/creator-applications/:id/review", h.mw.JWTAuth(), admin, h.ReviewCreatorApplication)
}
