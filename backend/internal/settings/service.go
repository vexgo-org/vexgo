// Package settings implements the admin configuration domain: SMTP, AI,
// general site settings and theme configuration.
package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"

	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler. Each error carries
// the exact message of the original handler response it replaces.
var (
	// SMTP config errors.
	ErrSMTPNotConfigured = errors.New("please configure SMTP settings first")
	ErrSMTPDisabled      = errors.New("SMTP is not enabled, please enable and save configuration first")
	ErrSMTPIncomplete    = errors.New("please fill in all SMTP configuration fields")
	ErrSMTPNoRecipient   = errors.New("please fill in test email address first")

	// AI config errors.
	ErrAINotConfigured    = errors.New("please configure AI settings first")
	ErrAIDisabled         = errors.New("AI is not enabled, please enable and save configuration first")
	ErrAIIncomplete       = errors.New("please fill in all AI configuration fields (endpoint, API key, model name)")
	ErrAIIncompleteModels = errors.New("please fill in all AI configuration fields (endpoint, API key)")

	// Theme errors.
	ErrThemeNotFound       = errors.New("theme not found")
	ErrPreviewNotSpecified = errors.New("preview not specified")
	ErrPreviewNotFound     = errors.New("preview image not found")
)

// SecretCipher is the seam for encrypting secrets at rest. It is implemented
// by the secrets package and injected so it can be faked in tests. A nil
// cipher means no key is configured: secrets are stored as plaintext.
type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(stored string) (string, error)
}

// Deps holds the dependencies required by the settings domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Themes    *public.Renderer
	Mailer    *mailer.Service
	Cipher    SecretCipher
}

// Service contains the business logic of the settings domain.
type Service struct {
	repo   Repository
	themes *public.Renderer
	mailer *mailer.Service
	cipher SecretCipher
}

// NewService creates a settings service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), themes: deps.Themes, mailer: deps.Mailer, cipher: deps.Cipher}
}

// encryptSecret encrypts a secret before it is stored. Empty values are passed
// through, and without a configured cipher the plaintext is stored as-is
// (no-key fallback). Errors are wrapped with the setting name so the admin
// can tell which secret failed to save.
func (s *Service) encryptSecret(value, setting string) (string, error) {
	if value == "" || s.cipher == nil {
		return value, nil
	}
	encrypted, err := s.cipher.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("encrypt %s: %w", setting, err)
	}
	return encrypted, nil
}

// decryptSecret decrypts a stored secret for internal use. Values without the
// encrypted marker are passed through unchanged. When decryption fails (e.g.
// the key was rotated) the secret is treated as unset and an error naming the
// affected setting is logged; the admin must re-save the secret.
func (s *Service) decryptSecret(stored, setting string) string {
	if stored == "" || s.cipher == nil {
		return stored
	}
	decrypted, err := s.cipher.Decrypt(stored)
	if err != nil {
		slog.Error("failed to decrypt stored secret, treating it as unset; please re-save it", "setting", setting, "err", err)
		return ""
	}
	return decrypted
}

// SMTPConfigRequest carries the fields accepted when updating the SMTP config.
type SMTPConfigRequest struct {
	Enabled   bool
	Host      string
	Port      int
	Username  string
	Password  string // if empty, don't update password
	FromEmail string
	FromName  string
	TestEmail string
}

// GetSMTPConfig returns the stored SMTP configuration with the password
// masked, or the default configuration when no row exists.
func (s *Service) GetSMTPConfig(ctx context.Context) (model.SMTPConfig, error) {
	config, err := s.repo.GetSMTPConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			return model.SMTPConfig{
				Enabled:   false,
				Host:      "",
				Port:      587,
				Username:  "",
				Password:  "",
				FromEmail: "",
				FromName:  "VexGo",
			}, nil
		}
		return config, err
	}

	// Don't return password field
	config.Password = ""
	return config, nil
}

// UpdateSMTPConfig creates or updates the SMTP configuration and returns it
// with the password masked.
func (s *Service) UpdateSMTPConfig(ctx context.Context, req SMTPConfigRequest) (model.SMTPConfig, error) {
	config, err := s.repo.GetSMTPConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			password, encErr := s.encryptSecret(req.Password, "smtp_config.password")
			if encErr != nil {
				return model.SMTPConfig{}, encErr
			}
			config = model.SMTPConfig{
				Enabled:   req.Enabled,
				Host:      req.Host,
				Port:      req.Port,
				Username:  req.Username,
				Password:  password,
				FromEmail: req.FromEmail,
				FromName:  req.FromName,
				TestEmail: req.TestEmail,
			}
			if err := s.repo.CreateSMTPConfig(ctx, &config); err != nil {
				return config, fmt.Errorf("failed to create SMTP config: %w", err)
			}
		} else {
			return config, fmt.Errorf("failed to get SMTP config: %w", err)
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.Host = req.Host
		config.Port = req.Port
		config.Username = req.Username
		config.FromEmail = req.FromEmail
		config.FromName = req.FromName
		config.TestEmail = req.TestEmail

		// Only update password if new password is provided
		if req.Password != "" {
			password, encErr := s.encryptSecret(req.Password, "smtp_config.password")
			if encErr != nil {
				return config, encErr
			}
			config.Password = password
		}

		if err := s.repo.SaveSMTPConfig(ctx, &config); err != nil {
			return config, fmt.Errorf("failed to update SMTP config: %w", err)
		}
	}

	// Return configuration without password
	config.Password = ""
	return config, nil
}

