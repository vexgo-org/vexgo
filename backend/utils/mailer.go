package utils

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"vexgo/backend/model"

	"gorm.io/gorm"
)

// Mailer email sender
type Mailer struct {
	DB *gorm.DB
}

// NewMailer creates a new Mailer instance
func NewMailer(db *gorm.DB) *Mailer {
	return &Mailer{DB: db}
}

// SendVerificationEmail sends email verification email
func (m *Mailer) SendVerificationEmail(toEmail, toName, verificationLink string) error {
	// Get SMTP configuration
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}

	// Check if SMTP is enabled
	if !config.Enabled {
		return fmt.Errorf("SMTP is not enabled")
	}

	// Email body (text version)
	textBody := fmt.Sprintf(`
Dear %s,

Thank you for registering for our blog system! Please click the following link to complete email verification:

%s

This link will expire in 5 minutes.

If you did not register for this account, please ignore this email.
	`, toName, verificationLink)

	// Email body (HTML version)
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
		           <h1>Email Verification</h1>
		       </div>
		       <div class="content">
		           <p>Dear %s,</p>
		           <p>Thank you for registering for our blog system! Please click the button below to complete email verification:</p>
		           <p>
		               <a href="%s" class="button">Verify Email</a>
		           </p>
	            <p>Or copy and paste the following link into your browser:</p>
	            <p>%s</p>
	            <p>This link will expire in 5 minutes.</p>
		       </div>
		       <div class="footer">
		           <p>If you did not register for this account, please ignore this email.</p>
		       </div>
		   </div>
</body>
</html>
	`, toName, verificationLink, verificationLink)

	// Build email message
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := toEmail

	// Email headers
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = "Please Verify Your Email Address"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	// Build email body
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

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	log.Printf("Connecting to SMTP server %s...", addr)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		log.Printf("Failed to send email: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Verification email successfully sent to %s", toEmail)
	return nil
}

// GenerateVerificationToken generates verification token
func (m *Mailer) GenerateVerificationToken(userID uint) (string, error) {
	// Generate random token (should use more secure method in production)
	token := fmt.Sprintf("verify-%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	updates := map[string]interface{}{
		"verification_token": token,
		"token_expires_at":   expiresAt,
	}
	if err := m.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to save verification token: %w", err)
	}

	return token, nil
}

// VerifyEmail verifies email address
func (m *Mailer) VerifyEmail(token string) error {
	var user model.User
	if err := m.DB.Where("verification_token = ?", token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("invalid verification token")
		}
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Check if token is expired
	if user.TokenExpiresAt.Before(time.Now()) {
		return fmt.Errorf("verification token has expired")
	}

	// Update user verification status
	if err := m.DB.Model(&user).Updates(map[string]interface{}{
		"email_verified":     true,
		"verification_token": "",
		"token_expires_at":   time.Time{},
	}).Error; err != nil {
		return fmt.Errorf("failed to update user verification status: %w", err)
	}

	return nil
}

// IsEmailEnabled checks if SMTP is enabled
func (m *Mailer) IsEmailEnabled() (bool, error) {
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return false, fmt.Errorf("failed to get SMTP config: %w", err)
	}
	return config.Enabled, nil
}

// SendPasswordResetEmail sends password reset email
func (m *Mailer) SendPasswordResetEmail(toEmail, toName, resetLink string) error {
	// Get SMTP configuration
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}

	// Check if SMTP is enabled
	if !config.Enabled {
		return fmt.Errorf("SMTP is not enabled")
	}

	// Email body (text version)
	textBody := fmt.Sprintf(`
Dear %s,

We received a password reset request from your account. Please click the following link to reset your password:

%s

This link will expire in 5 minutes.

If you did not request a password reset, please ignore this email.
	`, toName, resetLink)

	// Email body (HTML version)
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
		           <h1>Password Reset</h1>
		       </div>
		       <div class="content">
		           <p>Dear %s,</p>
		           <p>We received a password reset request from your account. Please click the button below to reset your password:</p>
		           <p>
		               <a href="%s" class="button">Reset Password</a>
		           </p>
	            <p>Or copy and paste the following link into your browser:</p>
	            <p>%s</p>
	            <p>This link will expire in 5 minutes.</p>
		       </div>
		       <div class="footer">
		           <p>If you did not request a password reset, please ignore this email.</p>
		       </div>
		   </div>
</body>
</html>
	`, toName, resetLink, resetLink)

	// Build email message
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := toEmail

	// Email headers
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = "Password Reset Request"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	// Build email body
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

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	log.Printf("Sending password reset email to %s...", toEmail)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		log.Printf("Failed to send password reset email: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Password reset email successfully sent to %s", toEmail)
	return nil
}

// GeneratePasswordResetToken generates password reset token
func (m *Mailer) GeneratePasswordResetToken(userID uint) (string, error) {
	// Generate random token
	token := fmt.Sprintf("reset-%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	updates := map[string]interface{}{
		"verification_token": token,
		"token_expires_at":   expiresAt,
	}
	if err := m.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to save reset token: %w", err)
	}

	return token, nil
}

