package handler

import (
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"blog-system/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetSMTPConfig 获取 SMTP 配置
func GetSMTPConfig(c *gin.Context) {
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
			c.JSON(http.StatusOK, model.SMTPConfig{
				Enabled:   false,
				Host:      "",
				Port:      587,
				Username:  "",
				Password:  "",
				FromEmail: "",
				FromName:  "Blog System",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}

	// 不返回密码字段
	config.Password = ""
	c.JSON(http.StatusOK, config)
}

// UpdateSMTPConfig 更新 SMTP 配置
func UpdateSMTPConfig(c *gin.Context) {
	var req struct {
		Enabled   bool   `json:"enabled"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"` // 如果为空则不更新密码
		FromEmail string `json:"fromEmail"`
		FromName  string `json:"fromName"`
		TestEmail string `json:"testEmail"` // 测试邮件收件人邮箱
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取现有配置
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新配置
			config = model.SMTPConfig{
				Enabled:   req.Enabled,
				Host:      req.Host,
				Port:      req.Port,
				Username:  req.Username,
				Password:  req.Password,
				FromEmail: req.FromEmail,
				FromName:  req.FromName,
				TestEmail: req.TestEmail,
			}
			if err := db.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SMTP config"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
			return
		}
	} else {
		// 更新现有配置
		config.Enabled = req.Enabled
		config.Host = req.Host
		config.Port = req.Port
		config.Username = req.Username
		config.FromEmail = req.FromEmail
		config.FromName = req.FromName
		config.TestEmail = req.TestEmail

		// 只有当提供了新密码时才更新密码
		if req.Password != "" {
			config.Password = req.Password
		}

		if err := db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SMTP config"})
			return
		}
	}

	// 返回配置，但不包含密码
	config.Password = ""
	c.JSON(http.StatusOK, config)
}

// TestSMTP 测试 SMTP 配置
func TestSMTP(c *gin.Context) {
	// 获取当前管理员用户邮箱（从 JWT token 中获取）
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 获取 SMTP 配置
	var config model.SMTPConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "请先配置 SMTP 设置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SMTP config"})
		return
	}

	// 检查是否启用
	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP 未启用，请先启用并保存配置"})
		return
	}

	// 检查必要字段
	if config.Host == "" || config.Port == 0 || config.Username == "" || config.Password == "" || config.FromEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请完整填写 SMTP 配置信息"})
		return
	}

	// 获取收件人邮箱：优先使用配置的测试邮箱，否则使用当前管理员邮箱
	var recipientEmail string
	if config.TestEmail != "" {
		recipientEmail = config.TestEmail
	} else {
		// 回退到使用管理员邮箱
		if userMap, ok := userContext.(map[string]interface{}); ok {
			if email, ok := userMap["email"].(string); ok {
				recipientEmail = email
			}
		}
	}
	if recipientEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写测试邮箱地址"})
		return
	}

	// 发送测试邮件
	subject := "SMTP 配置测试邮件"
	textBody := fmt.Sprintf(`
尊敬的 %s，

这是一封测试邮件，用于验证您的 SMTP 配置是否正确工作。

如果您收到此邮件，说明 SMTP 配置成功！

配置信息：
- SMTP 服务器: %s:%d
- 发件人: %s <%s>

时间: %s
	`, config.FromName, config.Host, config.Port, config.FromName, config.FromEmail, time.Now().Format("2006-01-02 15:04:05"))

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .success { color: #4CAF50; font-weight: bold; }
        .info { background-color: #e3f2fd; padding: 15px; border-radius: 4px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>SMTP 配置测试</h1>
        </div>
        <div class="content">
            <p>尊敬的 %s，</p>
            <p class="success">✓ 测试邮件发送成功！</p>
            <p>您的 SMTP 配置已正确工作，可以开始使用邮件验证功能了。</p>
            
            <div class="info">
                <strong>配置信息：</strong><br>
                SMTP 服务器: %s:%d<br>
                发件人: %s <%s>
            </div>
            
            <p>时间: %s</p>
        </div>
    </div>
</body>
</html>
	`, config.FromName, config.Host, config.Port, config.FromName, config.FromEmail, time.Now().Format("2006-01-02 15:04:05"))

	// 构建邮件
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := recipientEmail

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n"
	message += "--boundary\r\n"
	message += "Content-Type: text/plain; charset=UTF-8\r\n\r\n"
	message += strings.TrimSpace(textBody) + "\r\n\r\n"
	message += "--boundary\r\n"
	message += "Content-Type: text/html; charset=UTF-8\r\n\r\n"
	message += strings.TrimSpace(htmlBody) + "\r\n\r\n"
	message += "--boundary--\r\n"

	// 连接 SMTP 服务器
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{to}, []byte(message)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("发送测试邮件失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "测试邮件已发送到您的邮箱",
		"to":      recipientEmail,
	})
}
