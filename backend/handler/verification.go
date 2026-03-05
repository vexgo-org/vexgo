package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"time"

	"blog-system/backend/model"
	"blog-system/backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VerifyEmail 验证邮箱
func VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证令牌不能为空"})
		return
	}

	mailer := utils.NewMailer(db)
	if err := mailer.VerifyEmail(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 重定向到成功页面或返回成功消息
	c.JSON(http.StatusOK, gin.H{
		"message": "邮箱验证成功！您现在可以登录了。",
	})
}

// GetVerificationStatus 获取当前用户的邮箱验证状态
func GetVerificationStatus(c *gin.Context) {
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	if userMap, ok := userContext.(map[string]interface{}); ok {
		if userID, ok := userMap["id"].(float64); ok {
			var user model.User
			if err := db.First(&user, uint(userID)).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"email_verified": user.EmailVerified,
				"email":          user.Email,
			})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
}

// GenerateCaptcha 生成滑动拼图验证码
func GenerateCaptcha(c *gin.Context) {
	// 生成验证码ID和令牌
	captchaID := uuid.New().String()
	token := uuid.New().String()

	// 设置拼图大小
	puzzleWidth := 40
	puzzleHeight := 40
	bgWidth := 320
	bgHeight := 160

	// 随机生成拼图位置（确保拼图完全在图片内）
	maxX := bgWidth - puzzleWidth - 20 // 留出20像素的边距
	minX := 20
	x := minX + randInt(maxX-minX)
	y := 20 + randInt(120) // Y轴位置在20-140之间，适应更小的拼图

	// 创建背景图片（蓝色渐变）
	bgImage := createGradientBackground(bgWidth, bgHeight)

	// 创建拼图形状
	puzzleShape := createPuzzleShape(puzzleWidth, puzzleHeight)

	// 从背景图片中提取拼图部分
	puzzleImage := extractPuzzleImage(bgImage, x, y, puzzleShape, puzzleWidth, puzzleHeight)

	// 在背景图片上绘制拼图轮廓
	bgImageWithHole := drawPuzzleHole(bgImage, x, y, puzzleShape, puzzleWidth, puzzleHeight)

	// 将图片转换为Base64
	bgImageBase64, err := imageToBase64(bgImageWithHole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "编码背景图片失败"})
		return
	}

	puzzleImageBase64, err := imageToBase64(puzzleImage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "编码拼图图片失败"})
		return
	}

	// 保存验证码信息到数据库
	captcha := model.Captcha{
		ID:        captchaID,
		Token:     token,
		X:         x,
		Y:         y,
		Width:     puzzleWidth,
		Height:    puzzleHeight,
		BgImage:   bgImageBase64,
		PuzzleImg: puzzleImageBase64,
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5分钟过期
		Used:      false,
	}

	if err := db.Create(&captcha).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存验证码失败"})
		return
	}

	// 返回验证码信息（不包含正确答案）
	c.JSON(http.StatusOK, gin.H{
		"id":         captchaID,
		"token":      token,
		"bg_image":   bgImageBase64,
		"puzzle_img": puzzleImageBase64,
		"y":          y, // 返回拼图的y坐标
		"expires_at": captcha.ExpiresAt,
	})
}

