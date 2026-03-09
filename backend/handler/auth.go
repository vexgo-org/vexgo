package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"vexgo/backend/config"
	"vexgo/backend/model"
	"vexgo/backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 根据隐私设置过滤用户信息
func FilterUserByPrivacy(user *model.User, viewerID uint, viewerRole string) {
	// 如果用户设置了隐私，需要根据查看者的身份来决定是否隐藏信息
	// 目前实现：用户自己和管理员可以看到所有信息
	// 其他用户根据隐私设置隐藏信息

	// 检查查看者是否是用户自己或管理员
	isSelf := viewerID == user.ID
	isAdmin := viewerRole == model.RoleAdmin || viewerRole == model.RoleSuperAdmin

	// 如果不是自己且不是管理员，则根据隐私设置过滤
	if !isSelf && !isAdmin {
		// 首先检查个人资料可见性设置
		if user.ProfileVisibility == "private" {
			// 如果设置为仅自己可见，则隐藏所有个人信息
			user.Email = ""
			user.Birthday = ""
			user.Bio = ""
		} else {
			// 如果是公开，则根据单独的隐藏设置过滤
			if user.HideEmail {
				user.Email = ""
			}
			if user.HideBirthday {
				user.Birthday = ""
			}
			if user.HideBio {
				user.Bio = ""
			}
		}
	}
}

// 验证滑动拼图验证码的公共函数
func verifyCaptcha(captchaID, captchaToken string, captchaX int, markAsUsed bool) (*model.Captcha, error) {
	// 查询验证码
	var captcha model.Captcha
	if err := db.Where("id = ? AND token = ?", captchaID, captchaToken).First(&captcha).Error; err != nil {
		return nil, fmt.Errorf("验证码无效")
	}

	// 检查验证码是否已使用
	if captcha.Used {
		return nil, fmt.Errorf("验证码已使用")
	}

	// 检查验证码是否过期
	if time.Now().After(captcha.ExpiresAt) {
		return nil, fmt.Errorf("验证码已过期")
	}

	// 验证位置（允许一定误差范围）
	tolerance := 10 // 允许5像素的误差
	if math.Abs(float64(captchaX-captcha.X)) > float64(tolerance) {
		return nil, fmt.Errorf("验证失败，请重试")
	}

	// 如果需要标记为已使用
	if markAsUsed {
		captcha.Used = true
		if err := db.Save(&captcha).Error; err != nil {
			return nil, fmt.Errorf("验证码验证失败")
		}
	}

	return &captcha, nil
}

// 登录：根据邮箱密码签发 JWT
func Login(c *gin.Context) {
	var req struct {
		Email        string `json:"email" binding:"required"`
		Password     string `json:"password" binding:"required"`
		CaptchaID    string `json:"captcha_id"`
		CaptchaToken string `json:"captcha_token"`
		CaptchaX     int    `json:"captcha_x"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		tolerance := 10
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

	var user model.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "邮箱或密码错误"})
		return
	}

	// 使用 bcrypt 比对哈希密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "邮箱或密码错误"})
		return
	}

	// 检查是否启用了 SMTP，如果启用则检查邮箱验证状态
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err == nil && enabled && !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{
			"message":        "请先验证您的邮箱地址。请检查您的收件箱并点击验证链接，或请求重新发送验证邮件。",
			"email_verified": false,
		})
		return
	}

	// 生成 token
	claims := jwt.MapClaims{
		"user_id":          user.ID,
		"username":         user.Username,
		"role":             user.Role,
		"password_version": user.PasswordVersion,
		"exp":              time.Now().Add(24 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),                                                  // Timestamp
		"jti":              fmt.Sprintf("%d-%s", user.ID, time.Now().Format(time.RFC3339Nano)), // Unique identifier
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": ss,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// 获取当前用户信息
func GetCurrentUser(c *gin.Context) {
	if uid, ok := c.Get("userID"); ok {
		var user model.User
		if err := db.First(&user, uid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": user})
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
}

// 更新文章（需要身份验证，只有作者或管理员可修改）
func UpdatePost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.Preload("Tags").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	// 权限检查
	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err == nil {
		if user.Role != "admin" && user.Role != "super_admin" && post.AuthorID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权修改该文章"})
			return
		}
	}

	// reuse postRequest definition from post.go by redeclaring locally
	var req struct {
		Title      string      `json:"title"`
		Content    string      `json:"content"`
		Category   interface{} `json:"category"`
		Tags       []string    `json:"tags"`
		Excerpt    string      `json:"excerpt"`
		CoverImage string      `json:"coverImage"`
		Status     string      `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		post.Title = req.Title
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.Category != nil {
		switch v := req.Category.(type) {
		case string:
			post.Category = v
		case float64:
			post.Category = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			post.Category = strconv.Itoa(v)
		case int64:
			post.Category = strconv.FormatInt(v, 10)
		default:
			post.Category = fmt.Sprintf("%v", v)
		}
	}
	if req.Excerpt != "" {
		post.Excerpt = req.Excerpt
	}
	if req.CoverImage != "" {
		post.CoverImage = req.CoverImage
	}
	if req.Status != "" {
		post.Status = req.Status
	}

	if len(req.Tags) > 0 {
		tags, err := resolveTags(req.Tags)
		if err == nil {
			db.Model(&post).Association("Tags").Replace(tags)
			post.Tags = tags
		}
	}

	db.Save(&post)
	c.JSON(http.StatusOK, gin.H{"message": "文章已更新", "post": post})
}

// 删除文章（需要身份验证，只有作者或管理员可删除）
func DeletePost(c *gin.Context) {
	id := c.Param("id")
	var post model.Post
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err == nil {
		if user.Role != "admin" && user.Role != "super_admin" && post.AuthorID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该文章"})
			return
		}
	}

	db.Delete(&post)
	c.JSON(http.StatusOK, gin.H{"message": "文章已删除"})
}

