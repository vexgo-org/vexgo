package model

import (
	"mime"
	"path"
	"strings"
	"time"

	"gorm.io/gorm"
)

type AssetType string

const (
	AssetTypeAudio   AssetType = "audio"
	AssetTypeImage   AssetType = "image"
	AssetTypeUnknown AssetType = "unknown"
	AssetTypeVideo   AssetType = "video"
)

type Asset struct {
	ID uint `gorm:"primaryKey"`

	OriginalName string `gorm:"size:255;index"`

	// StorageKey format `UUID.ext`
	StorageKey string `gorm:"size:1024;uniqueIndex"`

	URL      string
	MimeType string `gorm:"size:127;index"`

	Type AssetType `gorm:"size:50;index"`

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

func AssetTypeFromMIME(mimeType string) AssetType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return AssetTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return AssetTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return AssetTypeAudio
	}
	return AssetTypeUnknown
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
