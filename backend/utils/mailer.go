package utils

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"blog-system/backend/model"

	"gorm.io/gorm"
)

// Mailer 邮件发送器
type Mailer struct {
	DB *gorm.DB
}

// NewMailer 创建邮件发送器实例
func NewMailer(db *gorm.DB) *Mailer {
	return &Mailer{DB: db}
}

// SendVerificationEmail 发送邮箱验证邮件
func (m *Mailer) SendVerificationEmail(toEmail, toName, verificationLink string) error {
	// 获取 SMTP 配置
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}

	// 检查是否启用 SMTP
	if !config.Enabled {
		return fmt.Errorf("SMTP is not enabled")
	}

	// 邮件正文
	textBody := fmt.Sprintf(`
尊敬的 %s，

感谢您注册我们的博客系统！请点击以下链接完成邮箱验证：

%s

此链接将在 5 分钟后失效。

如果您没有注册此账户，请忽略此邮件。
	`, toName, verificationLink)

	// HTML 格式的邮件正文
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
	       .button {
	           display: inline-block;
	           padding: 12px 24px;
	           background-color: #4CAF50;
	           color: white;
	           text-decoration: none;
	           border-radius: 4px;
	           margin: 20px 0;
	       }
	       .footer { margin-top: 20px; font-size: 12px; color: #777; }
	   </style>
</head>
<body>
	   <div class="container">
	       <div class="header">
	           <h1>邮箱验证</h1>
	       </div>
	       <div class="content">
	           <p>尊敬的 %s，</p>
	           <p>感谢您注册我们的博客系统！请点击下面的按钮完成邮箱验证：</p>
	           <p>
	               <a href="%s" class="button">验证邮箱</a>
	           </p>
            <p>或者复制以下链接到浏览器中打开：</p>
            <p>%s</p>
            <p>此链接将在 5 分钟后失效。</p>
	       </div>
	       <div class="footer">
	           <p>如果您没有注册此账户，请忽略此邮件。</p>
	       </div>
	   </div>
</body>
</html>
	`, toName, verificationLink, verificationLink)

	// 构建邮件
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := toEmail

	// 邮件头
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = "请验证您的邮箱地址"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	// 构建邮件体
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

	log.Printf("正在连接到 SMTP 服务器 %s...", addr)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		log.Printf("发送邮件失败: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("验证邮件已成功发送到 %s", toEmail)
	return nil
}

// GenerateVerificationToken 生成验证令牌
func (m *Mailer) GenerateVerificationToken(userID uint) (string, error) {
	// 生成随机令牌（实际应用中应该使用更安全的方式）
	token := fmt.Sprintf("verify-%d-%d", userID, time.Now().UnixNano())

	// 计算过期时间（5分钟后）
	expiresAt := time.Now().Add(5 * time.Minute)

	// 保存到数据库
	updates := map[string]interface{}{
		"verification_token": token,
		"token_expires_at":   expiresAt,
	}
	if err := m.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to save verification token: %w", err)
	}

	return token, nil
}

// VerifyEmail 验证邮箱
func (m *Mailer) VerifyEmail(token string) error {
	var user model.User
	if err := m.DB.Where("verification_token = ?", token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("invalid verification token")
		}
		return fmt.Errorf("failed to find user: %w", err)
	}

	// 检查令牌是否过期
	if user.TokenExpiresAt.Before(time.Now()) {
		return fmt.Errorf("verification token has expired")
	}

	// 更新用户验证状态
	if err := m.DB.Model(&user).Updates(map[string]interface{}{
		"email_verified":     true,
		"verification_token": "",
		"token_expires_at":   time.Time{},
	}).Error; err != nil {
		return fmt.Errorf("failed to update user verification status: %w", err)
	}

	return nil
}

// IsEmailEnabled 检查是否启用了 SMTP
func (m *Mailer) IsEmailEnabled() (bool, error) {
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return false, fmt.Errorf("failed to get SMTP config: %w", err)
	}
	return config.Enabled, nil
}

// SendPasswordResetEmail 发送密码重置邮件
func (m *Mailer) SendPasswordResetEmail(toEmail, toName, resetLink string) error {
	// 获取 SMTP 配置
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}

	// 检查是否启用 SMTP
	if !config.Enabled {
		return fmt.Errorf("SMTP is not enabled")
	}

	// 邮件正文
	textBody := fmt.Sprintf(`
尊敬的 %s，

我们收到了您的密码重置请求。请点击以下链接重置您的密码：

%s

此链接将在 5 分钟后失效。

如果您没有请求重置密码，请忽略此邮件。
	`, toName, resetLink)

	// HTML 格式的邮件正文
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	   <meta charset="UTF-8">
	   <style>
	       body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
	       .container { max-width: 600px; margin: 0 auto; padding: 20px; }
	       .header { background-color: #f44336; color: white; padding: 20px; text-align: center; }
	       .content { padding: 20px; background-color: #f9f9f9; }
	       .button {
	           display: inline-block;
	           padding: 12px 24px;
	           background-color: #f44336;
	           color: white;
	           text-decoration: none;
	           border-radius: 4px;
	           margin: 20px 0;
	       }
	       .footer { margin-top: 20px; font-size: 12px; color: #777; }
	   </style>
</head>
<body>
	   <div class="container">
	       <div class="header">
	           <h1>密码重置</h1>
	       </div>
	       <div class="content">
	           <p>尊敬的 %s，</p>
	           <p>我们收到了您的密码重置请求。请点击下面的按钮重置您的密码：</p>
	           <p>
	               <a href="%s" class="button">重置密码</a>
	           </p>
           <p>或者复制以下链接到浏览器中打开：</p>
           <p>%s</p>
           <p>此链接将在 5 分钟后失效。</p>
	       </div>
	       <div class="footer">
	           <p>如果您没有请求重置密码，请忽略此邮件。</p>
	       </div>
	   </div>
</body>
</html>
	`, toName, resetLink, resetLink)

	// 构建邮件
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := toEmail

	// 邮件头
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = "密码重置请求"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	// 构建邮件体
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

	log.Printf("正在发送密码重置邮件到 %s...", toEmail)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		log.Printf("发送密码重置邮件失败: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("密码重置邮件已成功发送到 %s", toEmail)
	return nil
}

// GeneratePasswordResetToken 生成密码重置令牌
func (m *Mailer) GeneratePasswordResetToken(userID uint) (string, error) {
	// 生成随机令牌
	token := fmt.Sprintf("reset-%d-%d", userID, time.Now().UnixNano())

	// 计算过期时间（1小时后）
	expiresAt := time.Now().Add(5 * time.Minute)

	// 保存到数据库
	updates := map[string]interface{}{
		"verification_token": token,
		"token_expires_at":   expiresAt,
	}
	if err := m.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to save reset token: %w", err)
	}

	return token, nil
}
