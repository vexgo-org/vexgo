package asset

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

func NewHandler(deps Deps) *Handler {
	return &Handler{
		svc: NewService(deps),
		mw:  middleware.NewAuth(deps.DB, deps.JWTSecret),
	}
}

// Upload godoc
//
//	@Summary		Upload an asset
//	@Description	Accepts a multipart/form-data body with a single 'file'
//	@Description	part. The storage key is a freshly generated UUID
//	@Description	plus the sanitized extension from the original filename;
//	@Description	any untrusted characters are stripped before persistence.
//	@Description	Requires authentication.
//	@Tags			assets
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			file	formData	file	true	"file to upload"
//	@Success		200		{object}	UploadResponse
//	@Failure		400		{object}	api.ErrorResponse	"missing or malformed form, rejected file type, or invalid filename"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse	"failed to persist file"
//	@Router			/assets/upload [post]
func (h *Handler) Upload(c *gin.Context) {
	// Extract user ID.
	userID := middleware.CurrentUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, api.ErrorResponse{Error: "Not logged in"})
		return
	}

	// Get the file.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, model.MaxBlobRequestBytes)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File upload failed"})
		return
	}

	// Check the file size.
	if file.Size <= 0 {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File is empty"})
		return
	}

	if file.Size > model.MaxBlobBytes {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File is too large"})
		return
	}

	// Open and upload the file.
	reader, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to open file"})
		return
	}
	defer reader.Close()

	// Upload the file to the storage backend.
	asset, err := h.svc.Upload(
		c.Request.Context(),
		userID,
		reader,
		file.Filename,
		file.Size,
	)
	if err != nil {
		slog.Error(
			"failed to upload file",
			"user_id", userID,
			"filename", file.Filename,
			"err", err,
		)

		switch {
		case errors.Is(err, ErrInvalidExtension),
			errors.Is(err, model.ErrUnsupportedMIMEType),
			errors.Is(err, ErrEmptyFile),
			errors.Is(err, ErrInvalidFileSize),
			errors.Is(err, ErrEmptyFilename),
			errors.Is(err, ErrFilenameTooLong):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid file"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "File upload failed"})
		}

		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		Message:  "File uploaded successfully",
		PublicID: asset.PublicID,
	})
}

// Delete godoc
//
//	@Summary		Delete an asset
//	@Description	Soft deletes the asset identified by its public ID.
//	@Description	Only the owner or an administrator may delete it.
//	@Description	Requires authentication.
//	@Tags			assets
//	@Produce		json
//	@Security		BearerAuth
//	@Param			publicID	path		string	true	"asset public id"
//	@Success		200			{object}	MessageResponse
//	@Failure		400			{object}	api.ErrorResponse
//	@Failure		401			{object}	api.ErrorResponse
//	@Failure		403			{object}	api.ErrorResponse	"not the owner"
//	@Failure		404			{object}	api.ErrorResponse	"asset does not exist"
//	@Failure		500			{object}	api.ErrorResponse	"failed to delete asset"
//	@Router			/assets/delete/{publicID} [post]
func (h *Handler) Delete(c *gin.Context) {
	// Get current user.
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid user"})
		return
	}

	// Get the asset public ID.
	publicID, err := uuid.Parse(c.Param("publicID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid asset ID"})
		return
	}

	if err := h.svc.Delete(
		c.Request.Context(),
		Actor{
			ID:   user.ID,
			Role: user.Role,
		},
		publicID,
	); err != nil {
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

// Original godoc
//
//	@Summary		Get original asset content
//	@Description	Returns the original content of an asset.
//	@Tags			assets
//	@Produce		application/octet-stream
//	@Param			publicID	path	string	true	"asset public id"
//	@Success		200			{file}	binary	"original asset content"
//	@Failure		404			"asset not found"
//	@Failure		503			{object}	api.ErrorResponse	"asset content unavailable"
//	@Failure		500			{object}	api.ErrorResponse	"failed to get original asset"
//	@Router			/assets/{publicID}/original [get]
func (h *Handler) Original(c *gin.Context) {
	publicID, err := uuid.Parse(c.Param("publicID"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	content, err := h.svc.ResolveOriginal(c.Request.Context(), publicID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.Status(http.StatusNotFound)
		case errors.Is(err, ErrContentUnavailable):
			c.JSON(http.StatusServiceUnavailable, api.ErrorResponse{Error: "Asset content unavailable"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to get original asset"})
		}
		return
	}
	defer content.Reader.Close()

	headers := map[string]string{
		"Cache-Control":          "private, max-age=31536000, immutable",
		"X-Content-Type-Options": "nosniff",
	}

	if content.Filename != "" {
		headers["Content-Disposition"] = mime.FormatMediaType(
			"inline",
			map[string]string{
				"filename": content.Filename,
			},
		)
	}

	c.DataFromReader(
		http.StatusOK,
		content.Size,
		content.MIMEType,
		content.Reader,
		headers,
	)
}
