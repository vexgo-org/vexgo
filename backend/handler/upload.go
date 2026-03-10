package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var DataDir string

// Get file extension
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ""
	}
	return ext
}

// Upload file (requires login) and record in database
func UploadFile(c *gin.Context) {
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

	// Create upload directory (using media folder in data directory)
	uploadDir := filepath.Join(DataDir, "media")
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	// Get file extension
	ext := getFileExtension(file.Filename)

	// Generate UUID v4 as filename (preserve extension)
	uuid := uuid.New().String()
	var finalFilename string
	if ext != "" {
		finalFilename = fmt.Sprintf("%s%s", uuid, ext)
	} else {
		finalFilename = uuid
	}

	// Build save path
	fullPath := filepath.Join(uploadDir, finalFilename)

	// Save file
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	fileURL := fmt.Sprintf("/uploads/%s", finalFilename)

	media := model.MediaFile{
		URL:    fileURL,
		Size:   file.Size,
		Type:   "unknown",
		UserID: userID,
	}
	db.Create(&media)

	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"file":    media,
	})
}

// Upload multiple files (requires login) and record to database
func UploadFiles(c *gin.Context) {
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
	uploadDir := filepath.Join(DataDir, "media")
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	var uploadedFiles []model.MediaFile
	for _, file := range files {
		// Get file extension
		ext := getFileExtension(file.Filename)

		// Generate UUID v4 as filename
		uuid := uuid.New().String()
		var finalFilename string
		if ext != "" {
			finalFilename = fmt.Sprintf("%s%s", uuid, ext)
		} else {
			finalFilename = uuid
		}

		fullPath := filepath.Join(uploadDir, finalFilename)

		// Save file
		if err := c.SaveUploadedFile(file, fullPath); err != nil {
			continue
		}

		media := model.MediaFile{
			URL:    fmt.Sprintf("/uploads/%s", finalFilename),
			Size:   file.Size,
			Type:   "unknown",
			UserID: userID,
		}
		db.Create(&media)
		uploadedFiles = append(uploadedFiles, media)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File upload completed",
		"files":   uploadedFiles,
	})
}

// Create tag
func CreateTag(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag := model.Tag{
		Name: req.Name,
	}

	if err := db.Create(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tag"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Tag created successfully",
		"tag":     tag,
	})
}

// Get current user's uploaded files list
func GetMyFiles(c *gin.Context) {
	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var files []model.MediaFile
	db.Where("user_id = ?", userID).Find(&files)
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// Delete file (must be uploader or admin)
func DeleteFile(c *gin.Context) {
	id := c.Param("id")
	var media model.MediaFile
	if err := db.First(&media, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File does not exist"})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err == nil {
		if user.Role != "admin" && media.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this file"})
			return
		}
	}

	// Delete physical file
	// media.URL format is "/uploads/filename", need to convert to actual path
	filename := filepath.Base(media.URL)
	path := filepath.Join(DataDir, "media", filename)
	os.Remove(path)
	db.Delete(&media)
	c.JSON(http.StatusOK, gin.H{"message": "File deleted"})
}
