package auth

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ctxKey is the request-scope key for the authenticated user.
type ctxKey struct{}

var userKey = ctxKey{}

// UserFromContext returns the authenticated user, or a zero-value
// User with ok=false when the request is anonymous. Huma handlers
// reach the current user through this helper rather than
// `middleware.CurrentUser(*gin.Context)`.
func UserFromContext(ctx context.Context) (model.User, bool) {
	v, ok := ctx.Value(userKey).(model.User)
	return v, ok
}

// UserIDFromContext is a convenience for handlers that only need
// the user ID. Returns 0 for anonymous requests.
func UserIDFromContext(ctx context.Context) uint {
	u, ok := UserFromContext(ctx)
	if !ok {
		return 0
	}
	return u.ID
}

// UserRoleFromContext returns the role string for the authenticated
// user, or "" for anonymous requests.
func UserRoleFromContext(ctx context.Context) string {
	u, ok := UserFromContext(ctx)
	if !ok {
		return ""
	}
	return u.Role
}

// ContextMiddleware copies the user from the underlying gin context
// (set by middleware.JWTAuth and friends) into the request's
// context.Context so huma handlers can read it via
// UserFromContext. It must be registered on the huma.API via
// api.UseMiddleware *after* the gin middleware that sets the user.
//
// In practice, since humagin runs gin middleware first, this huma
// middleware can be attached to the whole API and it will only find
// a user when JWTAuth (or similar) has already populated the gin
// context. Endpoints that do not require auth still work: the user
// is simply absent from the request context.
func ContextMiddleware(ctx huma.Context, next func(huma.Context)) {
	if g := humagin.Unwrap(ctx); g != nil {
		if u, ok := middleware.CurrentUser(g); ok {
			newCtx := huma.WithContext(ctx, context.WithValue(ctx.Context(), userKey, u))
			next(newCtx)
			return
		}
	}
	next(ctx)
}
