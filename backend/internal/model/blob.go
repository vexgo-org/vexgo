package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type BlobType string

const (
	BlobTypeAudio   BlobType = "audio"
	BlobTypeImage   BlobType = "image"
	BlobTypeUnknown BlobType = "unknown"
	BlobTypeVideo   BlobType = "video"
)

type Blob struct {
	ID uint `gorm:"primaryKey"`

	// The key to locate the file in the storage backend.
	StorageKey string `gorm:"size:255;uniqueIndex"`

	MimeType string   `gorm:"size:127;index"`
	Type     BlobType `gorm:"size:50;index"`

	Size int64

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func BlobTypeFromMIME(mimeType string) BlobType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return BlobTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return BlobTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return BlobTypeAudio
	}
	return BlobTypeUnknown
}
