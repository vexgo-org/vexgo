package router

import (
	"reflect"
	"sort"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/auth"
	"github.com/vexgo-org/vexgo/backend/internal/captcha"
	"github.com/vexgo-org/vexgo/backend/internal/comment"
	"github.com/vexgo-org/vexgo/backend/internal/home"
	"github.com/vexgo-org/vexgo/backend/internal/notification"
	"github.com/vexgo-org/vexgo/backend/internal/post"
	"github.com/vexgo-org/vexgo/backend/internal/settings"
	"github.com/vexgo-org/vexgo/backend/internal/sso"
	"github.com/vexgo-org/vexgo/backend/internal/upload"
	"github.com/vexgo-org/vexgo/backend/internal/user"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

// TestRegisterAPIRoutes_RouteSurface locks the complete API route surface:
// every method+path the router registers. Adding or removing a route (or
// renaming a path) must update this list deliberately.
func TestRegisterAPIRoutes_RouteSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	secret := []byte("test-secret")

	r := gin.New()
	RegisterAPIRoutes(r, Deps{
		DB:           db,
		JWTSecret:    secret,
		Notification: notification.Deps{DB: db, JWTSecret: secret},
		Comment:      comment.Deps{DB: db, JWTSecret: secret},
		Post:         post.Deps{DB: db, JWTSecret: secret},
		Upload:       upload.Deps{DB: db, JWTSecret: secret},
		User:         user.Deps{DB: db, JWTSecret: secret},
		Captcha: captcha.Deps{
			DB:        db,
			JWTSecret: secret,
		},
		Auth:     auth.Deps{DB: db, JWTSecret: secret},
		SSO:      sso.Deps{DB: db, JWTSecret: secret},
		Home:     home.Deps{DB: db, JWTSecret: secret},
		Settings: settings.Deps{DB: db, JWTSecret: secret},
	})

	got := make([]string, 0, len(r.Routes()))
	for _, rt := range r.Routes() {
		got = append(got, rt.Method+" "+rt.Path)
	}
	sort.Strings(got)

	want := []string{
		// auth
		"POST /api/auth/register",
		"POST /api/auth/login",
		"GET /api/auth/me",
		"GET /api/auth/user",
		"PUT /api/auth/profile",
		"PUT /api/auth/password",
		"PUT /api/auth/email",
		"PUT /api/auth/settings",
		"POST /api/auth/request-password-reset",
		"POST /api/auth/reset-password",
		// comment
		"GET /api/comments/post/:id",
		"POST /api/comments",
		"DELETE /api/comments/:id",
		"GET /api/moderation/comments/pending",
		"GET /api/moderation/comments/approved",
		"GET /api/moderation/comments/rejected",
		"PUT /api/moderation/comments/approve/:id",
		"PUT /api/moderation/comments/reject/:id",
		"GET /api/moderation/comments/config",
		"PUT /api/moderation/comments/config",
		// home
		"GET /api/stats",
		// notification
		"GET /api/notifications",
		"GET /api/notifications/unread-count",
		"PUT /api/notifications/:id/read",
		"PUT /api/notifications/read-all",
		"DELETE /api/notifications/:id",
		// post
		"GET /api/posts",
		"GET /api/posts/by-id/:id",
		"GET /api/posts/:slug",
		"GET /api/categories",
		"GET /api/tags",
		"GET /api/stats/popular-posts",
		"GET /api/stats/latest-posts",
		"GET /api/likes/:postId",
		"GET /api/posts/user/:id",
		"POST /api/posts",
		"GET /api/posts/user/my-posts",
		"GET /api/posts/drafts",
		"PUT /api/posts/:id",
		"DELETE /api/posts/:id",
		"POST /api/categories",
		"POST /api/tags",
		"POST /api/likes/:postId",
		"GET /api/moderation/pending",
		"GET /api/moderation/approved",
		"GET /api/moderation/rejected",
		"PUT /api/moderation/approve/:id",
		"PUT /api/moderation/reject/:id",
		"PUT /api/moderation/resubmit/:id",
		// settings
		"GET /api/themes",
		"GET /api/theme/:id/preview",
		"GET /api/config/smtp",
		"PUT /api/config/smtp",
		"POST /api/config/smtp/test",
		"GET /api/config/ai",
		"PUT /api/config/ai",
		"POST /api/config/ai/test",
		"GET /api/config/ai/models",
		"GET /api/config/general",
		"PUT /api/config/general",
		"GET /api/config/theme",
		"PUT /api/config/theme",
		"POST /api/themes/upload",
		// sso
		"GET /api/sso/providers",
		"GET /api/sso/:provider/login",
		"GET /api/sso/:provider/callback",
		// upload
		"POST /api/upload/file",
		"POST /api/upload/files",
		"GET /api/upload/my-files",
		"DELETE /api/upload/:id",
		// user
		"GET /api/users",
		"PUT /api/users/:id/role",
		"DELETE /api/users/:id",
		"POST /api/users/apply-creator",
		"GET /api/users/creator-applications",
		"PUT /api/users/creator-applications/:id/review",
		// verification
		"GET /api/verify-email",
		"GET /api/captcha",
		"POST /api/captcha/verify",
		"GET /api/auth/verification-status",
	}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("route surface mismatch\n got: %v\nwant: %v", got, want)
	}
}
