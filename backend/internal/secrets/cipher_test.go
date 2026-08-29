package secrets

import (
	"strings"
	"testing"
)

func TestCipher_RoundTrip(t *testing.T) {
	t.Parallel()
	c, err := New("correct horse battery staple")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	secret := "s3cret-password-保密"
	stored, err := c.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if !IsEncrypted(stored) {
		t.Errorf("expected stored value to carry the %q marker, got %q", EncryptedPrefix, stored)
	}
	if strings.Contains(stored, "s3cret") {
		t.Errorf("stored value must not contain the plaintext, got %q", stored)
	}

	got, err := c.Decrypt(stored)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if got != secret {
		t.Errorf("round trip mismatch: got %q, want %q", got, secret)
	}
}

func TestCipher_DistinctCiphertextsPerCall(t *testing.T) {
	t.Parallel()
	c, err := New("passphrase")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	first, err := c.Encrypt("same plaintext")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	second, err := c.Encrypt("same plaintext")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if first == second {
		t.Errorf("expected distinct ciphertexts (fresh nonce per call), both were %q", first)
	}
}

func TestCipher_TamperedCiphertextRejected(t *testing.T) {
	t.Parallel()
	c, err := New("passphrase")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	stored, err := c.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// Flip the last character of the base64 payload (part of the GCM tag).
	tampered := stored[:len(stored)-1]
	switch stored[len(stored)-1] {
	case 'A':
		tampered += "B"
	default:
		tampered += "A"
	}

	if _, err := c.Decrypt(tampered); err == nil {
		t.Error("expected an error for tampered ciphertext, got nil")
	}
}

func TestCipher_WrongKeyRejected(t *testing.T) {
	t.Parallel()
	writer, err := New("original-key")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	reader, err := New("rotated-key")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	stored, err := writer.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if _, err := reader.Decrypt(stored); err == nil {
		t.Error("expected an error when decrypting with the wrong key, got nil")
	}
}

func TestCipher_PlaintextPassthrough(t *testing.T) {
	t.Parallel()
	c, err := New("passphrase")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	plain := "stored-before-encryption-was-enabled"
	got, err := c.Decrypt(plain)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if got != plain {
		t.Errorf("expected plaintext passthrough, got %q", got)
	}
}

func TestCipher_EmptyValues(t *testing.T) {
	t.Parallel()
	c, err := New("passphrase")
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	enc, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt(\"\") error: %v", err)
	}
	if enc != "" {
		t.Errorf("Encrypt(\"\") = %q, want \"\"", enc)
	}

	dec, err := c.Decrypt("")
	if err != nil {
		t.Fatalf("Decrypt(\"\") error: %v", err)
	}
	if dec != "" {
		t.Errorf("Decrypt(\"\") = %q, want \"\"", dec)
	}
}

func TestIsEncrypted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"prefixed ciphertext", EncryptedPrefix + "aGVsbG8=", true},
		{"plaintext", "hunter2", false},
		{"empty", "", false},
		{"lookalike without prefix", "enc:v2:aGVsbG8=", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsEncrypted(tt.value); got != tt.want {
				t.Errorf("IsEncrypted(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestNew_EmptyPassphraseRejected(t *testing.T) {
	t.Parallel()
	if _, err := New(""); err == nil {
		t.Error("expected an error for an empty passphrase, got nil")
	}
}
