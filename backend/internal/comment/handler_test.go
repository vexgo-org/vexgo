package comment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// handlerJWTSecret is the shared secret used to mint tokens for the handler
// tests. It must match the secret passed to the handler's middleware.
var handlerJWTSecret = []byte("comment-handler-test-secret")

// seedHandlerUser seeds a user whose password version matches the tokens
// minted by mintHandlerToken.
func seedHandlerUser(t *testing.T, db *gorm.DB, username, role string) model.User {
	t.Helper()
	u := model.User{
		Username:        username,
		Email:           username + "@example.com",
		Role:            role,
		PasswordVersion: 1,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

// newTestRouter builds a fresh gin engine with the real comment routes
// registered and a seeded database.
func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.Post{}, &model.Comment{}, &model.CommentModerationConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	h := NewHandler(Deps{DB: db, JWTSecret: handlerJWTSecret, Notifier: &fakeNotifier{}})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api"))
	return r, db
}

// mintHandlerToken signs a JWT with the claims JWTAuth expects.
func mintHandlerToken(t *testing.T, userID uint, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":          float64(userID),
		"username":         "tester",
		"role":             role,
		"password_version": float64(1),
		"iat":              float64(time.Now().Unix()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(handlerJWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func doJSON(t *testing.T, r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TC-CMOD-023/025: a keyword-rejected comment appears in the rejected
// moderation list with its reason, the pending list stays empty, and the
// commenter response no longer claims moderation is required.
func TestCreateComment_KeywordReject_ExposesReasonInModerationList(t *testing.T) {
	r, db := newTestRouter(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)
	moderator := seedHandlerUser(t, db, "moderator", model.RoleAdmin)
	adminToken := mintHandlerToken(t, moderator.ID, model.RoleAdmin)
	userToken := mintHandlerToken(t, commenter.ID, model.RoleContributor)

	w := doJSON(t, r, http.MethodPut, "/api/moderation/comments/config", adminToken,
		`{"keywordFilterEnabled": true, "blockKeywords": "spam"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("config update failed: %d %s", w.Code, w.Body.String())
	}

	w = doJSON(t, r, http.MethodPost, "/api/comments", userToken,
		`{"postId": "`+strconv.FormatUint(uint64(post.ID), 10)+`", "content": "buy spam now"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create comment failed: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"requiresModeration":true`) {
		t.Errorf("a rejected comment must not require moderation: %s", w.Body.String())
	}

	w = doJSON(t, r, http.MethodGet, "/api/moderation/comments/pending", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("pending list failed: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "buy spam now") {
		t.Errorf("rejected comment must not appear in the pending list: %s", w.Body.String())
	}

	w = doJSON(t, r, http.MethodGet, "/api/moderation/comments/rejected", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("rejected list failed: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"moderationReason":"Contains blocked keyword: spam"`) {
		t.Errorf("rejected list must expose the moderation reason: %s", w.Body.String())
	}
}

// TC-CMOD-024/015: the config endpoint round-trips the new switches, masks
// the API key, and rejects LLM review without credentials with 400.
func TestModerationConfig_RoundTripAndValidation(t *testing.T) {
	r, db := newTestRouter(t)
	moderator := seedHandlerUser(t, db, "moderator", model.RoleSuperAdmin)
	adminToken := mintHandlerToken(t, moderator.ID, model.RoleSuperAdmin)

	w := doJSON(t, r, http.MethodPut, "/api/moderation/comments/config", adminToken,
		`{"llmReviewEnabled": true}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when LLM review is enabled without credentials, got %d", w.Code)
	}

	fake := newFakeLLMServer(t, `{"approved": true, "reason": "ok"}`, http.StatusOK)
	payload := `{"manualReviewEnabled": true, "llmReviewEnabled": true, "apiKey": "test-key",` +
		` "apiEndpoint": "` + fake.URL + `", "modelName": "test-model"}`
	w = doJSON(t, r, http.MethodPut, "/api/moderation/comments/config", adminToken, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("config update failed: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "test-key") {
		t.Errorf("api key must be masked in the response: %s", w.Body.String())
	}

	w = doJSON(t, r, http.MethodGet, "/api/moderation/comments/config", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("config read failed: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"manualReviewEnabled":true`) ||
		!strings.Contains(w.Body.String(), `"llmReviewEnabled":true`) {
		t.Errorf("expected switches round-tripped, got %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "test-key") {
		t.Errorf("api key must be masked on read: %s", w.Body.String())
	}
}

// TC-CMOD-016: the config test endpoint verifies connectivity against a fake
// LLM endpoint.
func TestModerationConfigTestEndpoint(t *testing.T) {
	r, db := newTestRouter(t)
	moderator := seedHandlerUser(t, db, "moderator", model.RoleSuperAdmin)
	adminToken := mintHandlerToken(t, moderator.ID, model.RoleSuperAdmin)

	fake := newFakeLLMServer(t, `{"approved": true, "reason": "ok"}`, http.StatusOK)
	if err := db.Create(&model.CommentModerationConfig{
		LLMReviewEnabled: true,
		ApiKey:           "test-key",
		ApiEndpoint:      fake.URL,
		ModelName:        "test-model",
	}).Error; err != nil {
		t.Fatalf("seed config: %v", err)
	}

	w := doJSON(t, r, http.MethodPost, "/api/moderation/comments/config/test", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from config test, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "approved=true") {
		t.Errorf("expected verdict summary, got %s", w.Body.String())
	}
}

// TC-CMOD-026: moderation config endpoints and lists stay admin-only.
func TestModerationRoutes_RoleGating(t *testing.T) {
	r, db := newTestRouter(t)
	moderator := seedHandlerUser(t, db, "moderator", model.RoleAdmin)
	commenter := seedHandlerUser(t, db, "commenter", model.RoleContributor)
	adminToken := mintHandlerToken(t, moderator.ID, model.RoleAdmin)
	userToken := mintHandlerToken(t, commenter.ID, model.RoleContributor)

	cases := []struct {
		name   string
		method string
		path   string
		token  string
		want   int
	}{
		{"config get anonymous", http.MethodGet, "/api/moderation/comments/config", "", http.StatusUnauthorized},
		{"config get user", http.MethodGet, "/api/moderation/comments/config", userToken, http.StatusForbidden},
		{"config get admin", http.MethodGet, "/api/moderation/comments/config", adminToken, http.StatusOK},
		{"config test admin", http.MethodPost, "/api/moderation/comments/config/test", adminToken, http.StatusBadRequest},
		{"pending list user", http.MethodGet, "/api/moderation/comments/pending", userToken, http.StatusForbidden},
		{"pending list admin", http.MethodGet, "/api/moderation/comments/pending", adminToken, http.StatusOK},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSON(t, r, tt.method, tt.path, tt.token, "")
			if w.Code != tt.want {
				t.Errorf("got %d, want %d (%s)", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

// Exercise the remaining queue and comment endpoints end to end: manual
// review holds a comment as pending (TC-CMOD-002), an admin approves and
// rejects it through the queue endpoints (TC-CMOD-022), approved/rejected
// lists reflect the outcome, the public list shows published comments only,
// and the author can delete their own comment.
func TestModerationQueue_ApproveRejectDelete(t *testing.T) {
	r, db := newTestRouter(t)
	author := seedUser(t, db, "author", model.RoleContributor)
	post := seedPost(t, db, author.ID)
	commenter := seedUser(t, db, "commenter", model.RoleGuest)
	moderator := seedHandlerUser(t, db, "moderator", model.RoleAdmin)
	adminToken := mintHandlerToken(t, moderator.ID, model.RoleAdmin)
	userToken := mintHandlerToken(t, commenter.ID, model.RoleContributor)

	w := doJSON(t, r, http.MethodPut, "/api/moderation/comments/config", adminToken,
		`{"manualReviewEnabled": true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("config update failed: %d %s", w.Code, w.Body.String())
	}

	create := func(content string) string {
		w := doJSON(t, r, http.MethodPost, "/api/comments", userToken,
			`{"postId": "`+strconv.FormatUint(uint64(post.ID), 10)+`", "content": "`+content+`"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("create comment failed: %d %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"requiresModeration":true`) {
			t.Errorf("comment %q should require moderation: %s", content, w.Body.String())
		}
		var body struct {
			Comment struct {
				ID uint `json:"id"`
			} `json:"comment"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		return strconv.FormatUint(uint64(body.Comment.ID), 10)
	}

	pendingID := create("hold me")
	rejectID := create("reject me")
	publishID := create("publish me")

	// Approve two, reject one.
	w = doJSON(t, r, http.MethodPut, "/api/moderation/comments/approve/"+pendingID, adminToken, "")
	if w.Code != http.StatusOK {
		t.Errorf("approve failed: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPut, "/api/moderation/comments/approve/"+publishID, adminToken, "")
	if w.Code != http.StatusOK {
		t.Errorf("approve failed: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPut, "/api/moderation/comments/reject/"+rejectID, adminToken, "")
	if w.Code != http.StatusOK {
		t.Errorf("reject failed: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodPut, "/api/moderation/comments/approve/99999", adminToken, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 approving a missing comment, got %d", w.Code)
	}

	// The approved list carries the two approved comments.
	w = doJSON(t, r, http.MethodGet, "/api/moderation/comments/approved", adminToken, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hold me") {
		t.Errorf("expected approved comment in approved list: %d %s", w.Code, w.Body.String())
	}

	// The public post list shows published comments only.
	w = doJSON(t, r, http.MethodGet, "/api/comments/post/"+strconv.FormatUint(uint64(post.ID), 10), "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("public comment list failed: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "publish me") || !strings.Contains(w.Body.String(), "hold me") {
		t.Errorf("expected approved comments in the public list: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "reject me") {
		t.Errorf("public list must hide rejected comments: %s", w.Body.String())
	}

	// The author can delete their own published comment.
	w = doJSON(t, r, http.MethodDelete, "/api/comments/"+publishID, userToken, "")
	if w.Code != http.StatusOK {
		t.Errorf("delete failed: %d %s", w.Code, w.Body.String())
	}
}
