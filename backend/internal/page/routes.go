package page

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the page domain routes on the /api group.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/pages", h.GetPages)
	api.GET("/pages/:slug", h.GetPage)

	admin := h.mw.Permission(model.RoleAdmin, model.RoleSuperAdmin)
	api.POST("/pages", h.mw.JWTAuth(), admin, h.CreatePage)
	api.PUT("/pages/:id", h.mw.JWTAuth(), admin, h.UpdatePage)
	api.DELETE("/pages/:id", h.mw.JWTAuth(), admin, h.DeletePage)
}
