package sso

import (
	"context"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.SSOBinding{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	ssoCfg := &config.SSOConfig{}
	return NewService(Deps{DB: db, SSO: ssoCfg, JWTSecret: []byte("test-secret")}), db
}

func seedUser(t *testing.T, db *gorm.DB, username, email string) model.User {
	return seedUserVerified(t, db, username, email, false)
}

func seedUserVerified(t *testing.T, db *gorm.DB, username, email string, verified bool) model.User {
	t.Helper()
	u := model.User{Username: username, Email: email, Role: model.RoleGuest, EmailVerified: verified}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

func TestFindOrCreateUser_ExactBinding(t *testing.T) {
	svc, db := newTestService(t)
	u := seedUser(t, db, "alice", "alice@example.com")

	if err := db.Create(&model.SSOBinding{
		UserID:     u.ID,
		Provider:   "github",
		ProviderID: "gh-123",
		Email:      "alice@example.com",
		Name:       "Alice",
	}).Error; err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}

	// exact binding match wins, even with a different email
	user, err := svc.FindOrCreateUser(context.Background(), "github", &ssoUserInfo{
		providerID: "gh-123",
		username:   "Alice",
		email:      "changed@example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser error: %v", err)
	}
	if user.ID != u.ID {
		t.Errorf("expected bound user, got id %d", user.ID)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected original email preserved")
	}

	// no duplicate binding or user created
	var bindings int64
	db.Model(&model.SSOBinding{}).Count(&bindings)
	if bindings != 1 {
		t.Errorf("expected 1 binding, got %d", bindings)
	}
}

func TestFindOrCreateUser_EmailMatch(t *testing.T) {
	svc, db := newTestService(t)
	u := seedUserVerified(t, db, "bob", "bob@example.com", true)

	user, err := svc.FindOrCreateUser(context.Background(), "google", &ssoUserInfo{
		providerID:    "g-456",
		username:      "Bobby",
		email:         "bob@example.com",
		emailVerified: true,
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser error: %v", err)
	}
	if user.ID != u.ID {
		t.Errorf("expected email-matched user, got id %d", user.ID)
	}

	// binding persisted for next login
	var binding model.SSOBinding
	if err := db.Where("provider = ? AND provider_id = ?", "google", "g-456").First(&binding).Error; err != nil {
		t.Fatalf("expected binding persisted: %v", err)
	}
	if binding.UserID != u.ID {
		t.Errorf("expected binding to bob")
	}
}

// TestFindOrCreateUser_EmailLinkRequiresVerifiedEmail ensures an SSO identity
// carrying an unverified email cannot link to (and thereby take over) an
// existing local account.
func TestFindOrCreateUser_EmailLinkRequiresVerifiedEmail(t *testing.T) {
	t.Run("provider email unverified", func(t *testing.T) {
		svc, db := newTestService(t)
		seedUserVerified(t, db, "bob", "bob@example.com", true)

		_, err := svc.FindOrCreateUser(context.Background(), "google", &ssoUserInfo{
			providerID:    "g-789",
			username:      "Bobby",
			email:         "bob@example.com",
			emailVerified: false,
		})
		if err == nil {
			t.Fatal("expected error for unverified provider email, got nil")
		}

		// no binding may have been created for the attacker identity
		var bindings int64
		db.Model(&model.SSOBinding{}).Where("provider_id = ?", "g-789").Count(&bindings)
		if bindings != 0 {
			t.Errorf("expected no binding, got %d", bindings)
		}
	})

	t.Run("local account unverified", func(t *testing.T) {
		svc, db := newTestService(t)
		seedUserVerified(t, db, "bob", "bob@example.com", false)

		_, err := svc.FindOrCreateUser(context.Background(), "google", &ssoUserInfo{
			providerID:    "g-990",
			username:      "Bobby",
			email:         "bob@example.com",
			emailVerified: true,
		})
		if err == nil {
			t.Fatal("expected error for unverified local account, got nil")
		}

		var bindings int64
		db.Model(&model.SSOBinding{}).Where("provider_id = ?", "g-990").Count(&bindings)
		if bindings != 0 {
			t.Errorf("expected no binding, got %d", bindings)
		}
	})
}

func TestFindOrCreateUser_AutoRegister(t *testing.T) {
	svc, db := newTestService(t)

	user, err := svc.FindOrCreateUser(context.Background(), "github", &ssoUserInfo{
		providerID: "gh-new",
		username:   "New User",
		email:      "new@example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser error: %v", err)
	}
	if user.Role != model.RoleGuest {
		t.Errorf("expected guest role, got %s", user.Role)
	}
	if user.Username != "NewUser" {
		t.Errorf("expected sanitized username 'NewUser', got %q", user.Username)
	}

	// second login with same identity returns the same user
	again, err := svc.FindOrCreateUser(context.Background(), "github", &ssoUserInfo{
		providerID: "gh-new",
		username:   "New User",
		email:      "new@example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser error: %v", err)
	}
	if again.ID != user.ID {
		t.Errorf("expected same user on second login")
	}

	var users int64
	db.Model(&model.User{}).Count(&users)
	if users != 1 {
		t.Errorf("expected exactly 1 user, got %d", users)
	}
}

func TestFindOrCreateUser_UsernameCollision(t *testing.T) {
	svc, db := newTestService(t)
	seedUser(t, db, "alice", "taken@example.com")

	user, err := svc.FindOrCreateUser(context.Background(), "github", &ssoUserInfo{
		providerID: "gh-x",
		username:   "alice",
		email:      "alice2@example.com",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser error: %v", err)
	}
	if user.Username != "alice1" {
		t.Errorf("expected suffixed username alice1, got %q", user.Username)
	}
	if user.Email != "alice2@example.com" {
		t.Errorf("expected the new email kept")
	}
}

func TestFindOrCreateUser_BindingWithoutUser(t *testing.T) {
	svc, db := newTestService(t)
	// orphan binding pointing at a deleted user
	if err := db.Create(&model.SSOBinding{
		UserID:     99999,
		Provider:   "github",
		ProviderID: "gh-orphan",
	}).Error; err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}

	if _, err := svc.FindOrCreateUser(context.Background(), "github", &ssoUserInfo{providerID: "gh-orphan"}); err == nil {
		t.Errorf("expected error for orphan binding")
	}
}

func TestGenerateUsername_StripsInvalidChars(t *testing.T) {
	svc, _ := newTestService(t)
	name := svc.generateUsername(context.Background(), "héllo wörld!", "x@example.com")
	// only ASCII alphanumerics and underscore survive the sanitizer
	if name != "hllowrld" {
		t.Errorf("expected sanitized username, got %q", name)
	}
	if strings.ContainsAny(name, " !@") {
		t.Errorf("username contains invalid chars: %q", name)
	}
}

func TestProviders(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	cfg := &config.SSOConfig{
		GitHub:          config.SSOProviderConfig{ClientID: "gh-id"},
		AllowLocalLogin: true,
		OIDC: config.OIDCConfig{
			Enabled: true,
			SSOProviderConfig: config.SSOProviderConfig{
				ClientID: "oidc-id",
			},
		},
	}
	svc := NewService(Deps{DB: db, SSO: cfg, JWTSecret: []byte("secret")})

	enabled, allowLocal := svc.Providers()
	if len(enabled) != 2 || enabled[0] != "github" || enabled[1] != "oidc" {
		t.Errorf("expected [github oidc], got %v", enabled)
	}
	if !allowLocal {
		t.Errorf("expected local login allowed by default")
	}

	cfg.AllowLocalLogin = false
	_, allowLocal = svc.Providers()
	if allowLocal {
		t.Errorf("expected local login disabled")
	}
}
