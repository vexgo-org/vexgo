// Package database handles the database connection, schema migration and
// default seed data. It is the single source of the *gorm.DB instance which is
// injected into every domain package via constructors.
package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/vexgo-org/vexgo/backend/internal/config"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	dmsql "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open establishes the database connection based on configuration.
// Behavior mirrors the previous handler.InitDB connection logic.
func Open(cfg *config.Config, dataDir string) (*gorm.DB, error) {
	// Determine database type: config file takes priority, fallback to environment variable
	dbType := cfg.DBType
	if dbType == "" {
		dbType = os.Getenv("DB_TYPE")
	}

	switch dbType {
	case "mysql":
		// MySQL connection - use config values with environment fallback
		return openMySQL(cfg)
	case "postgres":
		// PostgreSQL connection - use config values with environment fallback
		return openPostgres(cfg)
	default:
		// SQLite connection (default)
		return openSQLite(dataDir)
	}
}

// openMySQL connects to a MySQL database, creating it if it does not exist.
func openMySQL(cfg *config.Config) (*gorm.DB, error) {
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
		port = 3306
		// If port not set in config, get from env
		if portStr := os.Getenv("DB_PORT"); portStr != "" {
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
				slog.Warn("invalid DB_PORT using default MySQL port", "port", portStr, "fallback", 3306)
				port = 3306
			}
		}
	}
	dbname := cfg.DBName
	if dbname == "" {
		dbname = os.Getenv("DB_NAME")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		// Check if the error is "Unknown database" (error code 1049)
		if mysqlErr, ok := err.(*dmsql.MySQLError); ok && mysqlErr.Number == 1049 {
			slog.Info("database not found, attempting to create it", "dbname", dbname)
			// DSN without database name to connect to the server
			serverDsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, port)
			serverDb, serverErr := gorm.Open(mysql.Open(serverDsn), &gorm.Config{})
			if serverErr != nil {
				return nil, fmt.Errorf("connect to MySQL server to create database: %w", serverErr)
			}
			// Create the database
			createDbSQL := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbname)
			if execErr := serverDb.Exec(createDbSQL).Error; execErr != nil {
				return nil, fmt.Errorf("create database '%s': %w", dbname, execErr)
			}
			slog.Info("database created successfully", "dbname", dbname)
			// Re-attempt connection to the newly created database
			db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
			if err != nil {
				return nil, fmt.Errorf("connect to newly created MySQL database: %w", err)
			}
		} else {
			return nil, fmt.Errorf("connect to MySQL database: %w", err)
		}
	}
	slog.Info("successfully connected to MySQL database")
	return db, nil
}

// openPostgres connects to a PostgreSQL database.
func openPostgres(cfg *config.Config) (*gorm.DB, error) {
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
		port = 5432
		if portStr := os.Getenv("DB_PORT"); portStr != "" {
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
				slog.Warn("invalid DB_PORT, using default PostgreSQL port", "port", portStr, "fallback", 5432)
				port = 5432
			}
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
			sslMode = "disable"
		}
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL database: %w", err)
	}
	slog.Info("successfully connected to PostgreSQL database")
	return db, nil
}

// openSQLite connects to an SQLite database stored in dataDir.
func openSQLite(dataDir string) (*gorm.DB, error) {
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	dbPath := filepath.Join(dataDir, "blog.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect to SQLite database: %w", err)
	}
	slog.Info("successfully connected to SQLite database")
	return db, nil
}

// AutoMigrate creates or updates the database schema for all models.
func AutoMigrate(db *gorm.DB) error {
	// Remove duplicate likes (same post+user) before the composite unique
	// index is created, so pre-existing duplicate data cannot break the
	// migration. On a fresh database the table does not exist yet and this
	// is a no-op.
	if db.Migrator().HasTable(&model.Like{}) {
		if err := db.Exec(
			"DELETE FROM likes WHERE id NOT IN (SELECT MIN(id) FROM likes GROUP BY post_id, user_id)",
		).Error; err != nil {
			return fmt.Errorf("deduplicate likes: %w", err)
		}
	}

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
		&model.Notification{},
		&model.CreatorApplication{},
	); err != nil {
		return err
	}

	// Backfill slugs for existing posts that don't have one yet.
	return backfillSlugs(db)
}

