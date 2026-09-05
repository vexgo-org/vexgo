package home

import (
	"net/http"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"

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

	c.JSON(http.StatusOK, api.StatsResponse{
		Stats: api.Stats{
			Posts:      stats.Posts,
			Users:      stats.Users,
			Comments:   stats.Comments,
			Categories: stats.Categories,
			Tags:       stats.Tags,
		},
	})
}
