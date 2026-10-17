package asset

import (
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/assets/upload", h.mw.JWTAuth(), h.Upload)
	api.POST("/assets/delete/:id", h.mw.JWTAuth(), h.Delete)
}
