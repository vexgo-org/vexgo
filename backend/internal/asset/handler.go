package asset

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
//	@Failure		400		{object}	api.ErrorResponse	"missing or malformed form"
//	@Failure		401		{object}	api.ErrorResponse
//	@Failure		500		{object}	api.ErrorResponse	"failed to persist file"
//	@Router			/asset/upload [post]
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
		case errors.Is(err, ErrInvalidExtension):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid file extension"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "File upload failed"})
		}

		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		Message: "File uploaded successfully",
		ID:      asset.ID,
	})
}

// Delete godoc
//
//	@Summary		Delete an asset
//	@Description	Soft deletes the asset identified by its ID.
//	@Description	Only the owner or an administrator may delete it.
//	@Description	Requires authentication.
//	@Tags			assets
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"asset id"
//	@Success		200	{object}	MessageResponse
//	@Failure		400	{object}	api.ErrorResponse
//	@Failure		401	{object}	api.ErrorResponse
//	@Failure		403	{object}	api.ErrorResponse	"not the owner"
//	@Failure		404	{object}	api.ErrorResponse	"asset does not exist"
//	@Failure		500	{object}	api.ErrorResponse	"failed to delete asset"
//	@Router			/asset/delete/{id} [post]
func (h *Handler) Delete(c *gin.Context) {
	// Get current user.
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid user"})
		return
	}

	// Get the asset ID.
	assetID, ok := parseUintParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid asset ID"})
		return
	}

	if err := h.svc.Delete(
		c.Request.Context(),
		Actor{
			ID:   user.ID,
			Role: user.Role,
		},
		assetID,
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

// parseUintParam parses a numeric route parameter by name, reporting whether it
// is valid. Zero, non-numeric and out-of-range values cannot identify a row and
// are treated as missing resources: callers must reject an invalid value
// instead of proceeding with 0, which would resolve to row 0 and write orphaned
// records or surface as a 500 rather than a client error. The bit size matches
// uint so the conversion can never truncate silently.
func parseUintParam(c *gin.Context, name string) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param(name), 10, strconv.IntSize)
	if err != nil || id64 == 0 {
		return 0, false
	}
	return uint(id64), true
}
