// Package home exposes site-wide counters (the "home" page stats).
package home

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
)

// Handler exposes the home domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a home HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// getStatsOutput wraps the stats response. huma renders Body as JSON.
type getStatsOutput struct {
	Body api.StatsResponse
}

// RegisterRoutes registers the home / stats domain operations on the
// given huma.API.
func (h *Handler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-stats",
		Method:      http.MethodGet,
		Path:        "/stats",
		Summary:     "Get site-wide stats",
		Tags:        []string{"stats"},
	}, h.GetStats)
}

// GetStats returns aggregate site statistics.
func (h *Handler) GetStats(ctx context.Context, _ *struct{}) (*getStatsOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	stats := h.svc.Stats(ctx, u.Role)
	return &getStatsOutput{
		Body: api.StatsResponse{
			Stats: api.Stats{
				Posts:      stats.Posts,
				Users:      stats.Users,
				Comments:   stats.Comments,
				Categories: stats.Categories,
				Tags:       stats.Tags,
			},
		},
	}, nil
}
