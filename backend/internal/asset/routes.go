package asset

import (
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/asset/upload", h.mw.JWTAuth(), h.Upload)
}
