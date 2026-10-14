package model

import (
	"time"

	"gorm.io/gorm"
)

type Asset struct {
	ID uint `gorm:"primaryKey"`

	OriginalName string `gorm:"size:255;index"`

	// Owner of this asset.
	UserID uint `gorm:"not null;index"`
	User   User `gorm:"foreignKey:UserID"`

	// Physical file of this asset.
	BlobID uint `gorm:"not null;index"`
	Blob   Blob `gorm:"foreignKey:BlobID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
