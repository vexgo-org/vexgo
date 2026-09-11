package settings

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/public"
)

// themeIDPattern is the allowlist for theme directory names coming from
// uploaded theme metadata: letters, digits, underscore and dash only, so the
// value can never traverse out of the themes directory.
var themeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Handler exposes the settings domain over HTTP.
type Handler struct {
	svc    *Service
	themes *public.Renderer
	mw     *middleware.Auth
}

// NewHandler creates a settings HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret), themes: deps.Themes}
}

// GetSMTPConfig godoc
//
//	@Summary		Get SMTP configuration
//	@Description	Returns the stored SMTP configuration with the
//	@Description	password masked.
//	@Tags			config
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	SMTPConfigResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/smtp [get]
func (h *Handler) GetSMTPConfig(c *gin.Context) {
	config, err := h.svc.GetSMTPConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get SMTP config"})
		return
	}
	c.JSON(http.StatusOK, SMTPConfigResponse{
		Enabled:   config.Enabled,
		Host:      config.Host,
		Port:      config.Port,
		Username:  config.Username,
		FromEmail: config.FromEmail,
		FromName:  config.FromName,
		TestEmail: config.TestEmail,
	})
}

// UpdateSMTPConfig godoc
//
//	@Summary		Update SMTP configuration
//	@Description	Updates the SMTP server, credentials, and sender
//	@Description	identity. An empty password field leaves the
//	@Description	existing password unchanged.
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		SMTPConfigUpdateRequest	true	"new config"
//	@Success		200		{object}	SMTPConfigResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/config/smtp [put]
func (h *Handler) UpdateSMTPConfig(c *gin.Context) {
	var req SMTPConfigUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateSMTPConfig(c.Request.Context(), SMTPConfigRequest(req))
	if err != nil {
		slog.Error("failed to update SMTP configuration", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update SMTP configuration"})
		return
	}

	c.JSON(http.StatusOK, SMTPConfigResponse{
		Enabled:   config.Enabled,
		Host:      config.Host,
		Port:      config.Port,
		Username:  config.Username,
		FromEmail: config.FromEmail,
		FromName:  config.FromName,
		TestEmail: config.TestEmail,
	})
}

// TestSMTP godoc
//
//	@Summary		Send a test email
//	@Description	Dispatches a small test message to the configured
//	@Description	test email (or to the current admin's address if
//	@Description	none is configured).
//	@Tags			config
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	TestSMTPResponse
//	@Failure		400	{object}	api.ErrorResponse	"SMTP disabled / incomplete / no recipient"
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		404	{object}	api.ErrorResponse	"SMTP not configured"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/smtp/test [post]
func (h *Handler) TestSMTP(c *gin.Context) {
	// Get current admin user email (from JWT token)
	userContext, exists := c.Get(middleware.CtxUserKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Get recipient email: use configured test email first, otherwise use current admin email
	var adminEmail string
	if userMap, ok := userContext.(map[string]any); ok {
		adminEmail, _ = userMap["email"].(string)
	}

	recipientEmail, err := h.svc.TestSMTP(c.Request.Context(), adminEmail)
	if err != nil {
		switch {
		case errors.Is(err, ErrSMTPNotConfigured):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrSMTPDisabled), errors.Is(err, ErrSMTPIncomplete), errors.Is(err, ErrSMTPNoRecipient):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			slog.Error("failed to send test email", "err", err)
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to send test email"})
		}
		return
	}

	c.JSON(http.StatusOK, TestSMTPResponse{
		Message: "Test email has been sent to your inbox",
		To:      recipientEmail,
	})
}

// GetGeneralSettings godoc
//
//	@Summary		Get general site settings
//	@Description	Returns captcha, registration, guest view, site
//	@Description	identity, and pagination defaults.
//	@Tags			config
//	@Produce		json
//	@Success		200	{object}	GeneralSettingsResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/general [get]
func (h *Handler) GetGeneralSettings(c *gin.Context) {
	config, err := h.svc.GetGeneralSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get general settings"})
		return
	}
	c.JSON(http.StatusOK, GeneralSettingsResponse{
		CaptchaEnabled:      config.CaptchaEnabled,
		RegistrationEnabled: config.RegistrationEnabled,
		AllowGuestViewPosts: config.AllowGuestViewPosts,
		SiteName:            config.SiteName,
		SiteDescription:     config.SiteDescription,
		SiteIcon:            config.SiteIcon,
		ItemsPerPage:        config.ItemsPerPage,
		SiteLanguage:        normalizeSiteLanguage(config.SiteLanguage),
	})
}

