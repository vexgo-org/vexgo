package handler

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"math"
	"net/http"
	"time"

	"vexgo/backend/model"
	"vexgo/backend/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Register(c *gin.Context) {
	var req struct {
		Email        string `json:"email" binding:"required,email"`
		Password     string `json:"password" binding:"required"`
		Username     string `json:"username" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if registration is allowed
	var settings model.GeneralSettings
	if err := db.First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Allow registration by default
			settings.RegistrationEnabled = true
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check registration settings"})
			return
		}
	}

	if !settings.RegistrationEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled, please contact administrator"})
		return
	}

	// Check if captcha verification is enabled
	captchaEnabled, err := IsCaptchaEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check captcha settings"})
		return
	}

	// If captcha verification is enabled, verify captcha
	if captchaEnabled {
		if req.CaptchaID == "" || req.CaptchaToken == "" || req.CaptchaX == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Please complete captcha verification"})
			return
		}
		// Query captcha
		var captcha model.Captcha
		if err := db.Where("id = ? AND token = ?", req.CaptchaID, req.CaptchaToken).First(&captcha).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Captcha does not exist or has expired"})
			return
		}

		// Check if expired
		if time.Now().After(captcha.ExpiresAt) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha has expired"})
			return
		}

		// Verify position (allow certain tolerance)
		tolerance := 5
		if math.Abs(float64(req.CaptchaX-captcha.X)) > float64(tolerance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Verification failed, please try again"})
			return
		}

		// If captcha has not been used yet, mark it as used
		if !captcha.Used {
			captcha.Used = true
			if err := db.Save(&captcha).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Captcha verification failed"})
				return
			}
		}
		// If captcha already used, pre-verification successful, pass directly
	}

	// Check if user already exists
	var existingUser model.User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Encrypt password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create new user
	newUser := model.User{
		Username:      req.Username,
		Email:         req.Email,
		Password:      string(hashedPassword),
		Role:          model.RoleGuest, // Default role is guest
		EmailVerified: false,
	}

	if err := db.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Check if SMTP is enabled, if so send verification email
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err == nil && enabled {
		// Generate verification token
		token, err := mailer.GenerateVerificationToken(newUser.ID)
		if err != nil {
			logrus.Printf("Failed to generate verification token: %v", err)
		} else {
			// Build verification link - use request protocol and hostname
			protocol := "http"
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				protocol = "https"
			}
			host := c.Request.Host
			verificationLink := fmt.Sprintf("%s://%s/verify-email?token=%s", protocol, host, token)

			// Send verification email
			if err := mailer.SendVerificationEmail(newUser.Email, newUser.Username, verificationLink); err != nil {
				logrus.Printf("Failed to send verification email: %v", err)
			} else {
				c.JSON(http.StatusCreated, gin.H{
					"message":               "Registration successful! Please verify your email address before logging in. Check your inbox and click the verification link.",
					"user":                  newUser,
					"email_verified":        false,
					"requires_verification": true,
				})
				return
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":               "Registration successful",
		"user":                  newUser,
		"email_verified":        newUser.EmailVerified,
		"requires_verification": false,
	})
}