// TestSMTP sends a test email to the configured test address (or the admin's
// email as fallback) using the stored SMTP configuration.
func (s *Service) TestSMTP(ctx context.Context, adminEmail string) (string, error) {
	config, err := s.repo.GetSMTPConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", ErrSMTPNotConfigured
		}
		return "", err
	}

	// Check if enabled
	if !config.Enabled {
		return "", ErrSMTPDisabled
	}

	// Check required fields
	if config.Host == "" || config.Port == 0 || config.Username == "" || config.Password == "" || config.FromEmail == "" {
		return "", ErrSMTPIncomplete
	}

	// Get recipient email: use configured test email first, otherwise use current admin email
	var recipientEmail string
	if config.TestEmail != "" {
		recipientEmail = config.TestEmail
	} else {
		recipientEmail = adminEmail
	}
	if recipientEmail == "" {
		return "", ErrSMTPNoRecipient
	}

	if err := s.mailer.SendTestSMTPEmail(ctx, recipientEmail, &mailer.TestSMTPEmailTemplateData{
		Name:  config.FromName,
		Host:  config.Host,
		Port:  config.Port,
		Email: config.FromEmail,
		Time:  time.Now().Format("2006-01-02 15:04:05"),
	}); err != nil {
		return "", fmt.Errorf("failed to send test email: %w", err)
	}

	return recipientEmail, nil
}

// GeneralSettingsRequest carries the fields accepted when updating the
// general settings.
type GeneralSettingsRequest struct {
	CaptchaEnabled      bool
	RegistrationEnabled bool
	AllowGuestViewPosts bool
	SiteName            string
	SiteDescription     string
	SiteIcon            string
	ItemsPerPage        int
}

// GetGeneralSettings returns the stored general settings, or the defaults
// when no row exists.
func (s *Service) GetGeneralSettings(ctx context.Context) (model.GeneralSettings, error) {
	config, err := s.repo.GetGeneralSettings(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			return model.GeneralSettings{
				CaptchaEnabled:      false,
				RegistrationEnabled: true,
				AllowGuestViewPosts: true,
				SiteName:            "VexGo",
				SiteDescription:     "",
				SiteIcon:            "",
				ItemsPerPage:        20,
			}, nil
		}
		return config, err
	}
	return config, nil
}

// UpdateGeneralSettings creates or updates the general settings.
func (s *Service) UpdateGeneralSettings(ctx context.Context, req GeneralSettingsRequest) (model.GeneralSettings, error) {
	config, err := s.repo.GetGeneralSettings(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			config = model.GeneralSettings{
				CaptchaEnabled:      req.CaptchaEnabled,
				RegistrationEnabled: req.RegistrationEnabled,
				AllowGuestViewPosts: req.AllowGuestViewPosts,
				SiteName:            req.SiteName,
				SiteDescription:     req.SiteDescription,
				SiteIcon:            req.SiteIcon,
				ItemsPerPage:        req.ItemsPerPage,
			}
			if err := s.repo.CreateGeneralSettings(ctx, &config); err != nil {
				return config, fmt.Errorf("failed to create general settings: %w", err)
			}
		} else {
			return config, fmt.Errorf("failed to get general settings: %w", err)
		}
	} else {
		// Update existing configuration
		config.CaptchaEnabled = req.CaptchaEnabled
		config.RegistrationEnabled = req.RegistrationEnabled
		config.AllowGuestViewPosts = req.AllowGuestViewPosts
		config.SiteName = req.SiteName
		config.SiteDescription = req.SiteDescription
		config.SiteIcon = req.SiteIcon
		config.ItemsPerPage = req.ItemsPerPage

		if err := s.repo.SaveGeneralSettings(ctx, &config); err != nil {
			return config, fmt.Errorf("failed to update general settings: %w", err)
		}
	}

	return config, nil
}

