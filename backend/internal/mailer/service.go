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

func NewService(deps Deps) *Service {
	return &Service{
		client: NewClient(),
		repo:   NewRepository(deps.DB),
	}
}

// SendVerificationEmail sends email verification email
func (s *Service) SendVerificationEmail(ctx context.Context, toEmail, toName, verificationLink string) error {
	if err := s.readConfig(ctx); err != nil {
		return err
	}

	data := verificationEamilTemplateData{
		Name: toName,
		Link: verificationLink,
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

	if err := s.client.Send(Message{
		To:       []string{toEmail},
		Subject:  SUBJECT,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}); err != nil {
		return fmt.Errorf("send verification email failed: %w", err)
	}

	return nil
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
