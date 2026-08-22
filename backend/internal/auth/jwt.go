package auth

import (
	"fmt"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

// IssueJWT signs a JWT for the given user with the provided HMAC secret.
// It is shared by the auth login flow and the sso callback flow.
func IssueJWT(user *model.User, secret []byte) (string, error) {
	claims := jwt.MapClaims{
		"user_id":          user.ID,
		"username":         user.Username,
		"role":             user.Role,
		"password_version": user.PasswordVersion,
		"exp":              time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"jti":              fmt.Sprintf("%d-%s", user.ID, time.Now().Format(time.RFC3339Nano)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}