// UpdateGeneralSettings godoc
//
//	@Summary	Update general site settings
//	@Tags		config
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		GeneralSettingsUpdateRequest	true	"new general settings"
//	@Success	200		{object}	GeneralSettingsUpdateResponse
//	@Failure	400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/config/general [put]
func (h *Handler) UpdateGeneralSettings(c *gin.Context) {
	var req GeneralSettingsUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateGeneralSettings(c.Request.Context(), GeneralSettingsRequest(req))
	if err != nil {
		slog.Error("failed to update general settings", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update general settings"})
		return
	}

	c.JSON(http.StatusOK, GeneralSettingsUpdateResponse{
		Message:         "General settings updated successfully",
		GeneralSettings: config,
	})
}

// GetAIConfig godoc
//
//	@Summary		Get AI configuration
//	@Description	Returns provider, endpoint and model with the API
//	@Description	key masked.
//	@Tags			config
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	AIConfigResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/ai [get]
func (h *Handler) GetAIConfig(c *gin.Context) {
	config, err := h.svc.GetAIConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get AI config"})
		return
	}
	c.JSON(http.StatusOK, AIConfigResponse{
		Enabled:     config.Enabled,
		Provider:    config.Provider,
		ApiEndpoint: config.ApiEndpoint,
		ModelName:   config.ModelName,
	})
}

// UpdateAIConfig godoc
//
//	@Summary		Update AI configuration
//	@Description	Updates the AI provider, endpoint, and model. An
//	@Description	empty apiKey field leaves the existing key
//	@Description	unchanged.
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		AIConfigUpdateRequestBody	true	"new AI config"
//	@Success		200		{object}	AIConfigUpdateResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid payload"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/config/ai [put]
func (h *Handler) UpdateAIConfig(c *gin.Context) {
	var req AIConfigUpdateRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateAIConfig(c.Request.Context(), AIConfigRequest(req))
	if err != nil {
		slog.Error("failed to update AI configuration", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update AI configuration"})
		return
	}

	c.JSON(http.StatusOK, AIConfigUpdateResponse{
		Message:  "AI config updated successfully",
		AIConfig: config,
	})
}

// TestAI godoc
//
//	@Summary		Test the AI provider
//	@Description	Issues a small test prompt against the configured
//	@Description	provider to confirm credentials and endpoint are
//	@Description	wired up correctly.
//	@Tags			config
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	AITestResponse
//	@Failure		400	{object}	api.ErrorResponse	"AI disabled / incomplete config"
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		404	{object}	api.ErrorResponse	"AI not configured"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/ai/test [post]
func (h *Handler) TestAI(c *gin.Context) {
	// Get current admin user information (from JWT token)
	if _, exists := c.Get(middleware.CtxUserKey); !exists {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Unauthorized"})
		return
	}

	result, err := h.svc.TestAI(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrAINotConfigured):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIIncomplete):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			slog.Error("failed to test AI endpoint", "err", err)
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to test AI endpoint"})
		}
		return
	}

	c.JSON(http.StatusOK, AITestResponse{
		Message:  result.Message,
		Response: toString(result.Response),
	})
}

// GetAIModels godoc
//
//	@Summary		List available AI models
//	@Description	Asks the configured AI provider for the list of
//	@Description	models it currently exposes.
//	@Tags			config
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	AIModelsResponse
//	@Failure		400	{object}	api.ErrorResponse	"AI disabled / incomplete config"
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		404	{object}	api.ErrorResponse	"AI not configured"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/ai/models [get]
func (h *Handler) GetAIModels(c *gin.Context) {
	result, err := h.svc.AIModels(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrAINotConfigured):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIIncompleteModels):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			slog.Error("failed to fetch AI models", "err", err)
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch AI models"})
		}
		return
	}

	c.JSON(http.StatusOK, AIModelsResponse{
		Message: result.Message,
		Models:  toStringSlice(result.Response),
	})
}

// GetThemes godoc
//
//	@Summary	List available themes
//	@Tags		config
//	@Produce	json
//	@Success	200	{object}	ThemesListResponse
//	@Router		/config/themes [get]
func (h *Handler) GetThemes(c *gin.Context) {
	themes := h.svc.GetThemes()
	c.JSON(http.StatusOK, ThemesListResponse{Themes: themes})
}

