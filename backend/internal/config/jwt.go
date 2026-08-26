package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
)

// ComputeJWTSecret ensures cfg.JWTSecret is populated. It checks, in order:
//   - cfg.JWTSecret already set (from config file or flag)
//   - JWT_SECRET environment variable
//   - random 256-bit key (development fallback)
func (cfg *Config) ComputeJWTSecret() {
	if len(cfg.JWTSecret) > 0 {
		slog.Info("JWT secret loaded from config", "len", len(cfg.JWTSecret))
		return
	}

	s := os.Getenv("JWT_SECRET")
	if s == "" {
		slog.Warn("JWT_SECRET not set — generating a random secret for development")
		key := make([]byte, 32) // 256 bits
		if _, err := rand.Read(key); err != nil {
			slog.Error("failed to generate random JWT secret", "err", err)
			os.Exit(1)
		}
		s = hex.EncodeToString(key)
	}
	cfg.JWTSecret = []byte(s)
	slog.Info("JWT secret initialized", "len", len(cfg.JWTSecret))

	// Load frontend URL
	cfg.FrontendURL = os.Getenv("FRONTEND_URL")
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:5173"
		slog.Warn("FRONTEND_URL not set — using default url", "url", cfg.FrontendURL)
	} else {
		slog.Info("frontend URL is set", "url", cfg.FrontendURL)
	}
}
