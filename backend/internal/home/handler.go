package home

import (
	"net/http"

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

// GetStats godoc
//
//	@Summary		Aggregate site statistics
//	@Description	Returns the public post, user, comment, category and tag
//	@Description	counters. Anonymous callers see a reduced view that omits
//	@Description	pending posts; authenticated callers see the full set.
//	@Tags			stats
//	@Produce		json
//	@Success		200	{object}	StatsResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	// Get current user role
	u, _ := middleware.CurrentUser(c)

	stats := h.svc.Stats(c.Request.Context(), u.Role)

	c.JSON(http.StatusOK, StatsResponse{
		Stats: StatsAggregate(stats),
	})
}