// AIConfigRequest carries the fields accepted when updating the AI config.
type AIConfigRequest struct {
	Enabled     bool
	Provider    string
	ApiEndpoint string
	ApiKey      string // if empty, don't update API key
	ModelName   string
}

// GetAIConfig returns the stored AI configuration with the API key masked,
// or the defaults when no row exists.
func (s *Service) GetAIConfig(ctx context.Context) (model.AIConfig, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration
			return model.AIConfig{
				Enabled:     false,
				Provider:    "openai",
				ApiEndpoint: "",
				ApiKey:      "",
				ModelName:   "gpt-3.5-turbo",
			}, nil
		}
		return config, err
	}

	// Don't return API key field
	config.ApiKey = ""
	return config, nil
}

// UpdateAIConfig creates or updates the AI configuration and returns it with
// the API key masked.
func (s *Service) UpdateAIConfig(ctx context.Context, req AIConfigRequest) (model.AIConfig, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new configuration
			apiKey, encErr := s.encryptSecret(req.ApiKey, "ai_config.api_key")
			if encErr != nil {
				return model.AIConfig{}, encErr
			}
			config = model.AIConfig{
				Enabled:     req.Enabled,
				Provider:    req.Provider,
				ApiEndpoint: req.ApiEndpoint,
				ApiKey:      apiKey,
				ModelName:   req.ModelName,
			}
			if err := s.repo.CreateAIConfig(ctx, &config); err != nil {
				return config, fmt.Errorf("failed to create AI config: %w", err)
			}
		} else {
			return config, fmt.Errorf("failed to get AI config: %w", err)
		}
	} else {
		// Update existing configuration
		config.Enabled = req.Enabled
		config.Provider = req.Provider
		config.ApiEndpoint = req.ApiEndpoint
		config.ModelName = req.ModelName

		// Only update API key if new API key is provided
		if req.ApiKey != "" {
			apiKey, encErr := s.encryptSecret(req.ApiKey, "ai_config.api_key")
			if encErr != nil {
				return config, encErr
			}
			config.ApiKey = apiKey
		}

		if err := s.repo.SaveAIConfig(ctx, &config); err != nil {
			return config, fmt.Errorf("failed to update AI config: %w", err)
		}
	}

	// Return configuration without API key
	config.ApiKey = ""
	return config, nil
}

// AIResult carries the outcome of an AI connection test or model listing.
type AIResult struct {
	Message  string
	Response any
}

// TestAI verifies the AI configuration by calling the chat completions
// endpoint with a test prompt.
func (s *Service) TestAI(ctx context.Context) (*AIResult, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAINotConfigured
		}
		return nil, err
	}

	// The API key is stored encrypted; decrypt it for the outbound call.
	config.ApiKey = s.decryptSecret(config.ApiKey, "ai_config.api_key")

	// Check if enabled
	if !config.Enabled {
		return nil, ErrAIDisabled
	}

	// Check required fields
	if config.ApiEndpoint == "" || config.ApiKey == "" || config.ModelName == "" {
		return nil, ErrAIIncomplete
	}

	// Build base API URL
	// Remove trailing slash to ensure consistent format
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// If user entered full chat endpoint (ending with /chat/completions), extract base part
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if before, ok := strings.CutSuffix(baseURL, "/chat/completions"); ok {
		baseURL = before
	}

	// Ensure baseURL ends with /v1
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	// Build chat completion endpoint
	chatCompletionsURL := baseURL + "/chat/completions"

	// Build model validation endpoint
	modelsURL := baseURL + "/models"

	// Get test prompt: use simple test question
	testPrompt := "Say this is a test"

	// Step 1: Verify model exists (optional but recommended)
	modelExists, modelErr := checkModelExists(modelsURL, config.ApiKey, config.ModelName)
	if modelErr != nil {
		// Model check failed, but continue testing chat completion, endpoint may not support model listing
		slog.Warn("model validation warning (will continue test)", "err", modelErr)
	} else if !modelExists {
		return nil, fmt.Errorf("model '%s' does not exist or is not available, please check model name", config.ModelName)
	}

	// Step 2: Test chat completion functionality
	requestBody := map[string]any{
		"model": config.ModelName,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": testPrompt,
			},
		},
		"max_tokens":  100,
		"temperature": 0.7,
	}

	// Send HTTP request to AI API
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", chatCompletionsURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Check for error field
	if errorMsg, ok := result["error"]; ok {
		return nil, fmt.Errorf("AI API error: %v", errorMsg)
	}

	// Extract AI response content
	var aiResponse string
	if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					aiResponse = content
				}
			}
		}
	}

	return &AIResult{
		Message:  "AI connection test successful!",
		Response: aiResponse,
	}, nil
}

