package model

import (
	"time"
)

// SMTPConfig stores SMTP mail server configuration
type SMTPConfig struct {
	ID        uint      `gorm:"primaryKey"`
	Enabled   bool      `gorm:"default:false"` // Whether SMTP is enabled
	Host      string    `gorm:"size:255"`      // SMTP server address
	Port      int       // SMTP port
	Username  string    `gorm:"size:255"` // Email account
	Password  string    `gorm:"size:512"` // Email password or authorization code (encrypted at rest)
	FromEmail string    `gorm:"size:255"` // Sender email
	FromName  string    `gorm:"size:100"` // Sender name
	TestEmail string    `gorm:"size:255"` // Test email recipient
	CreatedAt time.Time // Creation time
	UpdatedAt time.Time // Update time
}

// GeneralSettings stores general system settings
type GeneralSettings struct {
	ID                  uint      `gorm:"primaryKey"`
	CaptchaEnabled      bool      `gorm:"default:false"` // Whether captcha verification is enabled
	RegistrationEnabled bool      // Whether registration is allowed
	AllowGuestViewPosts bool      // Whether guests can view posts
	SiteName            string    `gorm:"size:100;default:VexGo"` // Site name
	SiteDescription     string    `gorm:"type:text"`              // Site description
	ItemsPerPage        int       `gorm:"default:20"`             // Items per page
	SiteLanguage        string    `gorm:"size:10;default:en"`     // Default public theme language (e.g. en, zh)
	CreatedAt           time.Time // Creation time
	UpdatedAt           time.Time // Update time

	// Site icon.
	SiteIconID *uint
	SiteIcon   *Asset `gorm:"foreignKey:SiteIconID"`
}

// Captcha stores sliding puzzle captcha information. Only the validation
// state is persisted — the generated images are returned to the client
// directly and never stored, so anonymous challenge requests cannot grow the
// table.
type Captcha struct {
	ID        string    `gorm:"primaryKey;size:255"` // Captcha ID
	Token     string    `gorm:"size:255"`            // Verification token
	X         int       // Puzzle correct position X coordinate
	Y         int       // Puzzle correct position Y coordinate
	Width     int       // Puzzle width
	Height    int       // Puzzle height
	ExpiresAt time.Time // Expiration time
	Used      bool      `gorm:"default:false"` // Whether already used
	CreatedAt time.Time // Creation time
}

// CommentModerationConfig stores comment moderation configuration. The three
// switches are independent: the keyword filter and LLM review only reject
// content, while manual review holds every comment that is not published or
// rejected. All default to false, which publishes new comments immediately.
type CommentModerationConfig struct {
	ID                   uint   `gorm:"primaryKey"`
	ManualReviewEnabled  bool   `gorm:"default:false"`           // Hold comments for manual approval
	KeywordFilterEnabled bool   `gorm:"default:false"`           // Reject on blocked keyword hits
	LLMReviewEnabled     bool   `gorm:"default:false"`           // Reject comments flagged by the LLM
	ModelProvider        string `gorm:"default:''"`              // AI provider (openai, azure, etc.)
	ApiKey               string `gorm:"size:512;default:''"`     // API key (encrypted at rest)
	ApiEndpoint          string `gorm:"default:''"`              // API endpoint
	ModelName            string `gorm:"default:'gpt-3.5-turbo'"` // Model name
	ModerationPrompt     string `gorm:"default:''"`              // Moderation prompt
	BlockKeywords        string `gorm:"size:500;default:''"`     // Blocked keywords, comma-separated
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AIConfig stores AI model API configuration
type AIConfig struct {
	ID          uint      `gorm:"primaryKey"`
	Enabled     bool      `gorm:"default:false"`                    // Whether AI model is enabled
	Provider    string    `gorm:"size:50;default:'openai'"`         // Provider (openai, azure, etc.)
	ApiEndpoint string    `gorm:"size:500;default:''"`              // API endpoint URL
	ApiKey      string    `gorm:"size:512;default:''"`              // API key (encrypted at rest)
	ModelName   string    `gorm:"size:100;default:'gpt-3.5-turbo'"` // Model name
	CreatedAt   time.Time // Creation time
	UpdatedAt   time.Time // Update time
}

// ThemeConfig stores the active theme selection for the entire site
type ThemeConfig struct {
	ID          uint      `gorm:"primaryKey"`
	ActiveTheme string    `gorm:"size:100;default:'default'"` // Active theme ID
	CreatedAt   time.Time // Creation time
	UpdatedAt   time.Time // Update time
}
