package model

import "testing"

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
