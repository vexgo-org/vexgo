package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// init is called before main to load environment variables from a .env file.
func init() {
	// The path is relative to where the go binary is run.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No .env file found, will use environment variables from the system")
	}
}

// JWTSecret holds the HMAC secret used to sign tokens.
var JWTSecret []byte

// FrontendURL holds the frontend application URL for constructing links.
var FrontendURL string

// Init loads config values from environment and validates them.
// If jwtSecret is provided, it will be used instead of environment variable.
func Init(jwtSecret string) {
	s := jwtSecret
	if s == "" {
		// Fallback to environment variable if no config value provided
		s = os.Getenv("JWT_SECRET")
	}
	if s == "" {
		// Generate a secure random key in the development environment.
		log.Println("WARNING: JWT_SECRET not set — generating a random secret for development")
		key := make([]byte, 32) // 256 bits
		if _, err := rand.Read(key); err != nil {
			log.Fatalf("failed to generate random JWT secret: %v", err)
		}
		s = hex.EncodeToString(key)
	}
	JWTSecret = []byte(s)
	log.Printf("JWT secret initialized with length: %d bytes", len(JWTSecret))

	// Load frontend URL
	FrontendURL = os.Getenv("FRONTEND_URL")
	if FrontendURL == "" {
		FrontendURL = "http://localhost:5173" // Default development environment address
		log.Printf("WARNING: FRONTEND_URL not set — using default: %s", FrontendURL)
	} else {
		log.Printf("Frontend URL set to: %s", FrontendURL)
	}
}
