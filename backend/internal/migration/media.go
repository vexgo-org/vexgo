package migration

import (
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// mediaFile is a deprecated model used before.
// It is now a private model only used in `migration` package.
type mediaFile struct {
	ID   uint   `gorm:"primaryKey"`
	URL  string `gorm:"size:500"`
	Size int64
	Type string `gorm:"size:50"` // image/video etc.
	// StorageKey is what the backend was given; empty on rows written before keys
	// were recorded. Kept beside URL so a delete addresses the object, not the link.
	// Internal plumbing, so it stays out of the API surface like User.
	StorageKey string
	UserID     uint
	User       model.User `gorm:"foreignKey:UserID"`
	CreatedAt  time.Time
}
