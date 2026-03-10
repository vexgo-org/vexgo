package handler

import (
	"net/http"
	"strconv"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// Toggle like (requires login)
func ToggleLike(c *gin.Context) {
	postIDStr := c.Param("postId")
	id64, _ := strconv.ParseUint(postIDStr, 10, 64)
	postID := uint(id64)

	uid, _ := c.Get("userID")
	userID := uid.(uint)

	var like model.Like
	if err := db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err == nil {
		// Already liked -> unlike
		db.Delete(&like)
		var count int64
		db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)
		c.JSON(http.StatusOK, gin.H{"message": "Like removed", "postId": postID, "isLiked": false, "likesCount": count})
		return
	}

	like = model.Like{PostID: postID, UserID: userID}
	db.Create(&like)
	var count int64
	db.Model(&model.Like{}).Where("post_id = ?", postID).Count(&count)
	c.JSON(http.StatusOK, gin.H{"message": "Liked successfully", "postId": postID, "isLiked": true, "likesCount": count})
}

// Get like status (public, optional login)
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
