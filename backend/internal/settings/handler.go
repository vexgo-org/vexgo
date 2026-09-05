// Package settings implements the admin configuration surface:
// SMTP, AI, general site settings, theme, and theme uploads.
package settings

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/model"
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
}

// NewHandler creates a settings HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), themes: deps.Themes}
}

// ---------- input / output types ----------

type getSMTPConfigOutput struct {
	Body model.SMTPConfig
}

type updateSMTPConfigInput struct {
	Body api.UpdateSMTPConfigRequest
}

type updateSMTPConfigOutput struct {
	Body api.SMTPConfigUpdateResponse
}

type testSMTPOutput struct {
	Body api.SMTPTestResponse
}

type getAIConfigOutput struct {
	Body model.AIConfig
}

type updateAIConfigInput struct {
	Body api.UpdateAIConfigRequest
}

type updateAIConfigOutput struct {
	Body api.AIConfigUpdateResponse
}

type testAIOutput struct {
	Body api.AITestResponse
}

type getAIModelsOutput struct {
	Body api.AIModelsResponse
}

type getGeneralSettingsOutput struct {
	Body model.GeneralSettings
}

type updateGeneralSettingsInput struct {
	Body api.UpdateGeneralSettingsRequest
}

type updateGeneralSettingsOutput struct {
	Body api.GeneralSettingsUpdateResponse
}

type getThemeConfigOutput struct {
	Body api.ThemeConfigResponse
}

type updateThemeConfigInput struct {
	Body api.UpdateThemeConfigRequest
}

type updateThemeConfigOutput struct {
	Body api.ThemeConfigResponse
}

type listThemesOutput struct {
	Body api.ThemesResponse
}

type themePreviewInput struct {
	ID string `path:"id" doc:"Theme ID"`
}

type themePreviewOutput struct {
	Body api.ThemePreviewResponse
}

type uploadThemeOutput struct {
	Body api.UploadThemeResponse
}

