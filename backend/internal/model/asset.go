package model

import (
	"time"

	"gorm.io/gorm"
)

// MaxAssetFilenameLength is the longest `Asset.OriginalName` that fits the
// `size:255` column. The limit counts characters, not bytes, because that is
// what VARCHAR(255) counts on MySQL, MariaDB and Postgres, so callers must
// measure with utf8.RuneCountInString. Counting bytes would reject valid
// multibyte names well before the column would overflow.
const MaxAssetFilenameLength = 255

type AssetVisibility string

const (
	AssetVisibilityPublic  AssetVisibility = "public"
	AssetVisibilityPrivate AssetVisibility = "private"
)

type Asset struct {
	ID uint `gorm:"primaryKey"`

	// OriginalName is the client-supplied filename, kept as display metadata
	// only; the stored object is addressed by the blob's storage key. Limited
	// to MaxAssetFilenameLength characters.
	OriginalName string `gorm:"size:255;index"`

	// Owner of this asset.
	UserID uint `gorm:"not null;index"`
	User   User `gorm:"foreignKey:UserID"`

	// Physical file of this asset.
	BlobID uint `gorm:"not null;index"`
	Blob   Blob `gorm:"foreignKey:BlobID"`

	Visibility AssetVisibility `gorm:"not null;index;default:public"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
