package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type BlobType string

var ErrUnsupportedMIMEType = errors.New("unsupported MIME type")

const (
	BlobTypeAudio BlobType = "audio"
	BlobTypeImage BlobType = "image"
	BlobTypeVideo BlobType = "video"
)

const (
	MaxBlobBytes        = 25 << 20
	MaxBlobRequestBytes = MaxBlobBytes + (1 << 20)
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

func BlobTypeFromMIME(mimeType string) (BlobType, error) {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return BlobTypeImage, nil
	case strings.HasPrefix(mimeType, "video/"):
		return BlobTypeVideo, nil
	case strings.HasPrefix(mimeType, "audio/"):
		return BlobTypeAudio, nil
	}
	return "", fmt.Errorf("%w: %s", ErrUnsupportedMIMEType, mimeType)
}
