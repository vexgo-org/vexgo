package model

import (
	"time"
)

// SMTPConfig 存储 SMTP 邮件服务器配置
type SMTPConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Enabled   bool      `json:"enabled" gorm:"default:false"` // 是否启用 SMTP
	Host      string    `json:"host"`                         // SMTP 服务器地址
	Port      int       `json:"port"`                         // SMTP 端口
	Username  string    `json:"username"`                     // 邮箱账号
	Password  string    `json:"password"`                     // 邮箱密码或授权码
	FromEmail string    `json:"fromEmail"`                    // 发件人邮箱
	FromName  string    `json:"fromName"`                     // 发件人名称
	TestEmail string    `json:"testEmail"`                    // 测试邮件收件人邮箱
	CreatedAt time.Time `json:"created_at"`                   // 创建时间
	UpdatedAt time.Time `json:"updated_at"`                   // 更新时间
}

// GeneralSettings 存储通用系统设置
type GeneralSettings struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	CaptchaEnabled      bool      `json:"captchaEnabled" gorm:"default:false"`     // 是否启用滑块验证
	RegistrationEnabled bool      `json:"registrationEnabled" gorm:"default:true"` // 是否允许注册
	SiteName            string    `json:"siteName" gorm:"default:Blog System"`     // 网站名称
	SiteDescription     string    `json:"siteDescription"`                         // 网站描述
	ItemsPerPage        int       `json:"itemsPerPage" gorm:"default:20"`          // 每页显示数量
	CreatedAt           time.Time `json:"created_at"`                              // 创建时间
	UpdatedAt           time.Time `json:"updated_at"`                              // 更新时间
}

// Captcha 存储滑动拼图验证信息
type Captcha struct {
	ID        string    `json:"id" gorm:"primaryKey"`      // 验证码ID
	Token     string    `json:"token"`                     // 验证令牌
	X         int       `json:"x"`                         // 拼图正确位置X坐标
	Y         int       `json:"y"`                         // 拼图正确位置Y坐标
	Width     int       `json:"width"`                     // 拼图宽度
	Height    int       `json:"height"`                    // 拼图高度
	BgImage   string    `json:"bg_image"`                  // 背景图片Base64
	PuzzleImg string    `json:"puzzle_img"`                // 拼图图片Base64
	ExpiresAt time.Time `json:"expires_at"`                // 过期时间
	Used      bool      `json:"used" gorm:"default:false"` // 是否已使用
	CreatedAt time.Time `json:"created_at"`                // 创建时间
}

// CommentModerationConfig 评论审核配置
type CommentModerationConfig struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	Enabled            bool      `json:"enabled" gorm:"default:false"`              // 是否启用AI评论审核
	ModelProvider      string    `json:"modelProvider" gorm:"default:''"`           // AI模型提供商 (openai, azure, etc.)
	ApiKey             string    `json:"apiKey" gorm:"default:''"`                  // API密钥
	ApiEndpoint        string    `json:"apiEndpoint" gorm:"default:''"`             // API端点
	ModelName          string    `json:"modelName" gorm:"default:'gpt-3.5-turbo'"`  // 模型名称
	ModerationPrompt   string    `json:"moderationPrompt" gorm:"default:''"`        // 审核提示词
	BlockKeywords      string    `json:"blockKeywords" gorm:"type:text;default:''"` // 屏蔽关键词，逗号分隔
	AutoApproveEnabled bool      `json:"autoApproveEnabled" gorm:"default:true"`    // 是否自动批准低风险评论
	MinScoreThreshold  float64   `json:"minScoreThreshold" gorm:"default:0.5"`      // 最低分数阈值（低于此分数将被拒绝）
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// AIConfig 存储大模型API配置
type AIConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Enabled     bool      `json:"enabled" gorm:"default:false"`             // 是否启用大模型
	Provider    string    `json:"provider" gorm:"default:'openai'"`         // 提供商 (openai, azure, etc.)
	ApiEndpoint string    `json:"apiEndpoint" gorm:"default:''"`            // API端点URL
	ApiKey      string    `json:"apiKey" gorm:"default:''"`                 // API密钥
	ModelName   string    `json:"modelName" gorm:"default:'gpt-3.5-turbo'"` // 模型名称
	CreatedAt   time.Time `json:"created_at"`                               // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`                               // 更新时间
}
