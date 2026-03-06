package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

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

	// 创建上传目录
	uploadDir := "../frontend/dist/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	// 生成文件名
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
	filepath := filepath.Join(uploadDir, filename)

	// 保存文件
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
		return
	}

	fileURL := fmt.Sprintf("/uploads/%s", filename)

	media := model.MediaFile{
		URL:          fileURL,
		OriginalName: file.Filename,
		Size:         file.Size,
		Type:         "unknown",
		UserID:       userID,
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
	uploadDir := "../frontend/dist/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	var uploadedFiles []model.MediaFile
	for _, file := range files {
		filename := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
		filepath := filepath.Join(uploadDir, filename)

		if err := c.SaveUploadedFile(file, filepath); err != nil {
			continue
		}

		media := model.MediaFile{
			URL:          fmt.Sprintf("/uploads/%s", filename),
			OriginalName: file.Filename,
			Size:         file.Size,
			Type:         "unknown",
			UserID:       userID,
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
	path := "../frontend/dist" + media.URL
	os.Remove(path)
	db.Delete(&media)
	c.JSON(http.StatusOK, gin.H{"message": "文件已删除"})
}
