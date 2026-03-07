package cmd

import (
	"flag"
	"fmt"
	"os"
)

// Config holds the server configuration from command line arguments
type Config struct {
	Addr string // Address to listen on (e.g., "0.0.0.0" or "127.0.0.1")
	Port int    // Port to listen on
}

// ParseFlags parses command line flags and returns the server configuration
func ParseFlags() *Config {
	addr := flag.String("addr", "0.0.0.0", "Address to listen on (default: 0.0.0.0)")
	port := flag.Int("port", 3001, "Port to listen on (default: 3001)")

	// Parse command line flags
	flag.Parse()

	return &Config{
		Addr: *addr,
		Port: *port,
	}
}

// GetListenAddr returns the full listen address in the format "addr:port"
func (c *Config) GetListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Addr, c.Port)
}

// PrintUsage prints usage information for the server command
func PrintUsage() {
	fmt.Printf("Usage: %s [options]\n", os.Args[0])
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
}
