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

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/public"

	"github.com/gin-gonic/gin"
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

// GetSMTPConfig gets SMTP configuration
func (h *Handler) GetSMTPConfig(c *gin.Context) {
	config, err := h.svc.GetSMTPConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateSMTPConfig updates SMTP configuration
func (h *Handler) UpdateSMTPConfig(c *gin.Context) {
	var req api.UpdateSMTPConfigRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateSMTPConfig(c.Request.Context(), SMTPConfigRequest{
		Enabled:   req.Enabled,
		Host:      req.Host,
		Port:      req.Port,
		Username:  req.Username,
		Password:  req.Password,
		FromEmail: req.FromEmail,
		FromName:  req.FromName,
		TestEmail: req.TestEmail,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.SMTPConfigUpdateResponse{
		Message:    "SMTP configuration updated successfully",
		SMTPConfig: config,
	})
}

// TestSMTP tests SMTP configuration
func (h *Handler) TestSMTP(c *gin.Context) {
	// Get current admin user email (from JWT token)
	userContext, exists := c.Get(middleware.CtxUserKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrSMTPDisabled), errors.Is(err, ErrSMTPIncomplete), errors.Is(err, ErrSMTPNoRecipient):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, api.SMTPTestResponse{
		Message: "Test email has been sent to your inbox",
		To:      recipientEmail,
	})
}

// GetGeneralSettings gets general settings
func (h *Handler) GetGeneralSettings(c *gin.Context) {
	config, err := h.svc.GetGeneralSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get general settings"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateGeneralSettings updates general settings
func (h *Handler) UpdateGeneralSettings(c *gin.Context) {
	var req api.UpdateGeneralSettingsRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateGeneralSettings(c.Request.Context(), GeneralSettingsRequest{
		CaptchaEnabled:      req.CaptchaEnabled,
		RegistrationEnabled: req.RegistrationEnabled,
		AllowGuestViewPosts: req.AllowGuestViewPosts,
		SiteName:            req.SiteName,
		SiteDescription:     req.SiteDescription,
		SiteIcon:            req.SiteIcon,
		ItemsPerPage:        req.ItemsPerPage,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.GeneralSettingsUpdateResponse{
		Message:         "General settings updated successfully",
		GeneralSettings: config,
	})
}

// GetAIConfig gets AI configuration
func (h *Handler) GetAIConfig(c *gin.Context) {
	config, err := h.svc.GetAIConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI config"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateAIConfig updates AI configuration
func (h *Handler) UpdateAIConfig(c *gin.Context) {
	var req api.UpdateAIConfigRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	config, err := h.svc.UpdateAIConfig(c.Request.Context(), AIConfigRequest{
		Enabled:     req.Enabled,
		Provider:    req.Provider,
		ApiEndpoint: req.ApiEndpoint,
		ApiKey:      req.ApiKey,
		ModelName:   req.ModelName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.AIConfigUpdateResponse{
		Message:  "AI config updated successfully",
		AIConfig: config,
	})
}

// TestAI tests AI configuration connection
func (h *Handler) TestAI(c *gin.Context) {
	// Get current admin user information (from JWT token)
	if _, exists := c.Get(middleware.CtxUserKey); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	result, err := h.svc.TestAI(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrAINotConfigured):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIIncomplete):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  result.Message,
		"response": result.Response,
	})
}

// GetAIModels gets available AI model list
func (h *Handler) GetAIModels(c *gin.Context) {
	result, err := h.svc.AIModels(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrAINotConfigured):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIIncompleteModels):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
		"models":  result.Response,
	})
}

// GetThemes returns all available themes
func (h *Handler) GetThemes(c *gin.Context) {
	themes := h.svc.GetThemes()
	out := make([]api.Theme, 0, len(themes))
	for _, theme := range themes {
		out = append(out, api.Theme{
			ID:          theme.ID,
			Name:        theme.Name,
			Author:      theme.Author,
			Version:     theme.Version,
			Description: theme.Description,
			URL:         theme.URL,
		})
	}
	c.JSON(http.StatusOK, api.ThemesResponse{Themes: out})
}

