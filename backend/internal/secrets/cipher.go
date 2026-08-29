// Package secrets provides authenticated encryption at rest for secrets
// stored in the database (SMTP password, AI and comment-moderation API keys).
//
// Values are encrypted with AES-256-GCM under a key derived from a
// operator-configured passphrase (the settings_encryption_key config option)
// with scrypt and a fixed application-level salt constant. A fixed salt is
// acceptable here because the input is a high-entropy operator-chosen key and
// the threat model is exposure of a DB dump or backup, not of this binary.
//
// Encrypted values carry a versioned marker so ciphertext and plaintext can be
// told apart on disk: "enc:v1:" + base64(nonce || ciphertext+tag). Decrypt
// passes values without the marker through unchanged, so reading a database
// that still holds plaintext (or that was written while no key was configured)
// keeps working.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// EncryptedPrefix marks a stored value as AES-256-GCM ciphertext produced by
// this package. It doubles as the plaintext/ciphertext discriminator used by
// the startup migration and by the no-key plaintext fallback.
const EncryptedPrefix = "enc:v1:"

// keyDerivationSalt is the fixed application-level salt for scrypt. The input
// is a high-entropy operator-chosen passphrase, so a per-installation salt is
// not required; a constant keeps derivations reproducible across processes.
var keyDerivationSalt = []byte("vexgo:secrets:v1:scrypt")

// Cipher encrypts and decrypts secrets at rest. The zero value is not usable;
// create instances with New.
type Cipher struct {
	gcm cipher.AEAD
}

// New derives the encryption key from the given passphrase and returns a
// ready-to-use Cipher. An empty passphrase is rejected so a misconfigured
// deployment cannot silently disable encryption.
func New(passphrase string) (*Cipher, error) {
	if passphrase == "" {
		return nil, errors.New("secrets: passphrase must not be empty")
	}

	key, err := scrypt.Key([]byte(passphrase), keyDerivationSalt, 32768, 8, 1, 32)
	if err != nil {
		return nil, fmt.Errorf("secrets: derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: create GCM: %w", err)
	}

	return &Cipher{gcm: gcm}, nil
}

// IsEncrypted reports whether the stored value carries the encrypted marker
// and therefore needs decryption before use.
func IsEncrypted(stored string) bool {
	return strings.HasPrefix(stored, EncryptedPrefix)
}

// Encrypt seals the plaintext under the cipher key and returns
// "enc:v1:" + base64(nonce || ciphertext+tag). Empty values are passed through
// so "secret not set" stays distinguishable from an empty decrypted value.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: generate nonce: %w", err)
	}

	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt opens a stored value. Values without the EncryptedPrefix marker are
// passed through unchanged (plaintext tolerance); anything carrying the marker
// must decrypt and authenticate, otherwise an error is returned so callers can
// treat the secret as unset.
func (c *Cipher) Decrypt(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !IsEncrypted(stored) {
		return stored, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, EncryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("secrets: decode encrypted value: %w", err)
	}

	nonceSize := c.gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("secrets: encrypted value too short")
	}

	plaintext, err := c.gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("secrets: decrypt value: %w", err)
	}

	return string(plaintext), nil
}
