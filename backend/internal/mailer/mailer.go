// Package mailer sends transactional emails (verification, password reset,
// email change) and manages the associated tokens. Mailer is the only concrete
// implementation of MailSender; callers should depend on the interface so the
// SMTP side effect can be mocked in tests.
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// MailSender is the external-dependency seam for email sending and token
// management. It exists so domain packages can be tested without a real SMTP
// server.
type MailSender interface {
	SendVerificationEmail(ctx context.Context, toEmail, toName, verificationLink string) error
	GenerateVerificationToken(ctx context.Context, userID uint) (string, error)
	IsEmailEnabled() (bool, error)
	SendPasswordResetEmail(ctx context.Context, toEmail, toName, resetLink string) error
	SendEmailChangeEmail(ctx context.Context, toEmail, toName, newEmail, verificationLink string) error
}

// Mailer sends transactional emails via SMTP and manages the associated
// verification / password-reset tokens in the database.
type Mailer struct {
	DB *gorm.DB
}

// MailMessageArgs carries the parts of an outgoing email message.
type MailMessageArgs struct {
	To       string // recipient address
	Subject  string // email subject
	TextBody string // plain-text alternative body
	HTMLBody string // HTML body
}

// compile-time check that Mailer satisfies MailSender
var _ MailSender = (*Mailer)(nil)

// mailCaptureHook, when non-nil, receives the rendered parts of outgoing
// emails instead of sending them over SMTP. It exists solely as a test seam so
// callers can assert on email content without a real SMTP server.
var mailCaptureHook func(to, subject, textBody, htmlBody string)

// SetMailCaptureHook installs a hook that captures outgoing emails. Passing nil
// restores real SMTP sending. Intended for tests.
func SetMailCaptureHook(hook func(to, subject, textBody, htmlBody string)) {
	mailCaptureHook = hook
}

// NewMailer creates a new Mailer instance
func NewMailer(db *gorm.DB) *Mailer {
	return &Mailer{DB: db}
}

// SendVerificationEmail sends email verification email
func (m *Mailer) SendVerificationEmail(ctx context.Context, toEmail, toName, verificationLink string) error {
	config, err := m.getConfig()
	if err != nil {
		return err
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

	// Build email
	message := BuildMailMessage(&MailMessageArgs{
		To:       toEmail,
		Subject:  "Please Verify Your Email Address",
		TextBody: textBody,
		HTMLBody: htmlBody,
	}, config)

	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, "Please Verify Your Email Address", textBody, htmlBody)
		return nil
	}

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	slog.Info("connecting to SMTP server", "addr", addr)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		slog.Error("failed to send verification email", "err", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("verification email sent successfully", "to", toEmail)
	return nil
}

// GenerateVerificationToken generates verification token
func (m *Mailer) GenerateVerificationToken(ctx context.Context, userID uint) (string, error) {
	// Generate random token (should use more secure method in production)
	token := model.TokenPrefixVerify + fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	updates := map[string]any{
		"verification_token": token,
		"token_expires_at":   expiresAt,
	}
	if err := m.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to save verification token: %w", err)
	}

	return token, nil
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
func (m *Mailer) SendPasswordResetEmail(ctx context.Context, toEmail, toName, resetLink string) error {
	// Get SMTP configuration
	config, err := m.getConfig()
	if err != nil {
		return err
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

	// Build email
	message := BuildMailMessage(&MailMessageArgs{
		To:       toEmail,
		Subject:  "Password Reset Request",
		TextBody: textBody,
		HTMLBody: htmlBody,
	}, config)

	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, "Password Reset Request", textBody, htmlBody)
		return nil
	}

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	slog.Info("sending password reset email", "to", toEmail)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		slog.Error("failed to send password reset email", "err", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("password reset email sent successfully", "to", toEmail)
	return nil
}

// SendEmailChangeEmail sends email change confirmation email
func (m *Mailer) SendEmailChangeEmail(ctx context.Context, toEmail, toName, newEmail, verificationLink string) error {
	// Get SMTP configuration
	config, err := m.getConfig()
	if err != nil {
		return err
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

	// Build email
	message := BuildMailMessage(&MailMessageArgs{
		To:       toEmail,
		Subject:  "Confirm Email Change",
		TextBody: textBody,
		HTMLBody: htmlBody,
	}, config)

	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, "Confirm Email Change", textBody, htmlBody)
		return nil
	}

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	slog.Info("sending email change confirmation", "to", toEmail)
	if err := smtp.SendMail(addr, auth, config.FromEmail, []string{toEmail}, []byte(message)); err != nil {
		slog.Error("failed to send email change confirmation", "err", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("email change confirmation sent successfully", "to", toEmail)
	return nil
}

// getConfig loads the SMTP config from the database and fails when SMTP is
// not enabled.
func (m *Mailer) getConfig() (*model.SMTPConfig, error) {
	var config model.SMTPConfig
	if err := m.DB.First(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get SMTP config: %w", err)
	}

	// Check if SMTP is enabled
	if !config.Enabled {
		return nil, fmt.Errorf("SMTP is not enabled")
	}

	return &config, nil
}

// BuildMailMessage renders a multipart/alternative MIME message with both
// plain-text and HTML bodies, using the SMTP config for sender details.
func BuildMailMessage(arg *MailMessageArgs, config *model.SMTPConfig) string {
	const BOUNDARY = "\r\n\r\n--boundary\r\n"

	// Email headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	headers["To"] = arg.To
	headers["Subject"] = arg.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary\""

	// Build email body
	var message strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&message, "%s: %s\r\n", k, v)
	}

	message.WriteString("\r\n")
	message.WriteString("--boundary\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	message.WriteString(strings.TrimSpace(arg.TextBody))
	message.WriteString(BOUNDARY)
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	message.WriteString(strings.TrimSpace(arg.HTMLBody))
	message.WriteString(BOUNDARY)

	return message.String()
}
