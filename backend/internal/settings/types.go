// Wire types for the settings domain. Swag reads the JSON
// tags to populate the OpenAPI spec; orval turns the
// generated schemas into TypeScript interfaces.
package settings

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"
)

// SMTPConfigResponse is the body of GET and PUT
// /api/config/smtp. It is the same model.SMTPConfig the
// service layer returns, with the password masked.
type SMTPConfigResponse struct {
	configEnvelope
	Enabled   bool   `json:"enabled" example:"true"`
	Host      string `json:"host" example:"smtp.example.com"`
	Port      int    `json:"port" example:"587"`
	Username  string `json:"username" example:"noreply@example.com"`
	FromEmail string `json:"fromEmail" example:"noreply@example.com"`
	FromName  string `json:"fromName" example:"VexGo"`
	TestEmail string `json:"testEmail" example:"admin@example.com"`
}

// SMTPConfigUpdateRequest is the body of PUT /api/config/smtp.
type SMTPConfigUpdateRequest struct {
	Enabled   bool   `json:"enabled" example:"true"`
	Host      string `json:"host" example:"smtp.example.com"`
	Port      int    `json:"port" example:"587"`
	Username  string `json:"username" example:"noreply@example.com"`
	Password  string `json:"password" example:"app-password-or-secret"`
	FromEmail string `json:"fromEmail" example:"noreply@example.com"`
	FromName  string `json:"fromName" example:"VexGo"`
	TestEmail string `json:"testEmail" example:"admin@example.com"`
}

// TestSMTPResponse is the body of POST /api/config/smtp/test.
type TestSMTPResponse struct {
	Message string `json:"message" example:"Test email has been sent to your inbox"`
	To      string `json:"to" example:"admin@example.com"`
}

// GeneralSettingsResponse is the body of GET
// /api/config/general. Wraps the model so the schema shows
// up under settings.GeneralSettingsResponse rather than
// model.GeneralSettings (which is the GORM row).
type GeneralSettingsResponse struct {
	configEnvelope
	CaptchaEnabled      bool   `json:"captchaEnabled" example:"true"`
	RegistrationEnabled bool   `json:"registrationEnabled" example:"true"`
	AllowGuestViewPosts bool   `json:"allowGuestViewPosts" example:"false"`
	SiteName            string `json:"siteName" example:"My VexGo Site"`
	SiteDescription     string `json:"siteDescription"`
	SiteIcon            string `json:"siteIcon" example:"https://example.com/icon.png"`
	ItemsPerPage        int    `json:"itemsPerPage" example:"20"`
}

// GeneralSettingsUpdateRequest is the body of PUT
// /api/config/general.
type GeneralSettingsUpdateRequest struct {
	CaptchaEnabled      bool   `json:"captchaEnabled" example:"true"`
	RegistrationEnabled bool   `json:"registrationEnabled" example:"true"`
	AllowGuestViewPosts bool   `json:"allowGuestViewPosts" example:"false"`
	SiteName            string `json:"siteName" example:"My VexGo Site"`
	SiteDescription     string `json:"siteDescription"`
	SiteIcon            string `json:"siteIcon" example:"https://example.com/icon.png"`
	ItemsPerPage        int    `json:"itemsPerPage" example:"20"`
}

// GeneralSettingsUpdateResponse is the body of PUT
// /api/config/general on success.
type GeneralSettingsUpdateResponse struct {
	Message         string               `json:"message" example:"General settings updated successfully"`
	GeneralSettings model.GeneralSettings `json:"generalSettings"`
}

// AIConfigResponse is the body of GET /api/config/ai. The
// API key is masked in this shape.
type AIConfigResponse struct {
	configEnvelope
	Enabled     bool   `json:"enabled" example:"true"`
	Provider    string `json:"provider" example:"openai"`
	ApiEndpoint string `json:"apiEndpoint" example:"https://api.openai.com/v1"`
	ModelName   string `json:"modelName" example:"gpt-4o-mini"`
}

// AIConfigUpdateRequestBody is the body of PUT /api/config/ai.
type AIConfigUpdateRequestBody struct {
	Enabled     bool   `json:"enabled" example:"true"`
	Provider    string `json:"provider" example:"openai"`
	ApiEndpoint string `json:"apiEndpoint" example:"https://api.openai.com/v1"`
	ApiKey      string `json:"apiKey" example:"sk-..."`
	ModelName   string `json:"modelName" example:"gpt-4o-mini"`
}

// AIConfigUpdateResponse is the body of PUT /api/config/ai
// on success.
type AIConfigUpdateResponse struct {
	Message  string         `json:"message" example:"AI config updated successfully"`
	AIConfig model.AIConfig `json:"aiConfig"`
}

// AITestResponse is the body of POST /api/config/ai/test.
type AITestResponse struct {
	Message  string `json:"message" example:"AI test successful"`
	Response string `json:"response"`
}

// AIModelsResponse is the body of GET /api/config/ai/models.
// The `models` field is a free-form list of model ids; the
// provider returns whatever its API gives us.
type AIModelsResponse struct {
	Message string   `json:"message"`
	Models  []string `json:"models" example:"gpt-4o,gpt-4o-mini,gpt-3.5-turbo"`
}

// ThemesListResponse is the body of GET /api/config/themes.
type ThemesListResponse struct {
	Themes []public.ThemeInfo `json:"themes"`
}

// ThemeConfigResponse is the body of GET and PUT
// /api/config/theme. The `activeTheme` field is the slug of
// the currently active theme.
type ThemeConfigResponse struct {
	ActiveTheme string `json:"activeTheme" example:"vexgo-default"`
}

// ThemeConfigUpdateRequest is the body of PUT
// /api/config/theme.
type ThemeConfigUpdateRequest struct {
	ActiveTheme string `json:"activeTheme" binding:"required" example:"vexgo-default"`
}

// ThemeConfigUpdateResponse is the body of PUT
// /api/config/theme on success.
type ThemeConfigUpdateResponse struct {
	Message     string `json:"message" example:"Theme updated successfully"`
	ActiveTheme string `json:"activeTheme" example:"vexgo-default"`
}

// ThemeUploadResponse is the body of POST /api/config/theme/upload.
type ThemeUploadResponse struct {
	Message string `json:"message" example:"Theme uploaded successfully"`
}

// configEnvelope is an internal embedding slot that pulls
// model.X's json tags into this struct so swag can describe
// it under the settings package. It is empty by design.
type configEnvelope struct{}
