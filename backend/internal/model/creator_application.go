package model

import (
	"time"
)

// CreatorApplicationStatus application status
type CreatorApplicationStatus string

const (
	CreatorApplicationStatusPending  CreatorApplicationStatus = "pending"
	CreatorApplicationStatusApproved CreatorApplicationStatus = "approved"
	CreatorApplicationStatusRejected CreatorApplicationStatus = "rejected"
)

// CreatorApplication review actions.
const (
	CreatorApplicationActionApprove = "approve"
	CreatorApplicationActionReject  = "reject"
)

// CreatorApplication creator application model
type CreatorApplication struct {
	ID           uint `gorm:"primaryKey"`
	UserID       uint
	User         User `gorm:"foreignKey:UserID"`
	Status       CreatorApplicationStatus
	Reason       string
	ReviewerID   *uint
	Reviewer     *User `gorm:"foreignKey:ReviewerID"`
	ReviewReason string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName specifies table name
func (CreatorApplication) TableName() string {
	return "creator_applications"
}
