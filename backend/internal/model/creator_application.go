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
	ID           uint                     `gorm:"primaryKey" json:"id"`
	UserID       uint                     `json:"user_id"`
	User         User                     `json:"user" gorm:"foreignKey:UserID"`
	Status       CreatorApplicationStatus `json:"status"`
	Reason       string                   `json:"reason"`
	ReviewerID   *uint                    `json:"reviewer_id"`
	Reviewer     *User                    `json:"reviewer" gorm:"foreignKey:ReviewerID"`
	ReviewReason string                   `json:"review_reason"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

// TableName specifies table name
func (CreatorApplication) TableName() string {
	return "creator_applications"
}
