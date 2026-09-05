// Package upload handles file uploads to the local filesystem or
// to S3-compatible storage, and exposes user-facing file management.
package upload

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the upload domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates an upload HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// ---------- input / output types ----------

type uploadFileInput struct {
	// huma does not parse multipart bodies; the handler reads
	// c.FormFile("file") via GinContextFromContext.
}

type uploadFileOutput struct {
	Body api.UploadFileResponse
}

type uploadFilesInput struct{}

type uploadFilesOutput struct {
	Body api.UploadFilesResponse
}

type listMyFilesInput struct{}

type listMyFilesOutput struct {
	Body api.ListFilesResponse
}

type deleteFileInput struct {
	ID string `path:"id" doc:"Numeric file ID"`
}

type deleteFileOutput struct {
	Body api.UploadMessageResponse
}

// RegisterRoutes registers the upload domain operations.
//
// Authenticated multipart upload is implemented with a small
// GinContextMiddleware so the handler can keep using the gin
// idioms for form data (`c.FormFile`, `c.MultipartForm`). The
// middleware is appended after the huma-side context middleware
// already installed at the API level, so the gin JWT middleware
// (set up via the huma sub-group at the router) still runs first.
func (h *Handler) RegisterRoutes(api huma.API) {
	api.UseMiddleware(auth.GinContextMiddleware)

	huma.Register(api, huma.Operation{
		OperationID: "upload-file",
		Method:      http.MethodPost,
		Path:        "/upload/file",
		Summary:     "Upload a single file",
		Tags:        []string{"upload"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.UploadFile)

	huma.Register(api, huma.Operation{
		OperationID: "upload-files",
		Method:      http.MethodPost,
		Path:        "/upload/files",
		Summary:     "Upload multiple files",
		Tags:        []string{"upload"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.UploadFiles)

	huma.Register(api, huma.Operation{
		OperationID: "list-my-files",
		Method:      http.MethodGet,
		Path:        "/upload/my-files",
		Summary:     "List the current user's uploaded files",
		Tags:        []string{"upload"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.ListMyFiles)

	huma.Register(api, huma.Operation{
		OperationID: "delete-file",
		Method:      http.MethodDelete,
		Path:        "/upload/{id}",
		Summary:     "Delete a file (owner or admin)",
		Tags:        []string{"upload"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.DeleteFile)
}

// ---------- handlers ----------

func (h *Handler) UploadFile(ctx context.Context, _ *uploadFileInput) (*uploadFileOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	c := auth.GinContextFromContext(ctx)
	if c == nil {
		return nil, huma.NewError(500, "Missing gin context")
	}
	file, err := c.FormFile("file")
	if err != nil {
		return nil, huma.NewError(400, "File upload failed")
	}
	filename := generateFilename(file.Filename)
	src, err := file.Open()
	if err != nil {
		return nil, huma.NewError(500, "Failed to open file")
	}
	defer src.Close()
	media, err := h.svc.Upload(ctx, userID, filename, file.Size, src)
	if err != nil {
		return nil, huma.NewError(500, fmt.Sprintf("Failed to upload: %v", err))
	}
	return &uploadFileOutput{
		Body: api.UploadFileResponse{Message: "File uploaded successfully", File: media},
	}, nil
}

func (h *Handler) UploadFiles(ctx context.Context, _ *uploadFilesInput) (*uploadFilesOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	c := auth.GinContextFromContext(ctx)
	if c == nil {
		return nil, huma.NewError(500, "Missing gin context")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return nil, huma.NewError(400, "File upload failed")
	}
	files := form.File["files"]
	uploaded := make([]model.MediaFile, 0, len(files))
	for _, file := range files {
		filename := generateFilename(file.Filename)
		src, err := file.Open()
		if err != nil {
			continue
		}
		media, err := h.svc.Upload(ctx, userID, filename, file.Size, src)
		src.Close()
		if err != nil {
			continue
		}
		uploaded = append(uploaded, media)
	}
	return &uploadFilesOutput{
		Body: api.UploadFilesResponse{Message: "File upload completed", Files: uploaded},
	}, nil
}

func (h *Handler) ListMyFiles(ctx context.Context, _ *listMyFilesInput) (*listMyFilesOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	files, err := h.svc.ListByUser(ctx, userID)
	if err != nil {
		return nil, huma.NewError(500, "Failed to fetch files")
	}
	return &listMyFilesOutput{Body: api.ListFilesResponse{Files: files}}, nil
}

func (h *Handler) DeleteFile(ctx context.Context, in *deleteFileInput) (*deleteFileOutput, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, huma.NewError(401, "Not logged in")
	}
	err := h.svc.Delete(ctx, in.ID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, huma.NewError(404, "File does not exist")
		case errors.Is(err, ErrForbidden):
			return nil, huma.NewError(403, "Not authorized to delete this file")
		default:
			return nil, huma.NewError(500, "Failed to delete file")
		}
	}
	return &deleteFileOutput{Body: api.UploadMessageResponse{Message: "File deleted"}}, nil
}

// ---------- helpers (kept from the legacy handler) ----------

// getFileExtension returns the extension of a filename (including
// the dot). It is preserved from the legacy handler so the
// storage-naming policy is unchanged.
func getFileExtension(filename string) string {
	ext := ""
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			ext = filename[i:]
			break
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ext
}

// maxExtensionLength caps the client-supplied extension length so
// a hostile filename cannot push the stored name past filesystem
// limits.
const maxExtensionLength = 9

// sanitizeExtension extracts the client-supplied extension and
// keeps it only if it is 1-9 lowercase ASCII letters/digits. It
// is the only client-controlled part of the stored filename, so
// anything else (separators, spaces, control bytes, Windows ADS
// colons, overlong tails) means no extension at all.
func sanitizeExtension(filename string) string {
	ext := strings.TrimPrefix(getFileExtension(filename), ".")
	if ext == "" || len(ext) > maxExtensionLength {
		return ""
	}
	ext = strings.ToLower(ext)
	for i := 0; i < len(ext); i++ {
		c := ext[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ""
		}
	}
	return "." + ext
}

// generateFilename generates a unique filename with extension.
func generateFilename(originalName string) string {
	uid := uuid.New().String()
	if ext := sanitizeExtension(originalName); ext != "" {
		return uid + ext
	}
	return uid
}
