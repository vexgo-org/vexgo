package app

import (
	"fmt"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
)

// secretCipher is the seam the settings/comment/mailer Deps structs accept
// (each declares its own structurally identical SecretCipher interface; any
// implementation of this one satisfies all of them).
type secretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(stored string) (string, error)
}

// initCipher builds the at-rest cipher from the configured settings
// encryption key. Without a key the secrets stay in plaintext (no-key
// fallback) and a prominent warning is logged once at startup. The nil
// return in that case is an untyped nil interface: assigning it to the
// domains' SecretCipher fields keeps them truly nil, so the no-key fallback
// detection in the services works as intended. (Returning a typed nil
// *secrets.Cipher would leak a non-nil interface and break that check.)
func initCipher(cfg *config.Config) (secretCipher, error) {
	if cfg.SettingsEncryptionKey == "" {
		slog.Warn("SMTP password and AI/comment-moderation API keys will be stored in plaintext",
			"reason", "settings_encryption_key is not set")
		return nil, nil
	}

	cipher, err := secrets.New(cfg.SettingsEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("init settings encryption: %w", err)
	}
	return cipher, nil
}
