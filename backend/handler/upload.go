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

// 获取文件扩展名
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ""
	}
	return ext
}

// 上传文件（需登录），并在数据库记录
func UploadFile(c *gin.Context) {
	var userID uint = 0
	if uid, ok := c.Get("userID"); ok {
		if id, ok2 := uid.(uint); ok2 {
			userID = id
		}
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
		return
	}

	// 创建上传目录（使用数据目录下的 media 文件夹）
	uploadDir := filepath.Join(DataDir, "media")
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	// 获取文件扩展名
	ext := getFileExtension(file.Filename)

	// 生成 UUID v4 作为文件名（保留扩展名）
	uuid := uuid.New().String()
	var finalFilename string
	if ext != "" {
		finalFilename = fmt.Sprintf("%s%s", uuid, ext)
	} else {
		finalFilename = uuid
	}

	// 构建保存路径
	fullPath := filepath.Join(uploadDir, finalFilename)

	// 保存文件
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
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
		"message": "文件上传成功",
		"file":    media,
	})
}

// 上传多个文件（需登录），并记录到数据库
func UploadFiles(c *gin.Context) {
	var userID uint = 0
	if uid, ok := c.Get("userID"); ok {
		if id, ok2 := uid.(uint); ok2 {
			userID = id
		}
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
		return
	}

	files := form.File["files"]
	uploadDir := filepath.Join(DataDir, "media")
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	var uploadedFiles []model.MediaFile
	for _, file := range files {
		// 获取文件扩展名
		ext := getFileExtension(file.Filename)

		// 生成 UUID v4 作为文件名
		uuid := uuid.New().String()
		var finalFilename string
		if ext != "" {
			finalFilename = fmt.Sprintf("%s%s", uuid, ext)
		} else {
			finalFilename = uuid
		}

		fullPath := filepath.Join(uploadDir, finalFilename)

		// 保存文件
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
		"message": "文件上传完成",
		"files":   uploadedFiles,
	})
}

// 创建标签
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建标签失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "标签创建成功",
		"tag":     tag,
	})
}

// 获取当前用户上传的文件列表
func GetMyFiles(c *gin.Context) {
	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var files []model.MediaFile
	db.Where("user_id = ?", userID).Find(&files)
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// 删除文件（需是上传者或管理员）
func DeleteFile(c *gin.Context) {
	id := c.Param("id")
	var media model.MediaFile
	if err := db.First(&media, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err == nil {
		if user.Role != "admin" && media.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该文件"})
			return
		}
	}

	// 删除物理文件
	// media.URL 格式为 "/uploads/filename"，需要转换为实际路径
	filename := filepath.Base(media.URL)
	path := filepath.Join(DataDir, "media", filename)
	os.Remove(path)
	db.Delete(&media)
	c.JSON(http.StatusOK, gin.H{"message": "文件已删除"})
}
