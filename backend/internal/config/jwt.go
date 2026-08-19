package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// loadDotEnv loads environment variables from a .env file (best-effort).
// It is called at the start of ParseFlags so that env vars are available
// before config construction.
func loadDotEnv() {
	if err := godotenv.Load("../.env"); err != nil {
		logrus.Info("No .env file found, will use environment variables from the system")
	}
}

// ComputeJWTSecret ensures cfg.JWTSecret is populated. It checks, in order:
//   - cfg.JWTSecret already set (from config file or flag)
//   - JWT_SECRET environment variable
//   - random 256-bit key (development fallback)
func (cfg *Config) ComputeJWTSecret() {
	if len(cfg.JWTSecret) > 0 {
		logrus.Infof("JWT secret loaded from config (length: %d bytes)", len(cfg.JWTSecret))
		return
	}

	s := os.Getenv("JWT_SECRET")
	if s == "" {
		logrus.Warn("JWT_SECRET not set — generating a random secret for development")
		key := make([]byte, 32) // 256 bits
		if _, err := rand.Read(key); err != nil {
			logrus.Fatalf("failed to generate random JWT secret: %v", err)
		}
		s = hex.EncodeToString(key)
	}
	cfg.JWTSecret = []byte(s)
	logrus.Infof("JWT secret initialized (length: %d bytes)", len(cfg.JWTSecret))

	// Load frontend URL
	cfg.FrontendURL = os.Getenv("FRONTEND_URL")
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:5173"
		logrus.Warnf("FRONTEND_URL not set — using default: %s", cfg.FrontendURL)
	} else {
		logrus.Infof("Frontend URL set to: %s", cfg.FrontendURL)
	}
}
