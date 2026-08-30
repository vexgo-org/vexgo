package upload

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// maxExtensionLength caps the client-supplied extension length so a hostile
// filename cannot push the stored name past filesystem limits.
const maxExtensionLength = 9

// sanitizeExtension extracts the client-supplied extension and keeps it only
// if it is 1-9 lowercase ASCII letters/digits. It is the only client-controlled
// part of the stored filename, so anything else (separators, spaces, control
// bytes, Windows ADS colons, overlong tails) means no extension at all.
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

// generateFilename generates a unique filename with extension
func generateFilename(originalName string) string {
	uid := uuid.New().String()
	if ext := sanitizeExtension(originalName); ext != "" {
		return uid + ext
	}
	return uid
}

// UploadFile uploads a single file (requires login) and records it in the database.
func (h *Handler) UploadFile(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File upload failed"})
		return
	}

	// Generate unique filename
	filename := generateFilename(file.Filename)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	media, err := h.svc.Upload(c.Request.Context(), userID, filename, file.Size, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"file":    media,
	})
}

// UploadFiles uploads multiple files (requires login) and records them in the database.
func (h *Handler) UploadFiles(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File upload failed"})
		return
	}

	files := form.File["files"]
	var uploadedFiles []model.MediaFile

	for _, file := range files {
		filename := generateFilename(file.Filename)

		src, err := file.Open()
		if err != nil {
			continue
		}

		media, err := h.svc.Upload(c.Request.Context(), userID, filename, file.Size, src)
		src.Close()
		if err != nil {
			continue
		}
		uploadedFiles = append(uploadedFiles, media)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File upload completed",
		"files":   uploadedFiles,
	})
}

// GetMyFiles returns the current user's uploaded files.
func (h *Handler) GetMyFiles(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	files, err := h.svc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// DeleteFile deletes a file (must be uploader or admin).
func (h *Handler) DeleteFile(c *gin.Context) {
	id := c.Param("id")

	userID := middleware.CurrentUserID(c)

	err := h.svc.Delete(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "File does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this file"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted"})
}
