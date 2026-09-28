package model

import (
	"mime"
	"path"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Asset struct {
	ID uint `gorm:"primaryKey"`

	OriginalName string `gorm:"size:255;index"`

	StorageKey string `gorm:"size:1024;index"`
	URL        string
	MimeType   string `gorm:"size:127;index"`

	Type string `gorm:"size:50;index"`

	Size int64

	UserID uint `gorm:"not null;index"`
	User   User `gorm:"foreignKey:UserID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Asset) TableName() string {
	return "media_files"
}

func AssetTypeFromMIME(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	}
	return "unknown"
}

func MimeTypeFromName(name string) string {
	extension := strings.ToLower(path.Ext(name))

	contentType := mime.TypeByExtension(extension)
	if contentType != "" {
		return contentType
	}

	switch extension {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
