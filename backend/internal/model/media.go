package model

import "time"

// Media file model, used to record user uploaded resources

type MediaFile struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	URL  string `json:"url" gorm:"size:500"`
	Size int64  `json:"size"`
	Type string `json:"type" gorm:"size:50"` // image/video etc.
	// StorageKey is what the backend was given; empty on rows written before keys
	// were recorded. Kept beside URL so a delete addresses the object, not the link.
	// Internal plumbing, so it stays out of the API surface like User.
	StorageKey string    `json:"-"`
	UserID     uint      `json:"userId"`
	User       User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt  time.Time `json:"createdAt"`
}