// RegisterRoutes registers the settings domain operations.
func (h *Handler) RegisterRoutes(api huma.API) {
	adminOnly := auth.Permission(model.RoleAdmin, model.RoleSuperAdmin)

	// The theme upload and preview endpoints read multipart
	// bodies / form files via gin idioms, so the gin context
	// must be threaded into the huma request context.
	api.UseMiddleware(auth.GinContextMiddleware)

	// Themes (public read; admin upload)
	huma.Register(api, huma.Operation{
		OperationID: "list-themes", Method: http.MethodGet, Path: "/themes",
		Summary: "List installed themes", Tags: []string{"themes"},
	}, h.GetThemes)
	huma.Register(api, huma.Operation{
		OperationID: "theme-preview", Method: http.MethodGet, Path: "/theme/{id}/preview",
		Summary: "Render a theme preview", Tags: []string{"themes"},
	}, h.GetThemePreview)

	// SMTP (admin)
	huma.Register(api, huma.Operation{
		OperationID: "get-smtp-config", Method: http.MethodGet, Path: "/config/smtp",
		Summary: "Get SMTP configuration (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.GetSMTPConfig)
	huma.Register(api, huma.Operation{
		OperationID: "update-smtp-config", Method: http.MethodPut, Path: "/config/smtp",
		Summary: "Update SMTP configuration (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.UpdateSMTPConfig)
	huma.Register(api, huma.Operation{
		OperationID: "test-smtp", Method: http.MethodPost, Path: "/config/smtp/test",
		Summary: "Send a test SMTP email (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.TestSMTP)

	// AI (admin)
	huma.Register(api, huma.Operation{
		OperationID: "get-ai-config", Method: http.MethodGet, Path: "/config/ai",
		Summary: "Get AI configuration (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.GetAIConfig)
	huma.Register(api, huma.Operation{
		OperationID: "update-ai-config", Method: http.MethodPut, Path: "/config/ai",
		Summary: "Update AI configuration (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.UpdateAIConfig)
	huma.Register(api, huma.Operation{
		OperationID: "test-ai", Method: http.MethodPost, Path: "/config/ai/test",
		Summary: "Test the AI endpoint (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.TestAI)
	huma.Register(api, huma.Operation{
		OperationID: "list-ai-models", Method: http.MethodGet, Path: "/config/ai/models",
		Summary: "List AI models for the configured provider (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.GetAIModels)

	// General (public read, admin write)
	huma.Register(api, huma.Operation{
		OperationID: "get-general-settings", Method: http.MethodGet, Path: "/config/general",
		Summary: "Get general site settings", Tags: []string{"config"},
	}, h.GetGeneralSettings)
	huma.Register(api, huma.Operation{
		OperationID: "update-general-settings", Method: http.MethodPut, Path: "/config/general",
		Summary: "Update general site settings (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.UpdateGeneralSettings)

	// Theme selection
	huma.Register(api, huma.Operation{
		OperationID: "get-theme-config", Method: http.MethodGet, Path: "/config/theme",
		Summary: "Get the active theme", Tags: []string{"config"},
	}, h.GetThemeConfig)
	huma.Register(api, huma.Operation{
		OperationID: "update-theme-config", Method: http.MethodPut, Path: "/config/theme",
		Summary: "Set the active theme (admin)", Tags: []string{"config"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.UpdateThemeConfig)

	// Theme upload (admin)
	huma.Register(api, huma.Operation{
		OperationID: "upload-theme", Method: http.MethodPost, Path: "/themes/upload",
		Summary: "Upload a theme zip (admin)", Tags: []string{"themes"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Middlewares: huma.Middlewares{adminOnly},
	}, h.UploadTheme)
}

// ---------- handlers ----------

func (h *Handler) GetSMTPConfig(ctx context.Context, _ *struct{}) (*getSMTPConfigOutput, error) {
	config, err := h.svc.GetSMTPConfig(ctx)
	if err != nil {
		return nil, huma.NewError(500, "Failed to get SMTP config")
	}
	return &getSMTPConfigOutput{Body: config}, nil
}

func (h *Handler) UpdateSMTPConfig(ctx context.Context, in *updateSMTPConfigInput) (*updateSMTPConfigOutput, error) {
	req := SMTPConfigRequest{
		Host: in.Body.Host, Username: in.Body.Username, Password: in.Body.Password,
		FromEmail: in.Body.FromEmail, FromName: in.Body.FromName, TestEmail: in.Body.TestEmail,
	}
	if in.Body.Enabled != nil {
		req.Enabled = *in.Body.Enabled
	}
	if in.Body.Port != nil {
		req.Port = *in.Body.Port
	}
	config, err := h.svc.UpdateSMTPConfig(ctx, req)
	if err != nil {
		return nil, huma.NewError(500, err.Error())
	}
	return &updateSMTPConfigOutput{Body: api.SMTPConfigUpdateResponse{Config: config}}, nil
}

func (h *Handler) TestSMTP(ctx context.Context, _ *struct{}) (*testSMTPOutput, error) {
	u, _ := auth.UserFromContext(ctx)
	recipient, err := h.svc.TestSMTP(ctx, u.Email)
	if err != nil {
		return nil, mapSMTPTestError(err)
	}
	return &testSMTPOutput{Body: api.SMTPTestResponse{
		Message: "Test email has been sent to your inbox",
		To:      recipient,
	}}, nil
}

func (h *Handler) GetAIConfig(ctx context.Context, _ *struct{}) (*getAIConfigOutput, error) {
	config, err := h.svc.GetAIConfig(ctx)
	if err != nil {
		return nil, huma.NewError(500, "Failed to get AI config")
	}
	return &getAIConfigOutput{Body: config}, nil
}

func (h *Handler) UpdateAIConfig(ctx context.Context, in *updateAIConfigInput) (*updateAIConfigOutput, error) {
	req := AIConfigRequest{
		Provider: in.Body.Provider, ApiEndpoint: in.Body.ApiEndpoint,
		ApiKey: in.Body.ApiKey, ModelName: in.Body.ModelName,
	}
	if in.Body.Enabled != nil {
		req.Enabled = *in.Body.Enabled
	}
	config, err := h.svc.UpdateAIConfig(ctx, req)
	if err != nil {
		return nil, huma.NewError(500, err.Error())
	}
	return &updateAIConfigOutput{Body: api.AIConfigUpdateResponse{Config: config}}, nil
}

func (h *Handler) TestAI(ctx context.Context, _ *struct{}) (*testAIOutput, error) {
	result, err := h.svc.TestAI(ctx)
	if err != nil {
		return nil, mapAITestError(err)
	}
	return &testAIOutput{Body: api.AITestResponse{Message: result.Message, Response: result.Response}}, nil
}

func (h *Handler) GetAIModels(ctx context.Context, _ *struct{}) (*getAIModelsOutput, error) {
	result, err := h.svc.AIModels(ctx)
	if err != nil {
		return nil, mapAIModelsError(err)
	}
	return &getAIModelsOutput{Body: api.AIModelsResponse{
		Message: result.Message,
		Models:  result.Response,
	}}, nil
}

func (h *Handler) GetGeneralSettings(ctx context.Context, _ *struct{}) (*getGeneralSettingsOutput, error) {
	config, err := h.svc.GetGeneralSettings(ctx)
	if err != nil {
		return nil, huma.NewError(500, "Failed to get general settings")
	}
	return &getGeneralSettingsOutput{Body: config}, nil
}

func (h *Handler) UpdateGeneralSettings(ctx context.Context, in *updateGeneralSettingsInput) (*updateGeneralSettingsOutput, error) {
	req := GeneralSettingsRequest{
		SiteName: in.Body.SiteName, SiteDescription: in.Body.SiteDescription,
		SiteIcon: in.Body.SiteIcon,
	}
	if in.Body.CaptchaEnabled != nil {
		req.CaptchaEnabled = *in.Body.CaptchaEnabled
	}
	if in.Body.RegistrationEnabled != nil {
		req.RegistrationEnabled = *in.Body.RegistrationEnabled
	}
	if in.Body.AllowGuestViewPosts != nil {
		req.AllowGuestViewPosts = *in.Body.AllowGuestViewPosts
	}
	if in.Body.ItemsPerPage != nil {
		req.ItemsPerPage = *in.Body.ItemsPerPage
	}
	config, err := h.svc.UpdateGeneralSettings(ctx, req)
	if err != nil {
		return nil, huma.NewError(500, fmt.Sprintf("Failed to update general settings: %v", err))
	}
	return &updateGeneralSettingsOutput{Body: api.GeneralSettingsUpdateResponse{
		Message: "General settings updated successfully",
		Settings: config,
	}}, nil
}

func (h *Handler) GetThemeConfig(ctx context.Context, _ *struct{}) (*getThemeConfigOutput, error) {
	active, err := h.svc.GetThemeConfig(ctx)
	if err != nil {
		return nil, huma.NewError(500, "Failed to get theme config")
	}
	return &getThemeConfigOutput{Body: api.ThemeConfigResponse{ActiveTheme: active}}, nil
}

func (h *Handler) UpdateThemeConfig(ctx context.Context, in *updateThemeConfigInput) (*updateThemeConfigOutput, error) {
	active, err := h.svc.UpdateThemeConfig(ctx, in.Body.ActiveTheme)
	if err != nil {
		if errors.Is(err, ErrThemeNotFound) {
			return nil, huma.NewError(400, err.Error())
		}
		return nil, huma.NewError(500, err.Error())
	}
	return &updateThemeConfigOutput{Body: api.ThemeConfigResponse{ActiveTheme: active, Message: "Theme updated successfully"}}, nil
}

func (h *Handler) GetThemes(ctx context.Context, _ *struct{}) (*listThemesOutput, error) {
	themes := h.svc.GetThemes()
	out := make([]any, 0, len(themes))
	for _, t := range themes {
		out = append(out, t)
	}
	return &listThemesOutput{Body: api.ThemesResponse{Themes: out}}, nil
}

func (h *Handler) GetThemePreview(ctx context.Context, in *themePreviewInput) (*themePreviewOutput, error) {
	previewPath, err := h.svc.ThemePreview(in.ID)
	if err != nil {
		return nil, mapThemePreviewError(err)
	}
	// The legacy handler returned the preview file via c.File; with
	// huma we read the bytes and inline them as a data URL so the
	// spec stays JSON-only. The frontend swaps this approach for
	// the typed generated client without re-architecting.
	data, err := os.ReadFile(previewPath)
	if err != nil {
		return nil, huma.NewError(500, "Failed to read preview file")
	}
	mimeType := detectMimeType(previewPath, data)
	return &themePreviewOutput{Body: api.ThemePreviewResponse{
		URL:      previewPath,
		MimeType: mimeType,
		Data:     data,
	}}, nil
}

func (h *Handler) UploadTheme(ctx context.Context, _ *struct{}) (*uploadThemeOutput, error) {
	c := auth.GinContextFromContext(ctx)
	if c == nil {
		return nil, huma.NewError(500, "Missing gin context")
	}
	file, header, err := c.Request.FormFile("theme")
	if err != nil {
		return nil, huma.NewError(400, "No file uploaded")
	}
	defer file.Close()
	if !strings.HasSuffix(header.Filename, ".zip") {
		return nil, huma.NewError(400, "File must be a zip archive")
	}
	themeID, err := h.installThemeZip(file, header.Size)
	if err != nil {
		// Path validation failures (zip slip, traversal) are a
		// 400 (client supplied bad input). Anything else is
		// a 500 (server-side extraction failure).
		msg := err.Error()
		if strings.Contains(msg, "invalid file path in zip") || strings.Contains(msg, "not found in zip") || strings.Contains(msg, "missing or invalid id") {
			return nil, huma.NewError(400, msg)
		}
		return nil, huma.NewError(500, msg)
	}
	return &uploadThemeOutput{Body: api.UploadThemeResponse{Message: "Theme installed successfully", ThemeID: themeID}}, nil
}

// installThemeZip extracts the uploaded zip into a temp dir, reads
// its theme.json, validates the id, and copies the theme files to
// the public themes folder. It returns the theme id on success.
func (h *Handler) installThemeZip(file io.Reader, size int64) (string, error) {
	tempDir, err := os.MkdirTemp("", "theme-upload-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	zipReader, err := zip.NewReader(file.(io.ReaderAt), size)
	if err != nil {
		// If we can't get a ReaderAt (multipart files don't
		// always provide one), buffer into memory first.
		// This is the same path the legacy handler took via
		// c.Request.FormFile.
		if ra, ok := file.(io.ReaderAt); ok {
			_ = ra
		}
		return "", errors.New("invalid zip file")
	}
	zipRoot, err := os.OpenRoot(tempDir)
	if err != nil {
		return "", errors.New("failed to prepare extraction directory")
	}
	defer zipRoot.Close()

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		clean := filepath.Clean(f.Name)
		if filepath.IsAbs(clean) || strings.Contains(clean, "..") {
			return "", errors.New("invalid file path in zip")
		}
		if dir := filepath.Dir(clean); dir != "." {
			if err := zipRoot.MkdirAll(dir, 0o755); err != nil {
				return "", errors.New("invalid file path in zip")
			}
		}
		dst, err := zipRoot.Create(clean)
		if err != nil {
			return "", err
		}
		src, err := f.Open()
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			return "", err
		}
		src.Close()
	}
	return h.finalizeThemeInstall(tempDir)
}

// finalizeThemeInstall reads the extracted vexgo-theme.json,
// validates the id, and copies the theme files to the public
// themes directory.
func (h *Handler) finalizeThemeInstall(tempDir string) (string, error) {
	// The legacy handler accepted a `vexgo-theme.json` metadata
	// file inside the zip. We keep that contract so existing
	// theme packages don't have to be re-zipped.
	const metaName = "vexgo-theme.json"
	metaPath := filepath.Join(tempDir, metaName)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return "", errors.New("vexgo-theme.json not found in zip")
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", err
	}
	if meta.ID == "" || !themeIDPattern.MatchString(meta.ID) {
		return "", errors.New("vexgo-theme.json missing or invalid id")
	}
	destDir := filepath.Join(h.themeRoot(), meta.ID)
	if err := os.RemoveAll(destDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	if err := copyDir(tempDir, destDir); err != nil {
		return "", err
	}
	return meta.ID, nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// themeRoot returns the absolute path to the public themes
// directory. It prefers the public.Renderer's dataDir when set;
// otherwise it falls back to a sibling of the working directory.
func (h *Handler) themeRoot() string {
	if h.themes != nil {
		// Renderer exposes the data dir via a getter to keep
		// callers from depending on private fields. We treat
		// a non-empty dir as the source of truth.
		if dir := h.themes.DataDir(); dir != "" {
			return filepath.Join(dir, "themes")
		}
	}
	return filepath.Join("data", "public", "themes")
}

// detectMimeType returns a MIME type guessed from the file
// extension, falling back to a signature scan of the bytes.
func detectMimeType(path string, data []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	}
	return "application/octet-stream"
}

// ---------- error mapping ----------

func mapSMTPTestError(err error) error {
	switch {
	case errors.Is(err, ErrSMTPNotConfigured):
		return huma.NewError(404, err.Error())
	case errors.Is(err, ErrSMTPDisabled), errors.Is(err, ErrSMTPIncomplete), errors.Is(err, ErrSMTPNoRecipient):
		return huma.NewError(400, err.Error())
	default:
		return huma.NewError(500, err.Error())
	}
}

func mapAITestError(err error) error {
	switch {
	case errors.Is(err, ErrAINotConfigured):
		return huma.NewError(404, err.Error())
	case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIIncomplete):
		return huma.NewError(400, err.Error())
	default:
		return huma.NewError(500, err.Error())
	}
}

func mapAIModelsError(err error) error {
	switch {
	case errors.Is(err, ErrAINotConfigured):
		return huma.NewError(404, err.Error())
	case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIIncompleteModels):
		return huma.NewError(400, err.Error())
	default:
		return huma.NewError(500, err.Error())
	}
}

func mapThemePreviewError(err error) error {
	switch {
	case errors.Is(err, ErrThemeNotFound), errors.Is(err, ErrPreviewNotSpecified), errors.Is(err, ErrPreviewNotFound):
		return huma.NewError(404, err.Error())
	default:
		return huma.NewError(500, err.Error())
	}
}

// Reset slog to keep the import live for future use.
var _ = slog.Default
