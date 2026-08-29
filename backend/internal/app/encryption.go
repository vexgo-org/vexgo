package app

import (
	"fmt"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
)

// initCipher builds the at-rest cipher from the configured settings
// encryption key. Without a key the secrets stay in plaintext (no-key
// fallback) and a prominent warning is logged once at startup.
func initCipher(cfg *config.Config) (*secrets.Cipher, error) {
	if cfg.SettingsEncryptionKey == "" {
		slog.Warn("settings_encryption_key is not set; SMTP password and AI/comment-moderation API keys will be stored in plaintext")
		return nil, nil
	}

	cipher, err := secrets.New(cfg.SettingsEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("init settings encryption: %w", err)
	}
	return cipher, nil
}
