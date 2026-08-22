package settings

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the settings domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	admin := h.mw.Permission(model.RoleAdmin, model.RoleSuperAdmin)

	api.GET("/themes", h.GetThemes)
	api.GET("/theme/:id/preview", h.GetThemePreview)

	api.GET("/config/smtp", h.mw.JWTAuth(), admin, h.GetSMTPConfig)
	api.PUT("/config/smtp", h.mw.JWTAuth(), admin, h.UpdateSMTPConfig)
	api.POST("/config/smtp/test", h.mw.JWTAuth(), admin, h.TestSMTP)

	api.GET("/config/ai", h.mw.JWTAuth(), admin, h.GetAIConfig)
	api.PUT("/config/ai", h.mw.JWTAuth(), admin, h.UpdateAIConfig)
	api.POST("/config/ai/test", h.mw.JWTAuth(), admin, h.TestAI)
	api.GET("/config/ai/models", h.mw.JWTAuth(), admin, h.GetAIModels)

	api.GET("/config/general", h.GetGeneralSettings)
	api.PUT("/config/general", h.mw.JWTAuth(), admin, h.UpdateGeneralSettings)

	api.GET("/config/theme", h.GetThemeConfig)
	api.PUT("/config/theme", h.mw.JWTAuth(), admin, h.UpdateThemeConfig)

	// Theme upload endpoint
	api.POST("/themes/upload", h.mw.JWTAuth(), admin, h.UploadTheme)
}
