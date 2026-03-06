package handler

import (
	"blog-system/backend/model"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func InitDB() {
	var err error
	db, err = gorm.Open(sqlite.Open("blog.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
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
					TokenExpiresAt:    time.Time{},
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
						FromName:  "Blog System",
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
						SiteName:            "Blog System",
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
