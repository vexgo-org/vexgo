package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// SMTPConfigRequest carries the fields accepted when updating the SMTP config.
type SMTPConfigRequest struct {
	Enabled   bool
	Host      string
	Port      int
	Username  string
	Password  string // if empty, don't update password
	FromEmail string
	FromName  string
	TestEmail string
}

// GetSMTPConfig returns the stored SMTP configuration with the password
// masked, or the default configuration when no row exists.
func (s *Service) GetSMTPConfig(ctx context.Context) (model.SMTPConfig, error) {
	config, err := s.repo.GetSMTPConfig(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Return default configuration
		return model.SMTPConfig{
			Enabled:   false,
			Host:      "",
			Port:      587,
			Username:  "",
			Password:  "",
			FromEmail: "",
			FromName:  "VexGo",
		}, nil
	}
	if err != nil {
		return config, err
	}

	// Don't return password field
	config.Password = ""
	return config, nil
}

// UpdateSMTPConfig creates or updates the SMTP configuration and returns it
// with the password masked.
func (s *Service) UpdateSMTPConfig(ctx context.Context, req SMTPConfigRequest) (model.SMTPConfig, error) {
	config, err := s.repo.GetSMTPConfig(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new configuration
		password, encErr := s.encryptSecret(req.Password, "smtp_config.password")
		if encErr != nil {
			return model.SMTPConfig{}, encErr
		}
		config = model.SMTPConfig{
			Enabled:   req.Enabled,
			Host:      req.Host,
			Port:      req.Port,
			Username:  req.Username,
			Password:  password,
			FromEmail: req.FromEmail,
			FromName:  req.FromName,
			TestEmail: req.TestEmail,
		}
		if err := s.repo.CreateSMTPConfig(ctx, &config); err != nil {
			return config, fmt.Errorf("create SMTP config: %w", err)
		}

		// Return configuration without password
		config.Password = ""
		return config, nil
	}
	if err != nil {
		return config, fmt.Errorf("get SMTP config: %w", err)
	}

	// Update existing configuration
	config.Enabled = req.Enabled
	config.Host = req.Host
	config.Port = req.Port
	config.Username = req.Username
	config.FromEmail = req.FromEmail
	config.FromName = req.FromName
	config.TestEmail = req.TestEmail

	// Only update password if new password is provided
	if req.Password != "" {
		password, encErr := s.encryptSecret(req.Password, "smtp_config.password")
		if encErr != nil {
			return config, encErr
		}
		config.Password = password
	}

	if err := s.repo.SaveSMTPConfig(ctx, &config); err != nil {
		return config, fmt.Errorf("update SMTP config: %w", err)
	}

	// Return configuration without password
	config.Password = ""
	return config, nil
}

// TestSMTP sends a test email to the configured test address (or the admin's
// email as fallback) using the stored SMTP configuration.
func (s *Service) TestSMTP(ctx context.Context, adminEmail string) (string, error) {
	config, err := s.repo.GetSMTPConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrSMTPNotConfigured
		}
		return "", err
	}

	// Check if enabled
	if !config.Enabled {
		return "", ErrSMTPDisabled
	}

	// Check required fields
	if config.Host == "" || config.Port == 0 || config.Username == "" || config.Password == "" || config.FromEmail == "" {
		return "", ErrSMTPIncomplete
	}

	// Get recipient email: use configured test email first, otherwise use current admin email
	var recipientEmail string
	if config.TestEmail != "" {
		recipientEmail = config.TestEmail
	} else {
		recipientEmail = adminEmail
	}
	if recipientEmail == "" {
		return "", ErrSMTPNoRecipient
	}

	if err := s.mailer.SendTestSMTPEmail(ctx, recipientEmail, &mailer.TestSMTPEmailTemplateData{
		Name:  config.FromName,
		Host:  config.Host,
		Port:  config.Port,
		Email: config.FromEmail,
		Time:  time.Now().Format("2006-01-02 15:04:05"),
	}); err != nil {
		return "", fmt.Errorf("send test email: %w", err)
	}

	return recipientEmail, nil
}
