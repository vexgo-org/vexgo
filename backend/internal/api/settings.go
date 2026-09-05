package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// UpdateSMTPConfigRequest is the body of PUT /api/config/smtp.
// The handler treats an empty password as "do not update" and
// treats an empty port as "do not update" to keep the legacy
// partial-update contract.
type UpdateSMTPConfigRequest struct {
	Enabled   *bool  `json:"enabled,omitempty" doc:"Toggle the SMTP transport"`
	Host      string `json:"host,omitempty" doc:"SMTP server hostname"`
	Port      *int   `json:"port,omitempty" doc:"SMTP server port (1-65535)"`
	Username  string `json:"username,omitempty" doc:"SMTP username"`
	Password  string `json:"password,omitempty" doc:"SMTP password (encrypted at rest)"`
	FromEmail string `json:"fromEmail,omitempty" doc:"From address"`
	FromName  string `json:"fromName,omitempty" doc:"Display name"`
	TestEmail string `json:"testEmail,omitempty" doc:"Override recipient for the test endpoint"`
}

// SMTPConfigUpdateResponse is the body of PUT /api/config/smtp.
type SMTPConfigUpdateResponse struct {
	Config model.SMTPConfig `json:"config"`
}

// SMTPTestResponse is the body of POST /api/config/smtp/test.
type SMTPTestResponse struct {
	Message string `json:"message"`
	To      string `json:"to" doc:"Recipient of the test email"`
}

// UpdateAIConfigRequest is the body of PUT /api/config/ai.
type UpdateAIConfigRequest struct {
	Enabled     *bool  `json:"enabled,omitempty" doc:"Toggle AI features"`
	Provider    string `json:"provider,omitempty" doc:"openai|azure|... — selects the LLM client"`
	ApiEndpoint string `json:"apiEndpoint,omitempty" doc:"Custom AI endpoint URL"`
	ApiKey      string `json:"apiKey,omitempty" doc:"AI API key (encrypted at rest); empty keeps the stored value"`
	ModelName   string `json:"modelName,omitempty" doc:"Model name (e.g. gpt-3.5-turbo)"`
}

// AIConfigUpdateResponse is the body of PUT /api/config/ai.
type AIConfigUpdateResponse struct {
	Config model.AIConfig `json:"aiConfig" doc:"The updated configuration"`
}

// AITestResponse is the body of POST /api/config/ai/test.
type AITestResponse struct {
	Message  string `json:"message"`
	Response any    `json:"response,omitempty" doc:"Raw upstream response or summary"`
}

// AIModelsResponse is the body of GET /api/config/ai/models.
type AIModelsResponse struct {
	Message string `json:"message"`
	Models  any    `json:"models" doc:"List of models as returned by the upstream provider"`
}

// UpdateGeneralSettingsRequest is the body of PUT /api/config/general.
type UpdateGeneralSettingsRequest struct {
	CaptchaEnabled      *bool  `json:"captchaEnabled,omitempty"`
	RegistrationEnabled *bool  `json:"registrationEnabled,omitempty"`
	AllowGuestViewPosts *bool  `json:"allowGuestViewPosts,omitempty"`
	SiteName            string `json:"siteName,omitempty"`
	SiteDescription     string `json:"siteDescription,omitempty"`
	SiteIcon            string `json:"siteIcon,omitempty"`
	ItemsPerPage        *int   `json:"itemsPerPage,omitempty"`
}

// GeneralSettingsUpdateResponse is the body of PUT /api/config/general.
type GeneralSettingsUpdateResponse struct {
	Message  string              `json:"message"`
	Settings model.GeneralSettings `json:"generalSettings" doc:"The updated general settings"`
}

// UpdateThemeConfigRequest is the body of PUT /api/config/theme.
type UpdateThemeConfigRequest struct {
	ActiveTheme string `json:"activeTheme" required:"" doc:"Theme ID to activate"`
}

// ThemeConfigResponse is the body of GET and PUT /api/config/theme.
type ThemeConfigResponse struct {
	ActiveTheme string `json:"activeTheme"`
	Message     string `json:"message,omitempty" doc:"Set on PUT responses"`
}

// ThemesResponse is the body of GET /api/themes. The element type
// is the public package's ThemeInfo (re-exposed as `any` here so
// the spec does not need to know about the internal package).
type ThemesResponse struct {
	Themes []any `json:"themes"`
}

// ThemePreviewResponse is the body of GET /api/theme/{id}/preview.
// The legacy handler served a binary file via c.File; the huma port
// inlines the bytes so the response stays JSON.
type ThemePreviewResponse struct {
	URL      string `json:"url" doc:"Path to the preview file on disk"`
	MimeType string `json:"mimeType"`
	Data     []byte `json:"data,omitempty" doc:"Inline preview bytes"`
}

// UploadThemeResponse is the body of POST /api/themes/upload.
type UploadThemeResponse struct {
	Message string `json:"message"`
	ThemeID string `json:"themeId" doc:"ID of the installed theme"`
}
