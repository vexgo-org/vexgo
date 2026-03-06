package handler

import (
	"net/http"
	"strconv"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// 切换点赞（需登录）
func ToggleLike(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	var like model.Like
	if err := db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err == nil {
		// 已点赞 -> 取消
		db.Delete(&like)
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)
		c.JSON(http.StatusOK, gin.H{"message": "点赞已取消", "postId": postID, "isLiked": false, "likesCount": count})
		return
	}

	like = model.Like{PostID: postID, UserID: userID}
	db.Create(&like)
	var count int64
	db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)
	c.JSON(http.StatusOK, gin.H{"message": "点赞成功", "postId": postID, "isLiked": true, "likesCount": count})
}

// 获取点赞状态（公开，可选登录）
func GetLikeStatus(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	var isLiked bool
	if uid, exists := c.Get("userID"); exists {
		userID := uid.(uint)
		var like model.Like
		if db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error == nil {
			isLiked = true
		}
	}

	var count int64
	db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)

	c.JSON(http.StatusOK, gin.H{
		"postId":     postID,
		"likesCount": count,
		"isLiked":    isLiked,
	})
}
