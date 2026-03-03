package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

// JWTSecret holds the HMAC secret used to sign tokens.
var JWTSecret []byte

// Init loads config values from environment and validates them.
func Init() {
	s := os.Getenv("JWT_SECRET")
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
}
