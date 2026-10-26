package public

import (
	"context"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// SiteIconURL returns the URL of site icon.
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

// siteIconURL is a helper to compose the original URL
// of the site icon.
func siteIconURL(settings *model.GeneralSettings) string {
	if settings == nil ||
		settings.SiteIcon == nil {
		return ""
	}
	return asset.OriginalURL(settings.SiteIcon.PublicID)
}
