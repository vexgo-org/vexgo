package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"vexgo/backend/cmd"
	"vexgo/backend/model"

	"github.com/sirupsen/logrus"

	"github.com/glebarez/sqlite"
	dmsql "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// InitDB initializes the database connection based on configuration
func InitDB(cfg *cmd.Config, dataDir string) {
	// Determine database type: config file takes priority, fallback to environment variable
	dbType := cfg.DBType
	if dbType == "" {
		dbType = os.Getenv("DB_TYPE")
	}
	var err error

	if dbType == "mysql" {
		// MySQL connection - use config values with environment fallback
		user := cfg.DBUser
		if user == "" {
			user = os.Getenv("DB_USER")
		}
		password := cfg.DBPassword
		if password == "" {
			password = os.Getenv("DB_PASSWORD")
		}
		host := cfg.DBHost
		if host == "" {
			host = os.Getenv("DB_HOST")
		}
		port := cfg.DBPort
		if port == 0 {
			// If port not set in config, get from env
			portStr := os.Getenv("DB_PORT")
			if portStr != "" {
				fmt.Sscanf(portStr, "%d", &port)
			} else {
				port = 3306 // default MySQL port
			}
		}
		dbname := cfg.DBName
		if dbname == "" {
			dbname = os.Getenv("DB_NAME")
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, dbname)

		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			// Check if the error is "Unknown database" (error code 1049)
			if mysqlErr, ok := err.(*dmsql.MySQLError); ok && mysqlErr.Number == 1049 {
				logrus.Printf("Database '%s' not found, attempting to create it.", dbname)

				// DSN without database name to connect to the server
				serverDsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, port)
				serverDb, serverErr := gorm.Open(mysql.Open(serverDsn), &gorm.Config{})
				if serverErr != nil {
					logrus.Fatalf("failed to connect to MySQL server to create database: %v", serverErr)
				}

				// Create the database
				createDbSQL := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbname)
				if execErr := serverDb.Exec(createDbSQL).Error; execErr != nil {
					logrus.Fatalf("failed to create database '%s': %v", dbname, execErr)
				}
				logrus.Printf("Database '%s' created successfully.", dbname)

				// Re-attempt connection to the newly created database
				db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
				if err != nil {
					logrus.Fatalf("failed to connect to newly created MySQL database: %v", err)
				}
			} else {
				logrus.Fatalf("failed to connect to MySQL database: %v", err)
			}
		}
		logrus.Println("Successfully connected to MySQL database")
	} else if dbType == "postgres" {
		// PostgreSQL connection - use config values with environment fallback
		user := cfg.DBUser
		if user == "" {
			user = os.Getenv("DB_USER")
		}
		password := cfg.DBPassword
		if password == "" {
			password = os.Getenv("DB_PASSWORD")
		}
		host := cfg.DBHost
		if host == "" {
			host = os.Getenv("DB_HOST")
		}
		port := cfg.DBPort
		if port == 0 {
			// If port not set in config, get from env
			portStr := os.Getenv("DB_PORT")
			if portStr != "" {
				fmt.Sscanf(portStr, "%d", &port)
			} else {
				port = 5432 // default PostgreSQL port
			}
		}
		dbname := cfg.DBName
		if dbname == "" {
			dbname = os.Getenv("DB_NAME")
		}
		sslMode := cfg.DBSSLMode
		if sslMode == "" {
			sslMode = os.Getenv("DB_SSL_MODE")
			if sslMode == "" {
				sslMode = "disable" // default SSL mode
			}
		}

		// Build DSN for PostgreSQL
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslMode)

		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			logrus.Fatalf("failed to connect to PostgreSQL database: %v", err)
		}
		logrus.Println("Successfully connected to PostgreSQL database")
	} else {
		// SQLite connection (default)
		// Use dataDir from config (already set via command line or config file)
		if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
			logrus.Fatalf("failed to create data directory: %v", err)
		}
		dbPath := filepath.Join(dataDir, "blog.db")
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			logrus.Fatalf("failed to connect to SQLite database: %v", err)
		}
		logrus.Println("Successfully connected to SQLite database")
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&model.Post{},
		&model.User{},
		&model.Tag{},
		&model.Category{},
		&model.Comment{},
		&model.Like{},
		&model.MediaFile{},
		&model.SMTPConfig{},
		&model.Captcha{},
		&model.GeneralSettings{},
		&model.CommentModerationConfig{},
		&model.AIConfig{},
		&model.SSOBinding{},
		&model.ThemeConfig{},
		&model.Message{},
		&model.Notification{},
	); err != nil {
		logrus.Fatalf("auto migrate failed: %v", err)
	}

	// Create a default super admin (if not exists), store password using bcrypt
	var u model.User
	if err := db.Where("username = ?", "admin").First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pwHash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
			if err != nil {
				logrus.Printf("failed to hash default admin password: %v", err)
			} else {
				u = model.User{
					Username:          "admin",
					Email:             "admin@example.com",
					Password:          string(pwHash),
					Role:              "super_admin",
					EmailVerified:     true,
					VerificationToken: "",
					TokenExpiresAt:    nil,
				}
				if err := db.Create(&u).Error; err != nil {
					logrus.Printf("failed to create default admin: %v", err)
				}
			}

			// Create default SMTP configuration (if not exists)
			var smtpConfig model.SMTPConfig
			if err := db.First(&smtpConfig).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					smtpConfig = model.SMTPConfig{
						Enabled:   false,
						Host:      "",
						Port:      587,
						Username:  "",
						Password:  "",
						FromEmail: "",
						FromName:  "VexGo",
					}
					if err := db.Create(&smtpConfig).Error; err != nil {
						logrus.Printf("failed to create default smtp config: %v", err)
					} else {
						logrus.Println("default smtp config created")
					}
				}
			}

			// Create default general settings (if not exists)
			var generalSettings model.GeneralSettings
			if err := db.First(&generalSettings).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					generalSettings = model.GeneralSettings{
						CaptchaEnabled:      false, // Disable captcha by default
						RegistrationEnabled: true,  // Allow registration by default
						SiteName:            "VexGo",
						SiteDescription:     "",
						ItemsPerPage:        20,
					}
					if err := db.Create(&generalSettings).Error; err != nil {
						logrus.Printf("failed to create default general settings: %v", err)
					} else {
						logrus.Println("default general settings created")
					}
				}
			}
		}
	}

	// Create default AI configuration (if not exists)
	var aiConfig model.AIConfig
	if err := db.First(&aiConfig).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			aiConfig = model.AIConfig{
				Enabled:     false,
				Provider:    "openai",
				ApiEndpoint: "",
				ApiKey:      "",
				ModelName:   "gpt-3.5-turbo",
			}
			if err := db.Create(&aiConfig).Error; err != nil {
				logrus.Printf("failed to create default ai config: %v", err)
			} else {
				logrus.Println("default ai config created")
			}
		}
	}

	// Create default theme configuration (if not exists)
	var themeConfig model.ThemeConfig
	if err := db.First(&themeConfig).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			themeConfig = model.ThemeConfig{
				ActiveTheme: "default",
			}
			if err := db.Create(&themeConfig).Error; err != nil {
				logrus.Printf("failed to create default theme config: %v", err)
			} else {
				logrus.Println("default theme config created")
			}
		}
	}

	// Create a default category (if not exists)
	var defaultCategory model.Category
	if err := db.Where("name = ?", "Default").First(&defaultCategory).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			defaultCategory = model.Category{
				Name:        "Default",
				Description: "Default category for articles without a specified category",
			}
			if err := db.Create(&defaultCategory).Error; err != nil {
				logrus.Printf("failed to create default category: %v", err)
			} else {
				logrus.Println("default category created: Default")
			}
		}
	}
	// Set up the theme provider so the public package can read the active theme from DB
	SetupThemeProvider()
}

// DB returns the database instance
func DB() *gorm.DB {
	return db
}

// SetDB sets the database instance
func SetDB(database *gorm.DB) {
	db = database
}