// backfillSlugs generates unique slugs for existing posts that have an empty
// slug column, deriving them from each post's title.
func backfillSlugs(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Post{}) {
		return nil
	}

	var posts []model.Post
	if err := db.Where("slug = '' OR slug IS NULL").Find(&posts).Error; err != nil {
		return fmt.Errorf("backfill slugs: find empty: %w", err)
	}
	if len(posts) == 0 {
		return nil
	}

	for _, post := range posts {
		base := model.SlugFromTitle(post.Title)
		if base == "" {
			base = "post"
		}
		slug := base
		counter := 1
		for {
			// Check for collision with an existing slug in the database.
			var existing model.Post
			err := db.Where("slug = ? AND id != ?", slug, post.ID).First(&existing).Error
			if err != nil {
				break // no collision
			}
			counter++
			slug = fmt.Sprintf("%s-%d", base, counter)
		}
		if err := db.Model(&post).Update("slug", slug).Error; err != nil {
			return fmt.Errorf("backfill slugs: update post %d: %w", post.ID, err)
		}
	}

	return nil
}

// defaultAdminUsername is the login name of the seeded super admin account.
const (
	defaultAdminUsername = "admin"
	defaultAdminPassword = "password"
)

// Seed inserts default records (admin user, SMTP/general/AI/theme settings,
// default category) if they do not already exist.
func Seed(db *gorm.DB) error {
	// Create a default super admin (if not exists), store password using bcrypt
	var u model.User
	if err := db.Where("username = ?", defaultAdminUsername).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pwHash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("hash admin password: %w", err)
			}
			u = model.User{
				Username:      defaultAdminUsername,
				Email:         "admin@example.com",
				Password:      string(pwHash),
				Role:          model.RoleSuperAdmin,
				EmailVerified: true,
			}
			if err := db.Create(&u).Error; err != nil {
				return fmt.Errorf("create default admin: %w", err)
			}
			slog.Info("default admin user created")

			// Create default SMTP configuration (if not exists)
			if err := seedSMTP(db); err != nil {
				return err
			}
			// Create default general settings (if not exists)
			if err := seedGeneralSettings(db); err != nil {
				return err
			}
		}
	}

	// Create default AI configuration (if not exists)
	if err := seedAIConfig(db); err != nil {
		return err
	}
	// Create default theme configuration (if not exists)
	if err := seedThemeConfig(db); err != nil {
		return err
	}
	// Create a default category (if not exists)
	if err := seedCategory(db); err != nil {
		return err
	}

	return nil
}

// seedSMTP inserts the default (disabled) SMTP config when no row exists.
func seedSMTP(db *gorm.DB) error {
	var config model.SMTPConfig
	if err := db.First(&config).Error; err == gorm.ErrRecordNotFound {
		config = model.SMTPConfig{Enabled: false, Port: 587, FromName: "VexGo"}
		if err := db.Create(&config).Error; err != nil {
			return fmt.Errorf("create default SMTP config: %w", err)
		}
		slog.Info("default SMTP config created")
	}
	return nil
}

// seedGeneralSettings inserts the default general settings when no row exists.
func seedGeneralSettings(db *gorm.DB) error {
	var config model.GeneralSettings
	if err := db.First(&config).Error; err == gorm.ErrRecordNotFound {
		config = model.GeneralSettings{
			CaptchaEnabled:      false,
			RegistrationEnabled: true,
			AllowGuestViewPosts: true,
			SiteName:            "VexGo",
			ItemsPerPage:        20,
		}
		if err := db.Create(&config).Error; err != nil {
			return fmt.Errorf("create default general settings: %w", err)
		}
		slog.Info("default general settings created")
	}
	return nil
}

// seedAIConfig inserts the default (disabled) AI config when no row exists.
func seedAIConfig(db *gorm.DB) error {
	var config model.AIConfig
	if err := db.First(&config).Error; err == gorm.ErrRecordNotFound {
		config = model.AIConfig{
			Enabled:   false,
			Provider:  "openai",
			ModelName: "gpt-3.5-turbo",
		}
		if err := db.Create(&config).Error; err != nil {
			return fmt.Errorf("create default AI config: %w", err)
		}
		slog.Info("default AI config created")
	}
	return nil
}

// seedThemeConfig inserts the default theme selection when no row exists.
func seedThemeConfig(db *gorm.DB) error {
	var config model.ThemeConfig
	if err := db.First(&config).Error; err == gorm.ErrRecordNotFound {
		config = model.ThemeConfig{ActiveTheme: "default"}
		if err := db.Create(&config).Error; err != nil {
			return fmt.Errorf("create default theme config: %w", err)
		}
		slog.Info("default theme config created")
	}
	return nil
}

// seedCategory inserts the default "Default" category when it is missing.
func seedCategory(db *gorm.DB) error {
	var category model.Category
	if err := db.Where("name = ?", "Default").First(&category).Error; err == gorm.ErrRecordNotFound {
		category = model.Category{
			Name:        "Default",
			Description: "Default category for articles without a specified category",
		}
		if err := db.Create(&category).Error; err != nil {
			return fmt.Errorf("create default category: %w", err)
		}
		slog.Info("default category created")
	}
	return nil
}
