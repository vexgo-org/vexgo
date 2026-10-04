package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

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
