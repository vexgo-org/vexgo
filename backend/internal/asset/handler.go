package asset

import (
	"log/slog"
	"net/http"

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
	userID := middleware.CurrentUserID(c)

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, model.MaxAssetRequestBytes)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File upload failed"})
		return
	}

	if file.Size > model.MaxAssetBytes {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "File is too large"})
		return
	}

	reader, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to open file"})
		return
	}
	defer reader.Close()

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
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "File upload failed"})
		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		Message: "File uploaded successfully",
		File:    asset,
	})
}
