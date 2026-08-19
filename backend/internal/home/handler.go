package home

import (
	"net/http"

	"vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handler exposes the home domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a home HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// GetStats returns aggregate site statistics.
func (h *Handler) GetStats(c *gin.Context) {
	// Get current user role
	u, _ := middleware.CurrentUser(c)

	stats := h.svc.Stats(c.Request.Context(), u.Role)

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"posts":      stats.Posts,
			"users":      stats.Users,
			"comments":   stats.Comments,
			"categories": stats.Categories,
			"tags":       stats.Tags,
		},
	})
}
