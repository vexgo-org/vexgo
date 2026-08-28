package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
)

// ComputeJWTSecret ensures cfg.JWTSecret is populated. The secret is normally
// resolved from the config file or JWT_SECRET during Load, like every other
// key; when neither provides one, a random 256-bit key is generated as a
// development fallback. It also applies the FrontendURL development default.
func (cfg *Config) ComputeJWTSecret() error {
	if len(cfg.JWTSecret) > 0 {
		slog.Info("JWT secret loaded from config", "len", len(cfg.JWTSecret))
	} else {
		slog.Warn("JWT secret not set — generating a random secret for development")
		key := make([]byte, 32) // 256 bits
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("generate random JWT secret: %w", err)
		}
		cfg.JWTSecret = []byte(hex.EncodeToString(key))
		slog.Info("JWT secret initialized", "len", len(cfg.JWTSecret))
	}

	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:5173"
		slog.Warn("FRONTEND_URL not set — using default url", "url", cfg.FrontendURL)
	} else {
		slog.Info("frontend URL is set", "url", cfg.FrontendURL)
	}
	return nil
}
