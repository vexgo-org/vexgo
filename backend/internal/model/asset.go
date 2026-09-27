package model

import (
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
