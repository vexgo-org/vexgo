package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// Config holds the server configuration from command line arguments and/or config file
type Config struct {
	Addr    string // Address to listen on (e.g., "0.0.0.0" or "127.0.0.1")
	Port    int    // Port to listen on
	DataDir string // Data directory for storing sqlite database and media files

	// Database configuration
	DBType     string `yaml:"db_type"`     // Database type: "sqlite", "mysql", or "postgres"
	DBHost     string `yaml:"db_host"`     // Database host
	DBPort     int    `yaml:"db_port"`     // Database port
	DBUser     string `yaml:"db_user"`     // Database user
	DBPassword string `yaml:"db_password"` // Database password
	DBName     string `yaml:"db_name"`     // Database name
	DBSSLMode  string `yaml:"db_ssl_mode"` // PostgreSQL SSL mode (for postgres)
}

// ParseFlags parses command line flags and returns the server configuration
func ParseFlags() *Config {
	configFile := flag.String("c", "", "Path to configuration file (YAML format)")
	addr := flag.String("addr", "", "Address to listen on")
	port := flag.Int("port", 0, "Port to listen on")
	dataDir := flag.String("data", "", "Data directory for storing sqlite database and media files")

	// Parse command line flags
	flag.Parse()

	// Default configuration with environment variable fallback
	cfg := &Config{
		Addr:    getEnvOrDefault("ADDR", *addr, "0.0.0.0"),
		Port:    getIntEnvOrDefault("PORT", *port, 3001),
		DataDir: getEnvOrDefault("DATA_DIR", *dataDir, "./data"),
	}

	// If config file is specified, load it (overrides env and defaults, but not command line flags)
	if *configFile != "" {
		if err := loadConfigFile(*configFile, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load config file %s: %v\n", *configFile, err)
		} else {
			fmt.Printf("Loaded configuration from %s\n", *configFile)
		}
	}

	// Apply environment variable overrides for database config (only if not set by config file or command line)
	applyEnvOverrides(cfg)

	return cfg
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, value, defaultValue string) string {
	if value != "" {
		return value // command line flag takes precedence
	}
	if env := os.Getenv(key); env != "" {
		return env // environment variable next
	}
	return defaultValue // finally use default
}

// getIntEnvOrDefault gets integer environment variable or returns default value
func getIntEnvOrDefault(key string, value, defaultValue int) int {
	if value != 0 {
		return value // command line flag takes precedence
	}
	if env := os.Getenv(key); env != "" {
		var result int
		if _, err := fmt.Sscanf(env, "%d", &result); err == nil {
			return result // environment variable next
		}
	}
	return defaultValue // finally use default
}

// applyEnvOverrides applies environment variable overrides for database configuration
// Only applies if the config field is empty (not set by config file or command line)
func applyEnvOverrides(cfg *Config) {
	// Database type
	if cfg.DBType == "" {
		if env := os.Getenv("DB_TYPE"); env != "" {
			cfg.DBType = env
		}
	}

	// Database host
	if cfg.DBHost == "" {
		if env := os.Getenv("DB_HOST"); env != "" {
			cfg.DBHost = env
		}
	}

	// Database port
	if cfg.DBPort == 0 {
		if env := os.Getenv("DB_PORT"); env != "" {
			fmt.Sscanf(env, "%d", &cfg.DBPort)
		}
	}

	// Database user
	if cfg.DBUser == "" {
		if env := os.Getenv("DB_USER"); env != "" {
			cfg.DBUser = env
		}
	}

	// Database password
	if cfg.DBPassword == "" {
		if env := os.Getenv("DB_PASSWORD"); env != "" {
			cfg.DBPassword = env
		}
	}

	// Database name
	if cfg.DBName == "" {
		if env := os.Getenv("DB_NAME"); env != "" {
			cfg.DBName = env
		}
	}

	// PostgreSQL SSL mode
	if cfg.DBSSLMode == "" {
		if env := os.Getenv("DB_SSL_MODE"); env != "" {
			cfg.DBSSLMode = env
		}
	}
}

// loadConfigFile loads configuration from a YAML file
// It only fills empty fields, preserving values already set (e.g., from command line)
func loadConfigFile(filename string, cfg *Config) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal YAML into a temporary struct to avoid overwriting existing values
	var temp Config
	if err := yaml.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Only apply values from config file if the field is currently empty (zero value)
	// This preserves command line flag values which have highest priority
	if cfg.Addr == "" || cfg.Addr == "0.0.0.0" {
		if temp.Addr != "" {
			cfg.Addr = temp.Addr
		}
	}
	if cfg.Port == 0 {
		if temp.Port != 0 {
			cfg.Port = temp.Port
		}
	}
	if cfg.DataDir == "" || cfg.DataDir == "./data" {
		if temp.DataDir != "" {
			cfg.DataDir = temp.DataDir
		}
	}
	if cfg.DBType == "" {
		cfg.DBType = temp.DBType
	}
	if cfg.DBHost == "" {
		cfg.DBHost = temp.DBHost
	}
	if cfg.DBPort == 0 {
		cfg.DBPort = temp.DBPort
	}
	if cfg.DBUser == "" {
		cfg.DBUser = temp.DBUser
	}
	if cfg.DBPassword == "" {
		cfg.DBPassword = temp.DBPassword
	}
	if cfg.DBName == "" {
		cfg.DBName = temp.DBName
	}
	if cfg.DBSSLMode == "" {
		cfg.DBSSLMode = temp.DBSSLMode
	}

	return nil
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
