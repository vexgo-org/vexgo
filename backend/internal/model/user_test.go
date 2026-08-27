package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Live emailed-link tokens must never leave the server through a serialized
// model.User: instances are rendered raw by several endpoints, and a leaked
// reset/verification token allows account takeover.
func TestUserJSONOmitsTokenFields(t *testing.T) {
	expires := time.Now().Add(5 * time.Minute)
	u := User{
		ID:                1,
		Username:          "alice",
		Email:             "alice@example.com",
		VerificationToken: "reset-1-123",
		TokenExpiresAt:    &expires,
		PendingEmail:      "new@example.com",
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, leak := range []string{"verification_token", "token_expires_at", "pending_email", "reset-1-123"} {
		if strings.Contains(s, leak) {
			t.Errorf("serialized user leaks %q: %s", leak, s)
		}
	}

	// Sanity: the response fields clients do rely on survive.
	if !strings.Contains(s, `"email_verified"`) || !strings.Contains(s, `"email"`) {
		t.Errorf("expected client-visible fields to remain, got %s", s)
	}

	var roundTrip User
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundTrip.VerificationToken != "" || roundTrip.PendingEmail != "" || roundTrip.TokenExpiresAt != nil {
		t.Error("token fields must not survive a JSON round-trip")
	}
}