// VerifyCaptcha 验证滑动拼图
func VerifyCaptcha(c *gin.Context) {
	var req struct {
		ID    string `json:"id" binding:"required"`
		Token string `json:"token" binding:"required"`
		X     int    `json:"x" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询验证码
	var captcha model.Captcha
	if err := db.Where("id = ? AND token = ?", req.ID, req.Token).First(&captcha).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "验证码不存在或已过期"})
		return
	}

	// 检查是否已使用
	if captcha.Used {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码已使用"})
		return
	}

	// 检查是否过期
	if time.Now().After(captcha.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码已过期"})
		return
	}

	// 验证位置（允许一定误差范围）
	tolerance := 5 // 允许5像素的误差
	if math.Abs(float64(req.X-captcha.X)) > float64(tolerance) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证失败，请重试"})
		return
	}

	// 不在这里标记为已使用，而是在登录时标记
	// 返回验证成功
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// 辅助函数

// createGradientBackground 创建一个简单的渐变背景
func createGradientBackground(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 创建一个简单的蓝色渐变
			r := uint8(100 + x*155/width)
			g := uint8(150 + y*105/height)
			b := uint8(200)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// 添加一些简单的装饰
	for i := 0; i < 5; i++ {
		x := i * width / 5
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 100})
		}
	}

	return img
}

// createPuzzleShape 创建拼图形状
func createPuzzleShape(width, height int) [][]bool {
	// 创建一个拼图形状
	shape := make([][]bool, height)
	for y := 0; y < height; y++ {
		shape[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			// 创建一个基本的矩形形状
			if y >= height/4 && y <= 3*height/4 && x >= width/4 && x <= 3*width/4 {
				shape[y][x] = true
			} else if y >= height/3 && y <= 2*height/3 {
				shape[y][x] = true
			} else {
				shape[y][x] = false
			}
		}
	}

	// 添加拼图的凸起和凹陷
	midY := height / 2
	bumpSize := height / 4 // 凸起尺寸

	// 左侧凸起（圆形）
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			// 创建圆形凸起
			dist := math.Sqrt(float64((px-width/8)*(px-width/8) + (py-midY)*(py-midY)))
			if dist < float64(bumpSize/2) {
				shape[py][px] = true
			}
		}
	}

	// 右侧凹陷（圆形）
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			dist := math.Sqrt(float64((px-6*width/8)*(px-6*width/8) + (py-midY)*(py-midY)))
			if dist < float64(bumpSize/3) {
				shape[py][px] = false
			}
		}
	}

	// 上方凸起（半圆形）
	for py := 0; py < height/2; py++ {
		for px := width / 3; px < 2*width/3; px++ {
			dist := math.Sqrt(float64((px-width/2)*(px-width/2) + (py-height/6)*(py-height/6)))
			if dist < float64(bumpSize/3) {
				shape[py][px] = true
			}
		}
	}

	// 下方凹陷（半圆形）
	for py := height / 2; py < height; py++ {
		for px := width / 3; px < 2*width/3; px++ {
			dist := math.Sqrt(float64((px-width/2)*(px-width/2) + (py-5*height/6)*(py-5*height/6)))
			if dist < float64(bumpSize/3) {
				shape[py][px] = false
			}
		}
	}

	// 添加一些随机小凹凸增加复杂度
	for i := 0; i < 3; i++ {
		rx := width/4 + randInt(width/2)
		ry := height/4 + randInt(height/2)
		r := bumpSize / 6
		for py := 0; py < height; py++ {
			for px := 0; px < width; px++ {
				dist := math.Sqrt(float64((px-rx)*(px-rx) + (py-ry)*(py-ry)))
				if dist < float64(r) {
					if i%2 == 0 {
						shape[py][px] = true
					} else {
						shape[py][px] = false
					}
				}
			}
		}
	}

	return shape
}

// extractPuzzleImage 从背景图片中提取拼图部分
func extractPuzzleImage(bgImage *image.RGBA, x, y int, shape [][]bool, width, height int) *image.RGBA {
	puzzleImg := image.NewRGBA(image.Rect(0, 0, width, height))

	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			if py < len(shape) && px < len(shape[py]) && shape[py][px] {
				bgX := x + px
				bgY := y + py

				// 检查边界
				if bgX >= 0 && bgX < bgImage.Bounds().Dx() && bgY >= 0 && bgY < bgImage.Bounds().Dy() {
					puzzleImg.Set(px, py, bgImage.At(bgX, bgY))
				}
			} else {
				// 透明背景
				puzzleImg.Set(px, py, color.Transparent)
			}
		}
	}

	return puzzleImg
}

// drawPuzzleHole 在背景图片上绘制拼图轮廓
func drawPuzzleHole(bgImage *image.RGBA, x, y int, shape [][]bool, width, height int) *image.RGBA {
	// 创建背景图片的副本
	bgCopy := image.NewRGBA(bgImage.Bounds())
	draw.Draw(bgCopy, bgCopy.Bounds(), bgImage, image.Point{}, draw.Src)

	// 在拼图位置绘制半透明阴影
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			if py < len(shape) && px < len(shape[py]) && shape[py][px] {
				bgX := x + px
				bgY := y + py

				// 检查边界
				if bgX >= 0 && bgX < bgCopy.Bounds().Dx() && bgY >= 0 && bgY < bgCopy.Bounds().Dy() {
					// 获取原像素并调暗
					original := bgCopy.At(bgX, bgY)
					r, g, b, a := original.RGBA()
					// 调暗20%
					r = uint32(float64(r) * 0.8)
					g = uint32(float64(g) * 0.8)
					b = uint32(float64(b) * 0.8)
					bgCopy.Set(bgX, bgY, color.NRGBA{uint8(r / 256), uint8(g / 256), uint8(b / 256), uint8(a / 256)})
				}
			}
		}
	}

	return bgCopy
}

// imageToBase64 将图片转换为Base64字符串
func imageToBase64(img *image.RGBA) (string, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// randInt 生成随机整数
func randInt(max int) int {
	if max <= 0 {
		return 0
	}

	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		return 0
	}

	return int(b[0]) % max
}

// ResendVerificationEmail 重新发送验证邮件
func ResendVerificationEmail(c *gin.Context) {
	userContext, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	if userMap, ok := userContext.(map[string]interface{}); ok {
		if userID, ok := userMap["id"].(float64); ok {
			var user model.User
			if err := db.First(&user, uint(userID)).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
				return
			}

			if user.EmailVerified {
				c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱已验证"})
				return
			}

			mailer := utils.NewMailer(db)
			enabled, err := mailer.IsEmailEnabled()
			if err != nil || !enabled {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "邮件服务未启用"})
				return
			}

			// 生成新的验证令牌
			token, err := mailer.GenerateVerificationToken(user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "生成验证令牌失败"})
				return
			}

			// 构建验证链接
			verificationLink := c.Request.Host + "/verify-email?token=" + token
			if err := mailer.SendVerificationEmail(user.Email, user.Username, verificationLink); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "发送验证邮件失败"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "验证邮件已重新发送，请检查您的邮箱",
			})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
}
