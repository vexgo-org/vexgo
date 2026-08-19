package upload

import (
	"errors"
	"fmt"
	"net/http"

	"vexgo/backend/internal/model"

	"vexgo/backend/internal/middleware"

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

// generateFilename generates a unique filename with extension
func generateFilename(originalName string) string {
	ext := getFileExtension(originalName)
	uid := uuid.New().String()
	if ext != "" {
		return fmt.Sprintf("%s%s", uid, ext)
	}
	return uid
}

// UploadFile uploads a single file (requires login) and records it in the database.
func (h *Handler) UploadFile(c *gin.Context) {
	var userID uint = 0
	if uid, ok := c.Get("userID"); ok {
		if id, ok2 := uid.(uint); ok2 {
			userID = id
		}
	}

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

	media, err := h.svc.Upload(userID, filename, file.Size, src)
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
	var userID uint = 0
	if uid, ok := c.Get("userID"); ok {
		if id, ok2 := uid.(uint); ok2 {
			userID = id
		}
	}

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

		media, err := h.svc.Upload(userID, filename, file.Size, src)
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
	uid, _ := c.Get("userID")
	userID := uid.(uint)

	files, _ := h.svc.ListByUser(userID)

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// DeleteFile deletes a file (must be uploader or admin).
func (h *Handler) DeleteFile(c *gin.Context) {
	id := c.Param("id")

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	err := h.svc.Delete(id, userID)
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
