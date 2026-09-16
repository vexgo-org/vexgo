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
	ID        uint       `json:"id" gorm:"primaryKey"`
	Slug      string     `json:"slug" gorm:"size:100;uniqueIndex"`
	Title     string     `json:"title" binding:"required" gorm:"size:255"`
	Content   string     `json:"content" binding:"required" gorm:"type:text"`
	ShowInNav bool       `json:"showInNav" gorm:"default:false"`
	SortOrder int        `json:"sortOrder" gorm:"default:0"`
	Status    PageStatus `json:"status" gorm:"size:20;default:draft"`
	AuthorID  uint       `json:"authorId"`
	Author    User       `json:"author" gorm:"foreignKey:AuthorID"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
