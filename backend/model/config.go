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
