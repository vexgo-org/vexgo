package upload

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the upload domain routes on the /api group.
// Route paths and middleware chains are identical to the original registration
// in the legacy handler package.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/upload", h.mw.JWTAuth(), h.UploadFile)
	api.POST("/upload/multiple", h.mw.JWTAuth(), h.UploadFiles)
	api.GET("/upload/my", h.mw.JWTAuth(), h.GetMyFiles)
	api.DELETE("/upload/:id", h.mw.JWTAuth(), h.DeleteFile)
}
