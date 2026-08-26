package mailer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/wneessen/go-mail"
)

type SMTPMailer interface {
	LoadConfig(cfg *model.SMTPConfig) error
	Send(ctx context.Context, msg Message) error
	Enabled() bool
}

// SMTPClient encapsulate `mail.Client` and store SMTP configuration.
type SMTPClient struct {
	mu     sync.Mutex
	c      *mail.Client
	config *model.SMTPConfig
}

// Message stores mail message in a structural way.
type Message struct {
	To       []string
	Subject  string
	TextBody string
	HTMLBody string
}

// NewClient creates a new `SMTPClient` from a given configuration.
func NewClient() SMTPMailer {
	return &SMTPClient{}
}

// LoadConfig updates the configuration of SMTP client.
func (c *SMTPClient) LoadConfig(cfg *model.SMTPConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	client, err := createClientFromConfig(cfg)
	if err != nil {
		return err
	}

	c.c = client
	c.config = cfg
	return nil
}

// Send sends a message to recipient(s).
func (c *SMTPClient) Send(ctx context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateMessage(msg); err != nil {
		return err
	}

	message := mail.NewMsg()

	// Construct `from` field of a mail.
	// If `FromName` is configured, the format will be
	// FromName <FromEmail>
	if c.config.FromName != "" {
		if err := message.FromFormat(
			c.config.FromName,
			c.config.FromEmail,
		); err != nil {
			return fmt.Errorf("set sender failed: %w", err)
		}
	} else {
		if err := message.From(c.config.FromEmail); err != nil {
			return fmt.Errorf("set sender failed: %w", err)
		}
	}

	if err := message.To(msg.To...); err != nil {
		return fmt.Errorf("set recipients failed: %w", err)
	}

	message.Subject(msg.Subject)

	// Set mail body.
	switch {
	case msg.TextBody != "" && msg.HTMLBody != "":
		message.SetBodyString(mail.TypeTextPlain, msg.TextBody)
		message.AddAlternativeString(mail.TypeTextHTML, msg.HTMLBody)

	case msg.HTMLBody != "":
		message.SetBodyString(mail.TypeTextHTML, msg.HTMLBody)

	default:
		message.SetBodyString(mail.TypeTextPlain, msg.TextBody)
	}

	// Send the message.
	if err := c.c.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("send mail failed: %w", err)
	}

	return nil
}

func (c *SMTPClient) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.config != nil && c.config.Enabled
}

// createClientFromConfig is a helper function used to create `mail.Client`
// from a given `SMTPClientConfig`.
func createClientFromConfig(cfg *model.SMTPConfig) (*mail.Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	client, err := mail.NewClient(
		cfg.Host,
		mail.WithPort(cfg.Port),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(cfg.Password),
		mail.WithTLSPolicy(mail.TLSOpportunistic),
	)
	if err != nil {
		return nil, fmt.Errorf("create SMTP client failed: %w", err)
	}

	return client, nil
}

// validateConfig validates the configuration of SMTP client.
func validateConfig(cfg *model.SMTPConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("SMTP host is required")
	}

	if cfg.Port <= 0 {
		return errors.New("SMTP port must be greater than zero")
	}

	if strings.TrimSpace(cfg.FromEmail) == "" {
		return errors.New("sender address is required")
	}

	return nil
}

// validateMessage validates the message to be sent.
func validateMessage(msg Message) error {
	if len(msg.To) == 0 {
		return errors.New("at least one recipient is required")
	}

	if strings.TrimSpace(msg.Subject) == "" {
		return errors.New("mail subject is required")
	}

	if msg.TextBody == "" && msg.HTMLBody == "" {
		return errors.New("mail body is required")
	}

	return nil
}
