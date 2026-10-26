package public

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// SiteIconURL returns the configured site icon's asset URL, or an empty string
// when no icon is configured or it cannot be loaded.
func (s *Service) SiteIconURL(ctx context.Context) string {
	if s.repo == nil {
		return ""
	}

	settings, err := s.repo.GetGeneralSettings(ctx)
	if err != nil {
		return ""
	}

	return siteIconURL(&settings)
}

// siteIconURL composes the configured icon's original asset URL.
func siteIconURL(settings *model.GeneralSettings) string {
	if settings == nil ||
		settings.SiteIcon == nil {
		return ""
	}
	return asset.OriginalURL(settings.SiteIcon.PublicID)
}
