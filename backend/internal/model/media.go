package model

import "time"

// Media file model, used to record user uploaded resources

type MediaFile struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	URL       string    `json:"url" gorm:"size:500"`
	Size      int64     `json:"size"`
	Type      string    `json:"type" gorm:"size:50"` // image/video etc.
	UserID    uint      `json:"userId"`
	User      User      `json:"-" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}
