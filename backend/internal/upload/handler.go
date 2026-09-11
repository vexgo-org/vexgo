package upload

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the upload domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates an upload HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// getFileExtension returns the extension of a filename (including the dot).
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

// allowedUploadExtensions is the allowlist of extensions kept on the stored
// file. Local uploads are served from this origin, so a file the browser would
// render as a document is a stored XSS vector: html, svg, xhtml, xml and js are
// therefore deliberately absent. Any other extension is dropped; the /uploads
// handler then serves the resulting extensionless file as
// application/octet-stream, which cannot execute.
var allowedUploadExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".avif": {},
	".bmp": {}, ".ico": {}, ".tif": {}, ".tiff": {},
	".mp4": {}, ".webm": {}, ".mov": {}, ".mp3": {}, ".wav": {}, ".ogg": {}, ".m4a": {},
	".pdf": {}, ".txt": {}, ".md": {}, ".csv": {},
}

// Upload size limits. maxUploadBytes caps one file; the request cap adds
// multipart framing headroom so a single request cannot stream unbounded data
// into the server (memory or disk exhaustion).
const (
	maxUploadBytes        = 25 << 20
	maxUploadRequestBytes = maxUploadBytes + (1 << 20)
)

// generateFilename generates a unique filename, keeping the client-supplied
// extension only when it is on the allowlist. The name the client sent is
// otherwise never trusted.
func generateFilename(originalName string) string {
	uid := uuid.New().String()
	ext := strings.ToLower(getFileExtension(originalName))
	if _, ok := allowedUploadExtensions[ext]; ok {
		return uid + ext
	}
	return uid
}

// UploadFile godoc
//
//	@Summary		Upload a single file
//	@Description	Accepts a multipart/form-data body with a single 'file'
//	@Description	part. The stored filename is a freshly generated UUID
//	@Description	plus the sanitized extension from the original filename;
//	@Description	any untrusted characters are stripped before persistence.
//	@Description	Requires authentication.
//	@Tags			uploads
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			file	formData	file	true	"file to upload"
//	@Success		200		{object}	UploadResponse
//	@Failure		400		{object}	api.ErrorResponse	"missing or malformed form"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse	"failed to persist file"
//	@Router			/upload [post]
func (h *Handler) UploadFile(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	// Cap the request body before multipart parsing so an oversized upload
	// cannot exhaust memory or disk.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestBytes)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File upload failed"})
		return
	}
	if file.Size > maxUploadBytes {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File is too large"})
		return
	}

	// Generate unique filename
	filename := generateFilename(file.Filename)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to open file"})
		return
	}
	defer src.Close()

	media, err := h.svc.Upload(c.Request.Context(), userID, filename, file.Size, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: fmt.Sprintf("Failed to upload: %v", err)})
		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		Message: "File uploaded successfully",
		File:    media,
	})
}

// UploadFiles godoc
//
//	@Summary		Upload multiple files
//	@Description	Accepts a multipart/form-data body with one or more
//	@Description	'files' parts. Per-file failures are silently dropped;
//	@Description	the response only includes the rows that persisted.
//	@Description	Requires authentication.
//	@Tags			uploads
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			files	formData	file	true	"files to upload (repeatable)"
//	@Success		200		{object}	MultiUploadResponse
//	@Failure		400		{object}	api.ErrorResponse	"missing or malformed form"
//	@Failure		401		{object}	api.ErrorResponse
//	@Router			/upload/multiple [post]
func (h *Handler) UploadFiles(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	// Cap the request body before multipart parsing so an oversized upload
	// cannot exhaust memory or disk.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestBytes)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File upload failed"})
		return
	}

	files := form.File["files"]
	uploadedFiles := make([]model.MediaFile, 0, len(files))

	for _, file := range files {
		if file.Size > maxUploadBytes {
			continue
		}
		filename := generateFilename(file.Filename)

		src, err := file.Open()
		if err != nil {
			continue
		}

		media, err := h.svc.Upload(c.Request.Context(), userID, filename, file.Size, src)
		_ = src.Close()
		if err != nil {
			continue
		}
		uploadedFiles = append(uploadedFiles, media)
	}

	c.JSON(http.StatusOK, MultiUploadResponse{
		Message: "File upload completed",
		Files:   uploadedFiles,
	})
}

// GetMyFiles godoc
//
//	@Summary	List the authenticated user's uploads
//	@Tags		uploads
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	FilesListResponse
//	@Failure	401	{object}	api.ErrorResponse
//	@Failure	500	{object}	api.ErrorResponse
//	@Router		/upload/my [get]
func (h *Handler) GetMyFiles(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	files, err := h.svc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch files"})
		return
	}

	c.JSON(http.StatusOK, FilesListResponse{Files: files})
}

// DeleteFile godoc
//
//	@Summary		Delete an uploaded file
//	@Description	Only the original uploader or an admin may delete a file.
//	@Description	The id is the MediaFile.ID from the upload response.
//	@Tags			uploads
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"file id (UUID filename without extension)"
//	@Success		200	{object}	MessageResponse
//	@Failure		403	{object}	api.ErrorResponse	"not authorized"
//	@Failure		404	{object}	api.ErrorResponse	"file not found"
//	@Failure		500	{object}	api.ErrorResponse
//	@Router			/upload/{id} [delete]
func (h *Handler) DeleteFile(c *gin.Context) {
	id := c.Param("id")

	userID := middleware.CurrentUserID(c)

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "File does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Not authorized to delete this file"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete file"})
		}
		return
	}

	c.JSON(http.StatusOK, MessageResponse{Message: "File deleted"})
}
