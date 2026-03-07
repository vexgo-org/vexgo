package handler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"vexgo/backend/model"

	dmsql "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

// InitDB initializes the database connection based on environment variables
func InitDB(dataDir string) {
	dbType := os.Getenv("DB_TYPE")
	var err error

	if dbType == "mysql" {
		// MySQL connection
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		dbname := os.Getenv("DB_NAME")

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, dbname)

		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			// Check if the error is "Unknown database" (error code 1049)
			if mysqlErr, ok := err.(*dmsql.MySQLError); ok && mysqlErr.Number == 1049 {
				log.Printf("Database '%s' not found, attempting to create it.", dbname)

				// DSN without database name to connect to the server
				serverDsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, port)
				serverDb, serverErr := gorm.Open(mysql.Open(serverDsn), &gorm.Config{})
				if serverErr != nil {
					log.Fatalf("failed to connect to MySQL server to create database: %v", serverErr)
				}

				// Create the database
				createDbSQL := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbname)
				if execErr := serverDb.Exec(createDbSQL).Error; execErr != nil {
					log.Fatalf("failed to create database '%s': %v", dbname, execErr)
				}
				log.Printf("Database '%s' created successfully.", dbname)

				// Re-attempt connection to the newly created database
				db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
				if err != nil {
					log.Fatalf("failed to connect to newly created MySQL database: %v", err)
				}
			} else {
				log.Fatalf("failed to connect to MySQL database: %v", err)
			}
		}
		log.Println("Successfully connected to MySQL database")
	} else {
		// SQLite connection (default)
		if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
			log.Fatalf("failed to create data directory: %v", err)
		}
		dbPath := filepath.Join(dataDir, "blog.db")
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			log.Fatalf("failed to connect to SQLite database: %v", err)
		}
		log.Println("Successfully connected to SQLite database")
	}

	// 自动迁移模型
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
	); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	// 创建一个默认超级管理员（如果不存在），使用 bcrypt 存储密码
	var u model.User
	if err := db.Where("username = ?", "admin").First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			pwHash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("failed to hash default admin password: %v", err)
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
					log.Printf("failed to create default admin: %v", err)
				}
			}

			// 创建默认 SMTP 配置（如果不存在）
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
						log.Printf("failed to create default smtp config: %v", err)
					} else {
						log.Println("default smtp config created")
					}
				}
			}

			// 创建默认通用设置（如果不存在）
			var generalSettings model.GeneralSettings
			if err := db.First(&generalSettings).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					generalSettings = model.GeneralSettings{
						CaptchaEnabled:      false, // 默认不启用滑块验证
						RegistrationEnabled: true,  // 默认允许注册
						SiteName:            "VexGo",
						SiteDescription:     "",
						ItemsPerPage:        20,
					}
					if err := db.Create(&generalSettings).Error; err != nil {
						log.Printf("failed to create default general settings: %v", err)
					} else {
						log.Println("default general settings created")
					}
				}
			}
		}
	}

	// 创建默认 AI 配置（如果不存在）
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
				log.Printf("failed to create default ai config: %v", err)
			} else {
				log.Println("default ai config created")
			}
		}
	}

	// 创建一个默认分类（如果不存在）
	var defaultCategory model.Category
	if err := db.Where("name = ?", "Default").First(&defaultCategory).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			defaultCategory = model.Category{
				Name:        "Default",
				Description: "默认分类，用于未指定分类的文章",
			}
			if err := db.Create(&defaultCategory).Error; err != nil {
				log.Printf("failed to create default category: %v", err)
			} else {
				log.Println("default category created: Default")
			}
		}
	}
}

// DB returns the database instance
func DB() *gorm.DB {
	return db
}
