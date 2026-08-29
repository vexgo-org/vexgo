package app

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
	"github.com/vexgo-org/vexgo/backend/internal/settings"
)

// captureLogs swaps the default slog logger for one writing to a buffer and
// restores it via t.Cleanup.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

// TC-ENC-024: without a configured key no cipher is created and a prominent
// warning naming settings_encryption_key is logged at startup.
func TestInitCipher_NoKeyLogsWarning(t *testing.T) {
	buf := captureLogs(t)

	cipher, err := initCipher(&config.Config{SettingsEncryptionKey: ""})
	if err != nil {
		t.Fatalf("initCipher error: %v", err)
	}
	if cipher != nil {
		t.Error("expected nil cipher when no key is configured")
	}
	if !strings.Contains(buf.String(), "settings_encryption_key") ||
		!strings.Contains(buf.String(), "plaintext") {
		t.Errorf("expected a plaintext-fallback warning naming the option, got:\n%s", buf.String())
	}
}

// With a configured key a usable cipher is created and no warning is logged.
func TestInitCipher_WithKey(t *testing.T) {
	buf := captureLogs(t)

	cipher, err := initCipher(&config.Config{SettingsEncryptionKey: "test-key"})
	if err != nil {
		t.Fatalf("initCipher error: %v", err)
	}
	if cipher == nil {
		t.Fatal("expected a cipher when a key is configured")
	}
	if strings.Contains(buf.String(), "settings_encryption_key") {
		t.Errorf("expected no warning when a key is configured, got:\n%s", buf.String())
	}

	// The returned cipher must be usable.
	stored, err := cipher.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if !secrets.IsEncrypted(stored) {
		t.Errorf("expected encrypted value, got %q", stored)
	}
}

// An invalid key (empty after config resolution cannot happen, but a cipher
// construction failure) must abort startup with an error, not disable
// encryption silently.
func TestInitCipher_RejectsEmptyKeyExplicitly(t *testing.T) {
	// initCipher routes the empty key to the fallback path; New itself must
	// still reject an empty passphrase so a direct caller cannot misconfigure
	// the cipher.
	if _, err := secrets.New(""); err == nil {
		t.Error("expected an error for an empty passphrase, got nil")
	}
}

// Regression: without a key, the value handed to the domains' SecretCipher
// fields must be a true nil interface — a typed nil *secrets.Cipher would
// make the services' `cipher == nil` fallback checks silently pass and panic
// on first use.
func TestInitCipher_NoKeyLeavesDepsCipherNil(t *testing.T) {
	cipher, err := initCipher(&config.Config{SettingsEncryptionKey: ""})
	if err != nil {
		t.Fatalf("initCipher error: %v", err)
	}

	deps := settings.Deps{Cipher: cipher}
	if deps.Cipher != nil {
		t.Error("expected a truly nil SecretCipher in Deps, got a typed-nil interface")
	}
}
