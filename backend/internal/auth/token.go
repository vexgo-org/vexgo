package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// GeneratePasswordResetToken generates password reset token
func (s *Service) GeneratePasswordResetToken(ctx context.Context, userID uint) (string, error) {
	token, err := generateSecureToken(model.TokenPrefixReset)
	if err != nil {
		return "", fmt.Errorf("failed to generate password reset token: %w", err)
	}

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	if err := s.repo.UpdateUserToken(ctx, userID, tokenStorageForm(token), expiresAt); err != nil {
		return "", fmt.Errorf("failed to save password reset token: %w", err)
	}

	return token, nil
}

// GenerateEmailChangeToken generates email change verification token
func (s *Service) GenerateEmailChangeToken(ctx context.Context, userID uint, newEmail string) (string, error) {
	token, err := generateSecureToken(model.TokenPrefixEmailChange)
	if err != nil {
		return "", fmt.Errorf("failed to generate email change token: %w", err)
	}

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database, also store pending new email
	if err := s.repo.UpdateEmailChangeToken(ctx, userID, newEmail, tokenStorageForm(token), expiresAt); err != nil {
		return "", fmt.Errorf("failed to update email change token: %w", err)
	}

	return token, nil
}

func (s *Service) GenerateVerificationToken(ctx context.Context, userID uint) (string, error) {
	token, err := generateSecureToken(model.TokenPrefixVerify)
	if err != nil {
		return "", fmt.Errorf("failed to generate verification token: %w", err)
	}

	// Calculate expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Save to database
	if err := s.repo.UpdateUserToken(ctx, userID, tokenStorageForm(token), expiresAt); err != nil {
		return "", fmt.Errorf("failed to save verification token: %w", err)
	}

	return token, nil
}

func buildLinkWithToken(protocol, host, path, token string) string {
	u := url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   path,
		RawQuery: url.Values{
			"token": []string{token},
		}.Encode(),
	}
	return u.String()
}

// secureTokenEntropy is the number of random bytes in every emailed account
// token (reset, verification, email change): 32 bytes give 256 bits, far above
// the 128-bit floor for unguessable one-time tokens.
const secureTokenEntropy = 32

// generateSecureToken returns prefix + 256 bits of crypto/rand entropy encoded
// as unpadded base64url. The token is emailed in this raw form; only its
// storage form (see tokenStorageForm) is persisted.
func generateSecureToken(prefix string) (string, error) {
	b := make([]byte, secureTokenEntropy)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// tokenStorageForm maps a raw emailed token to its at-rest representation:
// the kind prefix followed by the SHA-256 of the rest. Only this form is
// persisted, so a database leak does not expose live one-time tokens, and
// lookups compare hashes instead of plaintext. The prefix is preserved so the
// token-kind checks on stored values (e.g. the verification-resend cooldown)
// keep working. Unknown tokens hash in full (empty prefix) and match nothing.
func tokenStorageForm(rawToken string) string {
	prefix := tokenKindPrefix(rawToken)
	sum := sha256.Sum256([]byte(rawToken[len(prefix):]))
	return prefix + hex.EncodeToString(sum[:])
}

// tokenKindPrefix returns the known account-token prefix of a raw token, or
// "" for unrecognized input. Prefixes are matched longest-first because
// TokenPrefixEmailChange contains an inner hyphen.
func tokenKindPrefix(rawToken string) string {
	for _, prefix := range []string{model.TokenPrefixEmailChange, model.TokenPrefixReset, model.TokenPrefixVerify} {
		if strings.HasPrefix(rawToken, prefix) {
			return prefix
		}
	}
	return ""
}