// GenerateEmailChangeToken generates email change verification token
func (m *Mailer) GenerateEmailChangeToken(userID uint, newEmail string) (string, error) {
	// Generate random token
	token := fmt.Sprintf("email-change-%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database, also store pending new email
	updates := map[string]interface{}{
		"verification_token": token,
		"token_expires_at":   expiresAt,
		"pending_email":      newEmail,
	}
	if err := m.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to save email change token: %w", err)
	}

	return token, nil
}

// SendEmailChangeEmail sends email change confirmation email
func (m *Mailer) SendEmailChangeEmail(toEmail, toName, newEmail, verificationLink string) error {
	// Get SMTP configuration
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return fmt.Errorf("failed to get SMTP config: %w", err)
	}

	// Check if SMTP is enabled
	if !config.Enabled {
		return fmt.Errorf("SMTP is not enabled")
	}

	// Email body (text version)
	textBody := fmt.Sprintf(`
Dear %s,

We received an email change request. Please click the following link to confirm changing your email to %s:

%s

This link will expire in 5 minutes.

If you did not request an email change, please ignore this email.
	`, toName, newEmail, verificationLink)

	// Email body (HTML version)
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
		   <meta charset="UTF-8">
		   <style>
		       body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
		       .container { max-width: 600px; margin: 0 auto; padding: 20px; }
		       .header { background-color: #2196F3; color: white; padding: 20px; text-align: center; }
		       .content { padding: 20px; background-color: #f9f9f9; }
		       .button {
		           display: inline-block;
		           padding: 12px 24px;
		           background-color: #2196F3;
		           color: white;
		           text-decoration: none;
		           border-radius: 4px;
		           margin: 20px 0;
		       }
		       .footer { margin-top: 20px; font-size: 12px; color: #777; }
		       .new-email { font-weight: bold; color: #2196F3; }
		   </style>
</head>
<body>
		   <div class="container">
		       <div class="header">
		           <h1>Confirm Email Change</h1>
		       </div>
		       <div class="content">
		           <p>Dear %s,</p>
		           <p>We received an email change request. Please click the button below to confirm changing your email to:</p>
		           <p class="new-email">%s</p>
		           <p>
		               <a href="%s" class="button">Confirm Change</a>
		           </p>
	            <p>Or copy and paste the following link into your browser:</p>
	            <p>%s</p>
	            <p>This link will expire in 5 minutes.</p>
		       </div>
		       <div class="footer">
		           <p>If you did not request an email change, please ignore this email.</p>
		       </div>
		   </div>
</body>
</html>
	`, toName, newEmail, verificationLink, verificationLink)

	// Build email message
	from := fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	to := toEmail

	// Email headers
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = "Confirm Email Change"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	// Build email body
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

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	log.Printf("Sending email change confirmation to %s...", toEmail)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		log.Printf("Failed to send email change confirmation: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email change confirmation successfully sent to %s", toEmail)
	return nil
}

// ConfirmEmailChange confirms email change
func (m *Mailer) ConfirmEmailChange(token string) error {
	log.Printf("=== ConfirmEmailChange processing started ===")
	log.Printf("Token: %s", token)

	var user model.User
	if err := m.DB.Where("verification_token = ?", token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("Error: Invalid verification token")
			return fmt.Errorf("invalid verification token")
		}
		log.Printf("Error: Failed to query user: %v", err)
		return fmt.Errorf("failed to find user: %w", err)
	}
	log.Printf("Found user: ID=%d, Username=%s, CurrentEmail=%s, PendingEmail=%s",
		user.ID, user.Username, user.Email, user.PendingEmail)

	// Check if token is expired
	if user.TokenExpiresAt.Before(time.Now()) {
		log.Printf("Error: Token expired (ExpiresAt: %v, Now: %v)", user.TokenExpiresAt, time.Now())
		return fmt.Errorf("verification token has expired")
	}
	log.Printf("Token not expired")

	// Check if there is a pending email
	if user.PendingEmail == "" {
		log.Printf("Error: No pending email (PendingEmail is empty)")
		return fmt.Errorf("no pending email change")
	}
	log.Printf("Pending email: %s", user.PendingEmail)

	// Check if the new email is already used by another user
	var existingUser model.User
	if err := m.DB.Where("email = ? AND id != ?", user.PendingEmail, user.ID).First(&existingUser).Error; err == nil {
		log.Printf("Error: Email already in use by another user (UserID=%d)", existingUser.ID)
		return fmt.Errorf("email already in use by another account")
	}
	log.Printf("New email is not used by another user")

	// Update email address
	log.Printf("Starting to update user email...")
	if err := m.DB.Model(&user).Updates(map[string]interface{}{
		"email":              user.PendingEmail,
		"email_verified":     true, // automatically verify the changed email
		"pending_email":      "",
		"verification_token": "",
		"token_expires_at":   time.Time{},
	}).Error; err != nil {
		log.Printf("Error: Failed to update email: %v", err)
		return fmt.Errorf("failed to update email: %w", err)
	}

	log.Printf("Email update successful! New email: %s", user.PendingEmail)
	log.Printf("=== ConfirmEmailChange processing completed ===")
	return nil
}
