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

const (
	MaxAssetBytes        = 25 << 20
	MaxAssetRequestBytes = MaxAssetBytes + (1 << 20)
)

type Asset struct {
	ID uint `json:"id" gorm:"primaryKey"`

	OriginalName string `json:"originalName" gorm:"size:255;index"`

	// StorageKey format `UUID.ext`
	StorageKey string `json:"-" gorm:"size:1024;uniqueIndex"`

	URL      string `json:"url"`
	MimeType string `json:"mimeType" gorm:"size:127;index"`

	Type AssetType `json:"type" gorm:"size:50;index"`

	Size int64 `json:"size"`

	UserID uint `json:"userId" gorm:"not null;index"`
	User   User `json:"-" gorm:"foreignKey:UserID"`

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
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
