package model

import (
	"time"
)

// SMTPConfig stores SMTP mail server configuration
type SMTPConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Enabled   bool      `json:"enabled" gorm:"default:false"` // Whether SMTP is enabled
	Host      string    `json:"host" gorm:"size:255"`         // SMTP server address
	Port      int       `json:"port"`                         // SMTP port
	Username  string    `json:"username" gorm:"size:255"`     // Email account
	Password  string    `json:"password" gorm:"size:512"`     // Email password or authorization code (encrypted at rest)
	FromEmail string    `json:"fromEmail" gorm:"size:255"`    // Sender email
	FromName  string    `json:"fromName" gorm:"size:100"`     // Sender name
	TestEmail string    `json:"testEmail" gorm:"size:255"`    // Test email recipient
	CreatedAt time.Time `json:"created_at"`                   // Creation time
	UpdatedAt time.Time `json:"updated_at"`                   // Update time
}

// GeneralSettings stores general system settings
type GeneralSettings struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	CaptchaEnabled      bool      `json:"captchaEnabled" gorm:"default:false"`    // Whether captcha verification is enabled
	RegistrationEnabled bool      `json:"registrationEnabled"`                    // Whether registration is allowed
	AllowGuestViewPosts bool      `json:"allowGuestViewPosts"`                    // Whether guests can view posts
	SiteName            string    `json:"siteName" gorm:"size:100;default:VexGo"` // Site name
	SiteDescription     string    `json:"siteDescription" gorm:"type:text"`       // Site description
	SiteIcon            string    `json:"siteIcon" gorm:"type:text"`              // Site icon URL
	ItemsPerPage        int       `json:"itemsPerPage" gorm:"default:20"`         // Items per page
	CreatedAt           time.Time `json:"created_at"`                             // Creation time
	UpdatedAt           time.Time `json:"updated_at"`                             // Update time
}

// Captcha stores sliding puzzle captcha information
type Captcha struct {
	ID        string    `json:"id" gorm:"primaryKey;size:255"` // Captcha ID
	Token     string    `json:"token" gorm:"size:255"`         // Verification token
	X         int       `json:"x"`                             // Puzzle correct position X coordinate
	Y         int       `json:"y"`                             // Puzzle correct position Y coordinate
	Width     int       `json:"width"`                         // Puzzle width
	Height    int       `json:"height"`                        // Puzzle height
	BgImage   string    `json:"bg_image" gorm:"type:text"`     // Background image Base64
	PuzzleImg string    `json:"puzzle_img" gorm:"type:text"`   // Puzzle image Base64
	ExpiresAt time.Time `json:"expires_at"`                    // Expiration time
	Used      bool      `json:"used" gorm:"default:false"`     // Whether already used
	CreatedAt time.Time `json:"created_at"`                    // Creation time
}

// CommentModerationConfig stores comment moderation configuration. The three
// switches are independent: the keyword filter and LLM review only reject
// content, while manual review holds every comment that is not published or
// rejected. All default to false, which publishes new comments immediately.
type CommentModerationConfig struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	ManualReviewEnabled  bool      `json:"manualReviewEnabled" gorm:"default:false"`  // Hold every new comment for manual approval
	KeywordFilterEnabled bool      `json:"keywordFilterEnabled" gorm:"default:false"` // Reject comments containing blocked keywords
	LLMReviewEnabled     bool      `json:"llmReviewEnabled" gorm:"default:false"`     // Reject comments flagged by the configured LLM
	ModelProvider        string    `json:"modelProvider" gorm:"default:''"`           // AI model provider (openai, azure, etc.)
	ApiKey               string    `json:"apiKey" gorm:"size:512;default:''"`         // API key (encrypted at rest)
	ApiEndpoint          string    `json:"apiEndpoint" gorm:"default:''"`             // API endpoint
	ModelName            string    `json:"modelName" gorm:"default:'gpt-3.5-turbo'"`  // Model name
	ModerationPrompt     string    `json:"moderationPrompt" gorm:"default:''"`        // Moderation prompt
	BlockKeywords        string    `json:"blockKeywords" gorm:"size:500;default:''"`  // Blocked keywords, comma-separated
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// AIConfig stores AI model API configuration
type AIConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Enabled     bool      `json:"enabled" gorm:"default:false"`                      // Whether AI model is enabled
	Provider    string    `json:"provider" gorm:"size:50;default:'openai'"`          // Provider (openai, azure, etc.)
	ApiEndpoint string    `json:"apiEndpoint" gorm:"size:500;default:''"`            // API endpoint URL
	ApiKey      string    `json:"apiKey" gorm:"size:512;default:''"`                 // API key (encrypted at rest)
	ModelName   string    `json:"modelName" gorm:"size:100;default:'gpt-3.5-turbo'"` // Model name
	CreatedAt   time.Time `json:"created_at"`                                        // Creation time
	UpdatedAt   time.Time `json:"updated_at"`                                        // Update time
}

// ThemeConfig stores the active theme selection for the entire site
type ThemeConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ActiveTheme string    `json:"activeTheme" gorm:"size:100;default:'default'"` // Active theme ID
	CreatedAt   time.Time `json:"created_at"`                                    // Creation time
	UpdatedAt   time.Time `json:"updated_at"`                                    // Update time
}