// 更新用户个人信息
func UpdateProfile(c *gin.Context) {
	var req struct {
		Username *string `json:"username"`
		Avatar   *string `json:"avatar"`
		Birthday *string `json:"birthday"`
		Bio      *string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	if req.Birthday != nil {
		user.Birthday = *req.Birthday
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// 修改密码
func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码不正确"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 增加密码版本号，使旧令牌失效
	user.Password = string(hashed)
	user.PasswordVersion++
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"message": "密码已修改"})
}

// 更新用户设置（隐私设置等）
func UpdateSettings(c *gin.Context) {
	var req struct {
		ProfileVisibility *string `json:"profile_visibility"`
		HideEmail         *bool   `json:"hide_email"`
		HideBirthday      *bool   `json:"hide_birthday"`
		HideBio           *bool   `json:"hide_bio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if req.ProfileVisibility != nil {
		user.ProfileVisibility = *req.ProfileVisibility
	}
	if req.HideEmail != nil {
		user.HideEmail = *req.HideEmail
	}
	if req.HideBirthday != nil {
		user.HideBirthday = *req.HideBirthday
	}
	if req.HideBio != nil {
		user.HideBio = *req.HideBio
	}

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存设置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "设置已更新",
		"user":    user,
	})
}

// 更新邮箱
func UpdateEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("userID")
	userID := uid.(uint)
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 检查新邮箱是否与当前邮箱相同
	if req.Email == user.Email {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新邮箱不能与当前邮箱相同"})
		return
	}

	// 检查新邮箱是否已被其他用户使用
	var existingUser model.User
	if err := db.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该邮箱已被其他用户使用"})
		return
	}

	// 检查是否启用了 SMTP
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查邮件配置失败"})
		return
	}

	if enabled {
		// 如果启用 SMTP，生成邮箱变更验证令牌并发送确认邮件
		token, err := mailer.GenerateEmailChangeToken(userID, req.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成验证令牌失败"})
			return
		}

		// 构建验证链接
		protocol := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			protocol = "https"
		}
		host := c.Request.Host
		verificationLink := fmt.Sprintf("%s://%s/verify-email?token=%s", protocol, host, token)

		// 发送确认邮件
		if err := mailer.SendEmailChangeEmail(user.Email, user.Username, req.Email, verificationLink); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "发送验证邮件失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "已发送验证邮件，请查收并点击链接完成邮箱变更",
			"pending": true,
		})
	} else {
		// 如果未启用 SMTP，直接更新邮箱
		if err := db.Model(&user).Update("email", req.Email).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新邮箱失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "邮箱更新成功",
			"pending": false,
			"user": gin.H{
				"email": req.Email,
			},
		})
	}
}

// 请求密码重置
func RequestPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找用户
	var user model.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// 出于安全考虑，即使用户不存在也返回成功，避免信息泄露
		c.JSON(http.StatusOK, gin.H{"message": "如果邮箱存在，重置链接已发送"})
		return
	}

	// 检查是否启用了 SMTP
	mailer := utils.NewMailer(db)
	enabled, err := mailer.IsEmailEnabled()
	if err != nil || !enabled {
		c.JSON(http.StatusOK, gin.H{"message": "如果邮箱存在，重置链接已发送"})
		return
	}

	// 生成密码重置令牌
	token, err := mailer.GeneratePasswordResetToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成重置令牌失败"})
		return
	}

	// 构建重置链接 - 使用请求的协议和主机名
	protocol := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		protocol = "https"
	}
	host := c.Request.Host
	resetLink := fmt.Sprintf("%s://%s/reset-password?token=%s", protocol, host, token)

	// 发送邮件
	if err := mailer.SendPasswordResetEmail(user.Email, user.Username, resetLink); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送邮件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "如果邮箱存在，重置链接已发送"})
}

// 重置密码
func ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找具有该令牌的用户
	var user model.User
	if err := db.Where("verification_token = ?", req.Token).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的重置令牌"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 检查令牌是否过期
	if user.TokenExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "重置令牌已过期"})
		return
	}

	// 生成新密码的哈希值
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 更新密码并清除重置令牌
	if err := db.Model(&user).Updates(map[string]interface{}{
		"password":           string(hashed),
		"verification_token": "",
		"token_expires_at":   time.Time{},
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已重置成功"})
}
