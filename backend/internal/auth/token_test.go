package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// TestGenerateTokens_CryptoRandomAndPrefixed ensures the emailed account
// tokens (reset / verification / email change) carry high entropy instead of
// the former predictable "userID-nanotime" format, and keep their prefixes so
// token kinds stay distinguishable.
func TestGenerateTokens_CryptoRandomAndPrefixed(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	cases := []struct {
		prefix string
		gen    func(ctx context.Context, userID uint) (string, error)
	}{
		{model.TokenPrefixReset, svc.GeneratePasswordResetToken},
		{model.TokenPrefixVerify, svc.GenerateVerificationToken},
	}

	for _, tc := range cases {
		t1, err := tc.gen(ctx, 1)
		if err != nil {
			t.Fatalf("generate token error: %v", err)
		}
		t2, err := tc.gen(ctx, 1)
		if err != nil {
			t.Fatalf("generate token error: %v", err)
		}

		if !strings.HasPrefix(t1, tc.prefix) {
			t.Errorf("expected prefix %q, got %q", tc.prefix, t1)
		}
		if t1 == t2 {
			t.Errorf("expected two tokens for the same user to differ")
		}
		if len(t1) < len(tc.prefix)+43 { // 43 = base64url length of 32 bytes
			t.Errorf("expected >= 256 bits of entropy, token too short: %q (%d chars)", t1, len(t1))
		}
	}
}

func TestIssueJWT(t *testing.T) {
	u := &model.User{ID: 1, Username: "alice", Role: model.RoleAdmin, PasswordVersion: 2}
	token, err := IssueJWT(u, testJWTSecret)
	if err != nil {
		t.Fatalf("IssueJWT error: %v", err)
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return testJWTSecret, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("expected valid token, got err=%v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["username"] != "alice" || claims["role"] != model.RoleAdmin {
		t.Errorf("unexpected claims: %v", claims)
	}
	if uint(claims["password_version"].(float64)) != 2 {
		t.Errorf("expected password version 2 in claims")
	}
}

// TestTokens_OnlyHashesAreStored ensures the emailed account tokens exist in
// the database only in their hashed storage form: a database leak must not
// expose live one-time tokens, and the raw value must still resolve through
// the hashed lookup.
func TestTokens_OnlyHashesAreStored(t *testing.T) {
	svc, _, db := newTestService(t)
	u := seedUser(t, db, "alice@example.com", "password123", model.RoleGuest, true)

	raw, err := svc.GenerateVerificationToken(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GenerateVerificationToken error: %v", err)
	}

	var stored model.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if stored.VerificationToken == raw {
		t.Error("raw token must not be stored in plaintext")
	}
	if !strings.HasPrefix(stored.VerificationToken, model.TokenPrefixVerify) {
		t.Errorf("expected kind prefix preserved on stored hash, got %q", stored.VerificationToken)
	}
	hashPart := strings.TrimPrefix(stored.VerificationToken, model.TokenPrefixVerify)
	if len(hashPart) != 64 { // hex-encoded SHA-256
		t.Errorf("expected 64-char hex hash, got %d chars", len(hashPart))
	}

	// the raw token still resolves through the hashed lookup
	user, err := svc.repo.FindUserByToken(context.Background(), raw)
	if err != nil {
		t.Fatalf("FindUserByToken with raw token: %v", err)
	}
	if user.ID != u.ID {
		t.Errorf("expected user %d, got %d", u.ID, user.ID)
	}

	// unknown tokens hash to something that matches nothing
	if _, err := svc.repo.FindUserByToken(context.Background(), model.TokenPrefixVerify+"nope"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound for unknown token, got %v", err)
	}
}
