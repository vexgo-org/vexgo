// Package settings implements the admin configuration domain: SMTP, AI,
// general site settings and theme configuration.
package settings

import (
	"fmt"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/mailer"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"

	"gorm.io/gorm"
)

// Deps holds the dependencies required by the settings domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Themes    ThemeManager
	Mailer    *mailer.Service
	Cipher    secrets.SecretCipher
}

// Service contains the business logic of the settings domain.
type Service struct {
	repo   Repository
	themes ThemeManager
	mailer *mailer.Service
	cipher secrets.SecretCipher
}

// NewService creates a settings service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), themes: deps.Themes, mailer: deps.Mailer, cipher: deps.Cipher}
}

// encryptSecret encrypts a secret before it is stored. Empty values are passed
// through, and without a configured cipher the plaintext is stored as-is
// (no-key fallback). Errors are wrapped with the setting name so the admin
// can tell which secret failed to save.
func (s *Service) encryptSecret(value, setting string) (string, error) {
	if value == "" || s.cipher == nil {
		return value, nil
	}
	encrypted, err := s.cipher.Encrypt(value)
	if err != nil {
		return "", fmt.Errorf("encrypt %s: %w", setting, err)
	}
	return encrypted, nil
}

// decryptSecret decrypts a stored secret for internal use. Values without the
// encrypted marker are passed through unchanged. When decryption fails (e.g.
// the key was rotated) the secret is treated as unset and an error naming the
// affected setting is logged; the admin must re-save the secret.
func (s *Service) decryptSecret(stored, setting string) string {
	if stored == "" || s.cipher == nil {
		return stored
	}
	decrypted, err := s.cipher.Decrypt(stored)
	if err != nil {
		slog.Error("failed to decrypt stored secret, treating it as unset; please re-save it", "setting", setting, "err", err)
		return ""
	}
	return decrypted
}
