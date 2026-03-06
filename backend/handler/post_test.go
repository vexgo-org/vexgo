package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"vexgo/backend/model"

	"github.com/gin-gonic/gin"
)

// helper to reset database file between tests
func resetDB(t *testing.T) {
	// remove existing file
	os.Remove("blog.db")
	InitDB()
}

// add a simple user and return its ID
func createTestUser(t *testing.T) uint {
	user := model.User{Username: "tester", Email: "test@example.com", Password: "pwd", Role: "user"}
	err := db.Create(&user).Error
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user.ID
}

// set user id into gin context
func setUserContext(c *gin.Context, uid uint) {
	c.Set("userID", uid)
}

func TestCreatePost_SavesDraftAndPublished(t *testing.T) {
	resetDB(t)
	uid := createTestUser(t)

	// build request with tags array
	// numeric category simulating frontend behavior
	reqBody := map[string]interface{}{
		"title":      "Hello",
		"content":    "world",
		"category":   1,
		"tags":       []string{"go", "gin"},
		"excerpt":    "ex",
		"coverImage": "/img.png",
		"status":     "published",
	}
	data, _ := json.Marshal(reqBody)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request, _ = http.NewRequest("POST", "/api/auth/posts", bytes.NewBuffer(data))
	c.Request.Header.Set("Content-Type", "application/json")
	setUserContext(c, uid)

	CreatePost(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body %s", rec.Code, rec.Body.String())
	}

	// verify post in db
	var post model.Post
	err := db.Preload("Tags").First(&post).Error
	if err != nil {
		t.Fatalf("post not saved: %v", err)
	}
	if post.Status != "published" {
		t.Errorf("status expected published, got %s", post.Status)
	}
	if len(post.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(post.Tags))
	}
}

func TestUpdatePost_ModifiesFields(t *testing.T) {
	resetDB(t)
	uid := createTestUser(t)

	// create initial post
	post := model.Post{Title: "A", Content: "B", Category: "1", AuthorID: uid, Status: "draft"}
	db.Create(&post)

	// update request change title, status, tags
	reqBody := map[string]interface{}{
		"title":  "New",
		"status": "published",
		"tags":   []string{"foo"},
	}
	data, _ := json.Marshal(reqBody)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request, _ = http.NewRequest("PUT", "/api/auth/posts/1", bytes.NewBuffer(data))
	c.Request.Header.Set("Content-Type", "application/json")
	setUserContext(c, uid)

	UpdatePost(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var updated model.Post
	db.Preload("Tags").First(&updated, post.ID)
	if updated.Title != "New" {
		t.Errorf("title not updated")
	}
	if updated.Status != "published" {
		t.Errorf("status not updated")
	}
	if len(updated.Tags) != 1 || updated.Tags[0].Name != "foo" {
		t.Errorf("tags not updated: %+v", updated.Tags)
	}
}

// ensure tags created correctly
func TestResolveTags_CreatesIfMissing(t *testing.T) {
	resetDB(t)
	_, err := resolveTags([]string{"x", "y", "x"})
	if err != nil {
		t.Fatalf("resolveTags error: %v", err)
	}
	var tags []model.Tag
	db.Find(&tags)
	if len(tags) != 2 {
		t.Errorf("expected 2 unique tags, got %d", len(tags))
	}
}
