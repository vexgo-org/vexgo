package handler

import (
	"fmt"
	"log"
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

	// 检查是否允许注册
	var settings model.GeneralSettings
	if err := db.First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 默认允许注册
			settings.RegistrationEnabled = true
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check registration settings"})
			return
		}
	}

	if !settings.RegistrationEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册功能已关闭，请联系管理员"})
		return
	}

	// 检查是否启用了滑块验证
	captchaEnabled, err := IsCaptchaEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check captcha settings"})
		return
	}

	// 如果启用了滑块验证，则验证验证码
	if captchaEnabled {
		if req.CaptchaID == "" || req.CaptchaToken == "" || req.CaptchaX == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请完成滑块验证"})
			return
		}
		// 查询验证码
		var captcha model.Captcha
		if err := db.Where("id = ? AND token = ?", req.CaptchaID, req.CaptchaToken).First(&captcha).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "验证码不存在或已过期"})
			return
		}

		// 检查是否过期
		if time.Now().After(captcha.ExpiresAt) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "验证码已过期"})
			return
		}

		// 验证位置（允许一定误差范围）
		tolerance := 5
		if math.Abs(float64(req.CaptchaX-captcha.X)) > float64(tolerance) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "验证失败，请重试"})
			return
		}

		// 如果验证码还未使用，则标记为已使用
		if !captcha.Used {
			captcha.Used = true
			if err := db.Save(&captcha).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码验证失败"})
				return
			}
		}
		// 如果验证码已使用，说明已经预验证成功，直接通过
	}

	// 检查用户是否已存在
	var existingUser model.User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// 创建新用户
	newUser := model.User{
		Username:      req.Username,
		Email:         req.Email,
		Password:      string(hashedPassword),
		Role:          "contributor", // 默认角色为投稿者
		EmailVerified: false,
	}

	if err := db.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// 检查是否启用了 SMTP，如果启用则发送验证邮件
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err == nil && enabled {
		// 生成验证令牌
		token, err := mailer.GenerateVerificationToken(newUser.ID)
		if err != nil {
			log.Printf("生成验证令牌失败: %v", err)
		} else {
			// 构建验证链接 - 使用请求的协议和主机名
			protocol := "http"
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				protocol = "https"
			}
			host := c.Request.Host
			verificationLink := fmt.Sprintf("%s://%s/verify-email?token=%s", protocol, host, token)

			// 发送验证邮件
			if err := mailer.SendVerificationEmail(newUser.Email, newUser.Username, verificationLink); err != nil {
				log.Printf("发送验证邮件失败: %v", err)
			} else {
				c.JSON(http.StatusCreated, gin.H{
					"message":               "注册成功！请先验证您的邮箱地址，然后才能登录。请检查您的收件箱并点击验证链接。",
					"user":                  newUser,
					"email_verified":        false,
					"requires_verification": true,
				})
				return
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":               "注册成功",
		"user":                  newUser,
		"email_verified":        newUser.EmailVerified,
		"requires_verification": false,
	})
}
