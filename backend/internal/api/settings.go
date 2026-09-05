package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// SMTPConfigUpdateResponse is the body of PUT /api/config/smtp.
type SMTPConfigUpdateResponse struct {
	Message    string           `json:"message"`
	SMTPConfig model.SMTPConfig `json:"smtpConfig"`
}

// SMTPTestResponse is the body of POST /api/config/smtp/test.
type SMTPTestResponse struct {
	Message string `json:"message"`
	To      string `json:"to"`
}

// GeneralSettingsUpdateResponse is the body of PUT /api/config/general.
type GeneralSettingsUpdateResponse struct {
	Message         string                `json:"message"`
	GeneralSettings model.GeneralSettings `json:"generalSettings"`
}

// AIConfigUpdateResponse is the body of PUT /api/config/ai.
type AIConfigUpdateResponse struct {
	Message  string         `json:"message"`
	AIConfig model.AIConfig `json:"aiConfig"`
}

// Theme is the metadata of an installed theme, derived from its
// vexgo-theme.json manifest.
type Theme struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// ThemesResponse is the body of GET /api/themes.
type ThemesResponse struct {
	Themes []Theme `json:"themes"`
}

// ActiveThemeResponse is the body of GET /api/themes/config.
type ActiveThemeResponse struct {
	ActiveTheme string `json:"activeTheme"`
}

// ThemeUpdateResponse is the body of PUT /api/themes/config.
type ThemeUpdateResponse struct {
	Message     string `json:"message"`
	ActiveTheme string `json:"activeTheme"`
}

// UpdateSMTPConfigRequest is the body of PUT /api/config/smtp. An empty
// password keeps the stored value.
type UpdateSMTPConfigRequest struct {
	Enabled   bool   `json:"enabled"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"` // if empty, don't update password
	FromEmail string `json:"fromEmail"`
	FromName  string `json:"fromName"`
	TestEmail string `json:"testEmail"` // test email recipient
}

// UpdateGeneralSettingsRequest is the body of PUT /api/config/general.
type UpdateGeneralSettingsRequest struct {
	CaptchaEnabled      bool   `json:"captchaEnabled"`
	RegistrationEnabled bool   `json:"registrationEnabled"`
	AllowGuestViewPosts bool   `json:"allowGuestViewPosts"`
	SiteName            string `json:"siteName"`
	SiteDescription     string `json:"siteDescription"`
	SiteIcon            string `json:"siteIcon"`
	ItemsPerPage        int    `json:"itemsPerPage"`
}

// UpdateAIConfigRequest is the body of PUT /api/config/ai. An empty API key
// keeps the stored value.
type UpdateAIConfigRequest struct {
	Enabled     bool   `json:"enabled"`
	Provider    string `json:"provider"`
	ApiEndpoint string `json:"apiEndpoint"`
	ApiKey      string `json:"apiKey"` // if empty, don't update API key
	ModelName   string `json:"modelName"`
}

// UpdateThemeConfigRequest is the body of PUT /api/config/theme.
type UpdateThemeConfigRequest struct {
	ActiveTheme string `json:"activeTheme" binding:"required"`
}