// GetThemePreview godoc
//
//	@Summary		Theme preview image
//	@Description	Streams the preview.png from the theme directory.
//	@Description	Returns 404 if the theme, or its preview file,
//	@Description	does not exist.
//	@Tags			config
//	@Produce		png
//	@Param			id	path	string	true	"theme id"
//	@Success		200	"preview image bytes"
//	@Failure		404	{object}	api.ErrorResponse	"theme or preview not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/config/themes/{id}/preview [get]
func (h *Handler) GetThemePreview(c *gin.Context) {
	themeID := c.Param("id")

	previewPath, err := h.svc.ThemePreview(themeID)
	if err != nil {
		switch {
		case errors.Is(err, ErrThemeNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrPreviewNotSpecified):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		case errors.Is(err, ErrPreviewNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
		default:
			slog.Error("failed to load theme preview", "err", err)
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to load theme preview"})
		}
		return
	}

	// Serve the preview image
	c.File(previewPath)
}

// GetThemeLanguages godoc
//
//	@Summary		Theme i18n languages
//	@Description	Lists the language codes a theme ships under i18n/.
//	@Tags			config
//	@Produce		json
//	@Param			id	path		string	true	"theme id"
//	@Success		200	{object}	ThemeLanguagesResponse
//	@Failure		404	{object}	api.ErrorResponse	"theme not found"
//	@Router			/config/themes/{id}/languages [get]
func (h *Handler) GetThemeLanguages(c *gin.Context) {
	themeID := c.Param("id")
	langs, err := h.svc.ThemeLanguages(themeID)
	if err != nil {
		if errors.Is(err, ErrThemeNotFound) {
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: err.Error()})
			return
		}
		slog.Error("failed to list theme languages", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to list theme languages"})
		return
	}
	c.JSON(http.StatusOK, ThemeLanguagesResponse{Theme: themeID, Languages: langs})
}

// GetThemeConfig godoc
//
//	@Summary	Get the active theme
//	@Tags		config
//	@Produce	json
//	@Success	200	{object}	ThemeConfigResponse
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/config/theme [get]
func (h *Handler) GetThemeConfig(c *gin.Context) {
	activeTheme, err := h.svc.GetThemeConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get theme config"})
		return
	}
	c.JSON(http.StatusOK, ThemeConfigResponse{ActiveTheme: activeTheme})
}

// UpdateThemeConfig godoc
//
//	@Summary	Set the active theme
//	@Tags		config
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		ThemeConfigUpdateRequest	true	"theme id"
//	@Success	200		{object}	ThemeConfigUpdateResponse
//	@Failure	400		{object}	api.ErrorResponse	"theme not found / invalid payload"
//	@Failure	401		{object}	api.ErrorResponse
//	@Failure	500		{object}	api.ErrorResponse
//	@Router		/config/theme [put]
func (h *Handler) UpdateThemeConfig(c *gin.Context) {
	var req ThemeConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}

	activeTheme, err := h.svc.UpdateThemeConfig(c.Request.Context(), req.ActiveTheme)
	if err != nil {
		if errors.Is(err, ErrThemeNotFound) {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
			return
		}
		slog.Error("failed to update theme configuration", "err", err)
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update theme configuration"})
		return
	}

	c.JSON(http.StatusOK, ThemeConfigUpdateResponse{
		Message:     "Theme updated successfully",
		ActiveTheme: activeTheme,
	})
}

