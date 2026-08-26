package mailer

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
}

type Service struct {
	repo   Repository
	client SMTPMailer
}

// mailCaptureHook, when non-nil, receives the rendered parts of outgoing
// emails instead of sending them over SMTP. It exists solely as a test seam so
// callers can assert on email content without a real SMTP server.
var mailCaptureHook func(to, subject, textBody, htmlBody string)

// SetMailCaptureHook installs a hook that captures outgoing emails. Passing nil
// restores real SMTP sending. Intended for tests.
func SetMailCaptureHook(hook func(to, subject, textBody, htmlBody string)) {
	mailCaptureHook = hook
}

func NewService(deps Deps) *Service {
	return &Service{
		client: NewClient(),
		repo:   NewRepository(deps.DB),
	}
}

// SendVerificationEmail sends email verification email
func (s *Service) SendVerificationEmail(
	ctx context.Context,
	toEmail string,
	data *VerificationEmailTemplateData,
) error {
	if err := s.readConfig(ctx); err != nil {
		return err
	}

	htmlBody, err := RenderHTMLTemplate(
		verificationEmailTemplateHTML,
		data,
	)
	if err != nil {
		return fmt.Errorf("render html verification email failed: %w", err)
	}

	textBody, err := RenderTextTemplate(
		verificationEmailTemplateText,
		data,
	)
	if err != nil {
		return fmt.Errorf("render text verification email failed: %w", err)
	}

	const SUBJECT = "Please Verify Your Email Address"

	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, SUBJECT, textBody, htmlBody)
		return nil
	}

	if err := s.client.Send(ctx, Message{
		To:       []string{toEmail},
		Subject:  SUBJECT,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}); err != nil {
		return fmt.Errorf("send verification email failed: %w", err)
	}

	return nil
}

// SendPasswordResetEmail sends password reset email
func (s *Service) SendPasswordResetEmail(
	ctx context.Context, toEmail string,
	data *PasswordResetEmailTemplateData,
) error {
	if err := s.readConfig(ctx); err != nil {
		return err
	}

	textBody, err := RenderTextTemplate(
		resetPasswordEmailTemplateText,
		data,
	)
	if err != nil {
		return fmt.Errorf("render text password reset email failed: %w", err)
	}

	htmlBody, err := RenderHTMLTemplate(
		resetPasswordEmailTemplateHTML,
		data,
	)
	if err != nil {
		return fmt.Errorf("render html password reset email failed: %w", err)
	}

	const SUBJECT = "Password Reset Request"
	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, SUBJECT, textBody, htmlBody)
		return nil
	}

	if err := s.client.Send(ctx, Message{
		To:       []string{toEmail},
		Subject:  SUBJECT,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}); err != nil {
		return fmt.Errorf("send password reset email failed: %w", err)
	}

	return nil
}

// SendEmailChangeEmail sends email change confirmation email
func (s *Service) SendEmailChangeEmail(
	ctx context.Context,
	toEmail string,
	data *EmailChangeEmailTemplateData,
) error {
	if err := s.readConfig(ctx); err != nil {
		return err
	}

	textBody, err := RenderTextTemplate(emailChangeEmailTemplateText, data)
	if err != nil {
		return fmt.Errorf("render text email change email failed: %w", err)
	}

	htmlBody, err := RenderHTMLTemplate(emailChangeEmailTemplateHTML, data)
	if err != nil {
		return fmt.Errorf("render html email change email failed: %w", err)
	}

	const SUBJECT = "Confirm Email Change"

	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, SUBJECT, textBody, htmlBody)
		return nil
	}

	if err := s.client.Send(ctx, Message{
		To:       []string{toEmail},
		Subject:  SUBJECT,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}); err != nil {
		return fmt.Errorf("send email change email failed: %w", err)
	}

	return nil
}

func (s *Service) SendTestSMTPEmail(
	ctx context.Context,
	toEmail string,
	data *TestSMTPEmailTemplateData,
) error {
	if err := s.readConfig(ctx); err != nil {
		return err
	}

	const SUBJECT = "SMTP Configuration Test Email"

	textBody, err := RenderTextTemplate(testSMTPEmailTemplateText, data)
	if err != nil {
		return fmt.Errorf("render text test SMTP email failed: %w", err)
	}

	htmlBody, err := RenderHTMLTemplate(testSMTPEmailTemplateHTML, data)
	if err != nil {
		return fmt.Errorf("render html test SMTP email failed: %w", err)
	}

	if mailCaptureHook != nil {
		mailCaptureHook(toEmail, SUBJECT, textBody, htmlBody)
		return nil
	}

	if err := s.client.Send(ctx, Message{
		To:       []string{toEmail},
		Subject:  SUBJECT,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}); err != nil {
		return fmt.Errorf("send test SMTP email failed: %w", err)
	}

	return nil
}

// Enabled indicates whether SMTP is enabled.
func (s *Service) Enabled(ctx context.Context) (bool, error) {
	cfg, err := s.repo.GetSMTPSetting(ctx)
	if err != nil {
		return false, fmt.Errorf("read SMTP configuration failed: %w", err)
	}
	return cfg.Enabled, nil
}

// readConfig reads SMTP configuration from database, and
// updates the configuraiton of SMTP client.
func (s *Service) readConfig(ctx context.Context) error {
	cfg, err := s.repo.GetSMTPSetting(ctx)
	if err != nil {
		return fmt.Errorf("read SMTP configuration failed: %w", err)
	}

	if err := s.client.LoadConfig(cfg); err != nil {
		return fmt.Errorf("load SMTP configuration failed: %w", err)
	}

	if !s.client.Enabled() {
		return errors.New("SMTP is not enabled")
	}

	return nil
}
