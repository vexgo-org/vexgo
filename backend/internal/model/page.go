package model

import (
	"time"
)

// PageStatus is the lifecycle state of a custom page.
type PageStatus string

const (
	PageStatusDraft     PageStatus = "draft"
	PageStatusPublished PageStatus = "published"
)

// Page is a standalone custom page rendered at /:slug.
type Page struct {
	ID        uint       `gorm:"primaryKey"`
	Slug      string     `gorm:"size:100;uniqueIndex"`
	Title     string     `binding:"required" gorm:"size:255"`
	Content   string     `binding:"required" gorm:"type:text"`
	ShowInNav bool       `gorm:"default:false"`
	SortOrder int        `gorm:"default:0"`
	Status    PageStatus `gorm:"size:20;default:draft"`
	AuthorID  uint
	Author    User `gorm:"foreignKey:AuthorID"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
