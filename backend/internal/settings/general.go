package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"
	"gorm.io/gorm"
)

// GeneralSettingsRequest carries the fields accepted when updating the
// general settings.
type GeneralSettingsRequest struct {
	CaptchaEnabled      bool
	RegistrationEnabled bool
	AllowGuestViewPosts bool
	SiteName            string
	SiteDescription     string
	SiteIconPublicID    *uuid.UUID
	ItemsPerPage        int
	SiteLanguage        string
}

// GetGeneralSettings returns the stored general settings, or the defaults
// when no row exists.
func (s *Service) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	config, err := s.repo.GetGeneralSettings(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Return default configuration
		return model.GeneralSettings{
			CaptchaEnabled:      false,
			RegistrationEnabled: true,
			AllowGuestViewPosts: true,
			SiteName:            "VexGo",
			SiteDescription:     "",
			SiteIconID:          nil,
			SiteIcon:            nil,
			ItemsPerPage:        20,
			SiteLanguage:        public.DefaultLanguage,
		}, nil
	}
	return config, err
}

// UpdateGeneralSettings creates or updates the general settings.
func (s *Service) UpdateGeneralSettings(ctx context.Context, req GeneralSettingsRequest) (model.GeneralSettings, error) {
	var siteIconID *uint = nil
	if req.SiteIconPublicID != nil {
		id, err := s.repo.FindIconIDByPublicID(ctx, *req.SiteIconPublicID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.GeneralSettings{}, ErrInvalidSiteIcon
			}
			return model.GeneralSettings{}, fmt.Errorf("find site icon by public ID: %w", err)
		}

		siteIconID = &id
	}

	config, err := s.repo.GetGeneralSettings(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new configuration
		config = model.GeneralSettings{
			CaptchaEnabled:      req.CaptchaEnabled,
			RegistrationEnabled: req.RegistrationEnabled,
			AllowGuestViewPosts: req.AllowGuestViewPosts,
			SiteName:            req.SiteName,
			SiteDescription:     req.SiteDescription,
			SiteIconID:          siteIconID,
			ItemsPerPage:        req.ItemsPerPage,
			SiteLanguage:        normalizeSiteLanguage(req.SiteLanguage),
		}
		if err := s.repo.CreateGeneralSettings(ctx, &config); err != nil {
			return config, fmt.Errorf("failed to create general settings: %w", err)
		}
		return s.repo.GetGeneralSettings(ctx)
	}
	if err != nil {
		return config, fmt.Errorf("failed to get general settings: %w", err)
	}

	// Update existing configuration
	config.CaptchaEnabled = req.CaptchaEnabled
	config.RegistrationEnabled = req.RegistrationEnabled
	config.AllowGuestViewPosts = req.AllowGuestViewPosts
	config.SiteName = req.SiteName
	config.SiteDescription = req.SiteDescription
	config.SiteIconID = siteIconID
	config.ItemsPerPage = req.ItemsPerPage
	config.SiteLanguage = normalizeSiteLanguage(req.SiteLanguage)

	if err := s.repo.SaveGeneralSettings(ctx, &config); err != nil {
		return config, fmt.Errorf("failed to update general settings: %w", err)
	}

	return s.repo.GetGeneralSettings(ctx)
}

// toGeneralSettingResponse transforms `model.GeneralSettings` to `GeneralSettingsResponse`.
func toGeneralSettingResponse(config *model.GeneralSettings) *GeneralSettingsResponse {
	resp := &GeneralSettingsResponse{
		CaptchaEnabled:      config.CaptchaEnabled,
		RegistrationEnabled: config.RegistrationEnabled,
		AllowGuestViewPosts: config.AllowGuestViewPosts,
		SiteName:            config.SiteName,
		SiteDescription:     config.SiteDescription,
		ItemsPerPage:        config.ItemsPerPage,
		SiteLanguage:        normalizeSiteLanguage(config.SiteLanguage),
	}

	if config.SiteIcon != nil {
		resp.SiteIconPublicID = &config.SiteIcon.PublicID
		resp.SiteIconURL = asset.OriginalURL(config.SiteIcon.PublicID)
	}

	return resp
}

// normalizeSiteLanguage maps admin input to the stored language code,
// defaulting to en for empty/invalid values.
func normalizeSiteLanguage(lang string) string {
	if normalized := public.NormalizeLanguage(lang); normalized != "" {
		return normalized
	}
	return public.DefaultLanguage
}
