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

func TestIsSuperAdmin(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{RoleSuperAdmin, true},
		{RoleAdmin, false},
		{RoleAuthor, false},
		{RoleContributor, false},
		{RoleGuest, false},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := IsSuperAdmin(c.role); got != c.want {
			t.Errorf("IsSuperAdmin(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

func TestIsAdmin(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{RoleSuperAdmin, true},
		{RoleAdmin, true},
		{RoleAuthor, false},
		{RoleContributor, false},
		{RoleGuest, false},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := IsAdmin(c.role); got != c.want {
			t.Errorf("IsAdmin(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

func TestIsAuthor(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{RoleSuperAdmin, true},
		{RoleAdmin, true},
		{RoleAuthor, true},
		{RoleContributor, false},
		{RoleGuest, false},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := IsAuthor(c.role); got != c.want {
			t.Errorf("IsAuthor(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

func TestIsContributor(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{RoleSuperAdmin, true},
		{RoleAdmin, true},
		{RoleAuthor, true},
		{RoleContributor, true},
		{RoleGuest, false},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := IsContributor(c.role); got != c.want {
			t.Errorf("IsContributor(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

func TestValidRole(t *testing.T) {
	for _, role := range []string{
		RoleSuperAdmin,
		RoleAdmin,
		RoleAuthor,
		RoleContributor,
		RoleGuest,
	} {
		if !ValidRole(role) {
			t.Errorf("ValidRole(%q) = false, want true", role)
		}
	}
	for _, role := range []string{"", "unknown", "ADMIN", "super_admin "} {
		if ValidRole(role) {
			t.Errorf("ValidRole(%q) = true, want false", role)
		}
	}
}