// GetThemePreview returns the preview image for a theme
func (h *Handler) GetThemePreview(c *gin.Context) {
	themeID := c.Param("id")

	previewPath, err := h.svc.ThemePreview(themeID)
	if err != nil {
		switch {
		case errors.Is(err, ErrThemeNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPreviewNotSpecified):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPreviewNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Serve the preview image
	c.File(previewPath)
}

// GetThemeConfig returns the currently active theme stored in the database
func (h *Handler) GetThemeConfig(c *gin.Context) {
	activeTheme, err := h.svc.GetThemeConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get theme config"})
		return
	}
	c.JSON(http.StatusOK, api.ActiveThemeResponse{ActiveTheme: activeTheme})
}

// UpdateThemeConfig sets the globally active theme in the database
func (h *Handler) UpdateThemeConfig(c *gin.Context) {
	var req api.UpdateThemeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request payload", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	activeTheme, err := h.svc.UpdateThemeConfig(c.Request.Context(), req.ActiveTheme)
	if err != nil {
		if errors.Is(err, ErrThemeNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, api.ThemeUpdateResponse{
		Message:     "Theme updated successfully",
		ActiveTheme: activeTheme,
	})
}

// UploadTheme handles theme zip file upload and extraction
func (h *Handler) UploadTheme(c *gin.Context) {
	// Get the file from the request
	file, header, err := c.Request.FormFile("theme")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Check if the file is a zip
	if !strings.HasSuffix(header.Filename, ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be a zip archive"})
		return
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "theme-upload-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary directory"})
		return
	}
	// Clean up the temporary directory once the upload is done.
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Read the zip file
	zipReader, err := zip.NewReader(file, header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid zip file"})
		return
	}

	// Extract the zip file. Entry names are untrusted input: os.Root confines
	// every created file to the extraction directory at the OS level, so
	// absolute paths, ".." segments or volume names in entries cannot escape
	// tempDir. The Clean/IsAbs/".." pre-check below only fails fast with a
	// client-facing 400; the Root is the actual guarantee.
	zipRoot, err := os.OpenRoot(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare extraction directory"})
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path in zip"})
			return
		}

		// Create the directory structure inside the extraction root
		if dir := filepath.Dir(clean); dir != "." {
			if err := zipRoot.MkdirAll(dir, 0o755); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path in zip"})
				return
			}
		}

		// Extract the file
		dstFile, err := zipRoot.Create(clean)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path in zip"})
			return
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open zip file"})
			return
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract file"})
			return
		}
	}

	// Check if the extracted directory contains a vexgo-theme.json file
	var themeInfo public.ThemeInfo
	var themeDir string

	// Find the directory containing vexgo-theme.json
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read extracted files"})
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
					c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read theme metadata"})
					return
				}

				if err := unmarshalThemeMeta(content, &themeInfo); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid theme metadata"})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read theme metadata"})
				return
			}

			if err := unmarshalThemeMeta(content, &themeInfo); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid theme metadata"})
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "No vexgo-theme.json found in the zip file"})
			return
		}
	}

	// Ensure the theme ID is valid. The ID may come from the uploaded
	// vexgo-theme.json, so it is treated as untrusted input: anything but a
	// plain directory name would make filepath.Join below escape the themes
	// directory (arbitrary RemoveAll/MkdirAll/write).
	if themeDir == "" || themeDir == public.DefaultTheme || !themeIDPattern.MatchString(themeDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid theme ID"})
		return
	}

	// Create the theme directory in data/theme
	targetThemeDir := filepath.Join(h.themes.DataDir(), public.ThemesDir, themeDir)

	// Remove existing theme directory if it exists
	if err := os.RemoveAll(targetThemeDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove existing theme directory"})
		return
	}

	// Create the target directory
	if err := os.MkdirAll(targetThemeDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create theme directory"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy theme files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Theme uploaded successfully"})
}

// unmarshalThemeMeta decodes theme metadata JSON.
func unmarshalThemeMeta(content []byte, info *public.ThemeInfo) error {
	return json.Unmarshal(content, info)
}
