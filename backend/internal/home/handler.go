package home

import (
	"net/http"

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
	var userRole string
	if userContext, exists := c.Get("user"); exists {
		if userMap, ok := userContext.(map[string]any); ok {
			if role, ok := userMap["role"].(string); ok {
				userRole = role
			}
		}
	}

	stats := h.svc.Stats(c.Request.Context(), userRole)

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
