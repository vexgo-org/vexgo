package asset

import (
	"github.com/gin-gonic/gin"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	noGuest := h.mw.Permission(
		model.RoleContributor,
		model.RoleAuthor,
		model.RoleAdmin,
		model.RoleSuperAdmin,
	)

	api.POST("/assets/upload", h.mw.JWTAuth(), noGuest, h.Upload)
	api.POST("/assets/delete/:id", h.mw.JWTAuth(), noGuest, h.Delete)

	api.GET("/assets/:id/original", h.Original)
}