// UploadTheme godoc
//
//	@Summary		Upload a theme zip
//	@Description	Accepts a multipart/form-data body with a single
//	@Description	'theme' part. The zip must contain a vexgo-theme.json
//	@Description	metadata file (either at the root or inside a single
//	@Description	subdirectory). Existing themes with the same id are
//	@Description	overwritten.
//	@Tags			config
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			theme	formData	file	true	"theme zip archive"
//	@Success		200		{object}	ThemeUploadResponse
//	@Failure		400		{object}	api.ErrorResponse	"invalid zip / missing metadata / zip-slip entry"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/config/theme/upload [post]
func (h *Handler) UploadTheme(c *gin.Context) {
	// Get the file from the request
	file, header, err := c.Request.FormFile("theme")
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "No file uploaded"})
		return
	}
	defer file.Close()

	// Check if the file is a zip
	if !strings.HasSuffix(header.Filename, ".zip") {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File must be a zip archive"})
		return
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "theme-upload-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create temporary directory"})
		return
	}
	// Clean up the temporary directory once the upload is done.
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Read the zip file
	zipReader, err := zip.NewReader(file, header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid zip file"})
		return
	}

	// Extract the zip file. Entry names are untrusted input: os.Root confines
	// every created file to the extraction directory at the OS level, so
	// absolute paths, ".." segments or volume names in entries cannot escape
	// tempDir. The Clean/IsAbs/".." pre-check below only fails fast with a
	// client-facing 400; the Root is the actual guarantee.
	zipRoot, err := os.OpenRoot(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to prepare extraction directory"})
		return
	}
	defer func() { _ = zipRoot.Close() }()

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Ensure the file path is safe
		clean := filepath.Clean(f.Name)
		if filepath.IsAbs(clean) || strings.Contains(clean, "..") {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid file path in zip"})
			return
		}

		// Create the directory structure inside the extraction root
		if dir := filepath.Dir(clean); dir != "." {
			if err := zipRoot.MkdirAll(dir, 0o755); err != nil {
				c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid file path in zip"})
				return
			}
		}

		// Extract the file
		dstFile, err := zipRoot.Create(clean)
		if err != nil {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid file path in zip"})
			return
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to open zip file"})
			return
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()

		if err != nil {
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to extract file"})
			return
		}
	}

	// Check if the extracted directory contains a vexgo-theme.json file
	var themeInfo public.ThemeInfo
	var themeDir string

	// Find the directory containing vexgo-theme.json
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to read extracted files"})
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			metaPath := filepath.Join(tempDir, entry.Name(), public.ThemeMetaFile)
			if _, err := os.Stat(metaPath); err == nil {
				// Found the theme directory
				themeDir = entry.Name()

				// Read and parse the theme metadata
				content, err := os.ReadFile(metaPath)
				if err != nil {
					c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Failed to read theme metadata"})
					return
				}

				if err := unmarshalThemeMeta(content, &themeInfo); err != nil {
					c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid theme metadata"})
					return
				}

				break
			}
		}
	}

	// If no theme directory found, check if the root contains vexgo-theme.json
	if themeDir == "" {
		metaPath := filepath.Join(tempDir, public.ThemeMetaFile)
		if _, err := os.Stat(metaPath); err == nil {
			// Theme files are in the root of the zip
			content, err := os.ReadFile(metaPath)
			if err != nil {
				c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Failed to read theme metadata"})
				return
			}

			if err := unmarshalThemeMeta(content, &themeInfo); err != nil {
				c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid theme metadata"})
				return
			}

			// Use the theme ID from metadata or generate one
			if themeInfo.ID == "" {
				// Generate a theme ID from the filename
				themeDir = strings.TrimSuffix(header.Filename, ".zip")
				// Remove any non-alphanumeric characters
				themeDir = strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
						return r
					}
					return '_'
				}, themeDir)
			} else {
				themeDir = themeInfo.ID
			}
		} else {
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "No vexgo-theme.json found in the zip file"})
			return
		}
	}

	// Ensure the theme ID is valid. The ID may come from the uploaded
	// vexgo-theme.json, so it is treated as untrusted input: anything but a
	// plain directory name would make filepath.Join below escape the themes
	// directory (arbitrary RemoveAll/MkdirAll/write).
	if themeDir == "" || themeDir == public.DefaultTheme || !themeIDPattern.MatchString(themeDir) {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid theme ID"})
		return
	}

	// Create the theme directory in data/theme
	targetThemeDir := filepath.Join(h.themes.DataDir(), public.ThemesDir, themeDir)

	// Remove existing theme directory if it exists
	if err := os.RemoveAll(targetThemeDir); err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to remove existing theme directory"})
		return
	}

	// Create the target directory
	if err := os.MkdirAll(targetThemeDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create theme directory"})
		return
	}

	// Copy files from temporary directory to target
	sourceDir := tempDir
	if themeDir != "" {
		sourceDir = filepath.Join(tempDir, themeDir)
	}

	// Check if source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		sourceDir = tempDir // Use root if themeDir doesn't exist
	}

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(targetThemeDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
	if err != nil {
		// Clean up partial installation
		if err := os.RemoveAll(targetThemeDir); err != nil {
			slog.Warn("failed to clean up partial theme installation", "err", err)
		}
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to copy theme files"})
		return
	}

	c.JSON(http.StatusOK, ThemeUploadResponse{Message: "Theme uploaded successfully"})
}

// unmarshalThemeMeta decodes theme metadata JSON.
func unmarshalThemeMeta(content []byte, info *public.ThemeInfo) error {
	return json.Unmarshal(content, info)
}

// toString renders an any value as a string. The service layer
// returns a string for AI test results, but the helper exists
// here to keep the JSON-shape conversion explicit at the wire
// boundary.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// toStringSlice renders an any value as a []string. The
// service layer returns a []string for AI model lists, but
// the helper exists to keep the JSON-shape conversion
// explicit at the wire boundary.
func toStringSlice(v any) []string {
	if s, ok := v.([]string); ok {
		return s
	}
	return nil
}