// AIModels returns the list of models available from the configured AI
// endpoint.
func (s *Service) AIModels(ctx context.Context) (*AIResult, error) {
	config, err := s.repo.GetAIConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAINotConfigured
		}
		return nil, err
	}

	// The API key is stored encrypted; decrypt it for the outbound call.
	config.ApiKey = s.decryptSecret(config.ApiKey, "ai_config.api_key")

	// Check if enabled
	if !config.Enabled {
		return nil, ErrAIDisabled
	}

	// Check required fields
	if config.ApiEndpoint == "" || config.ApiKey == "" {
		return nil, ErrAIIncompleteModels
	}

	// Build base API URL (consistent with TestAI function)
	baseURL := strings.TrimSuffix(config.ApiEndpoint, "/")

	// If user entered full chat endpoint (ending with /chat/completions), extract base part
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		baseURL = strings.TrimSuffix(baseURL, "/v1")
		baseURL = baseURL + "/v1"
	} else if before, ok := strings.CutSuffix(baseURL, "/chat/completions"); ok {
		baseURL = before
	}

	// Ensure baseURL ends with /v1
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	// Build models list endpoint
	modelsURL := baseURL + "/models"

	// Send request to fetch models list
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.ApiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch models, status: %d, response: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Check for errors
	if errorMsg, ok := result["error"]; ok {
		return nil, fmt.Errorf("API error: %v", errorMsg)
	}

	// Extract models list
	var models []map[string]any
	if data, ok := result["data"].([]any); ok {
		for _, model := range data {
			if modelMap, ok := model.(map[string]any); ok {
				modelInfo := map[string]any{
					"id":       modelMap["id"],
					"object":   modelMap["object"],
					"created":  modelMap["created"],
					"owned_by": modelMap["owned_by"],
				}
				models = append(models, modelInfo)
			}
		}
	}

	return &AIResult{
		Message:  "Models fetched successfully",
		Response: models,
	}, nil
}

// checkModelExists checks if model exists
func checkModelExists(modelsURL, apiKey, modelName string) (bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to connect to models endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("models endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to parse models response: %v", err)
	}

	// Check models list
	if data, ok := result["data"].([]any); ok {
		for _, model := range data {
			if modelMap, ok := model.(map[string]any); ok {
				if id, ok := modelMap["id"].(string); ok && id == modelName {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// GetThemes returns all available themes.
func (s *Service) GetThemes() []public.ThemeInfo {
	return s.themes.GetAvailableThemes()
}

// ThemePreview resolves the preview image path for a theme.
func (s *Service) ThemePreview(themeID string) (string, error) {
	// Check if theme exists
	if !s.themes.ThemeExists(themeID) {
		return "", ErrThemeNotFound
	}

	// Read theme metadata
	metaPath := filepath.Join(s.themes.DataDir(), public.ThemesDir, themeID, public.ThemeMetaFile)
	content, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read theme metadata: %w", err)
	}

	var themeInfo public.ThemeInfo
	if err := json.Unmarshal(content, &themeInfo); err != nil {
		return "", fmt.Errorf("invalid theme metadata: %w", err)
	}

	// Check if preview is specified
	if themeInfo.Preview == "" {
		return "", ErrPreviewNotSpecified
	}

	// Build preview image path
	previewPath := filepath.Join(s.themes.DataDir(), public.ThemesDir, themeID, themeInfo.Preview)

	// Check if preview image exists
	if _, err := os.Stat(previewPath); os.IsNotExist(err) {
		return "", ErrPreviewNotFound
	}

	return previewPath, nil
}

// GetThemeConfig returns the currently active theme stored in the database,
// falling back to the default theme.
func (s *Service) GetThemeConfig(ctx context.Context) (string, error) {
	config, err := s.repo.GetThemeConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return public.DefaultTheme, nil
		}
		return "", err
	}
	activeTheme := config.ActiveTheme
	if activeTheme == "" {
		activeTheme = public.DefaultTheme
	}
	return activeTheme, nil
}

// UpdateThemeConfig sets the globally active theme in the database.
func (s *Service) UpdateThemeConfig(ctx context.Context, activeTheme string) (string, error) {
	// Validate that the requested theme actually exists
	if !s.themes.ThemeExists(activeTheme) {
		return "", ErrThemeNotFound
	}

	config, err := s.repo.GetThemeConfig(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			config = model.ThemeConfig{ActiveTheme: activeTheme}
			if err := s.repo.CreateThemeConfig(ctx, &config); err != nil {
				return "", fmt.Errorf("failed to save theme config: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to get theme config: %w", err)
		}
	} else {
		config.ActiveTheme = activeTheme
		if err := s.repo.SaveThemeConfig(ctx, &config); err != nil {
			return "", fmt.Errorf("failed to update theme config: %w", err)
		}
	}

	return config.ActiveTheme, nil
}
