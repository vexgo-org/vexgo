// Package auth provides the auth-domain middleware (JWT
// verification) and the huma-side helpers (UserFromContext,
// ContextMiddleware, Permission) used by every huma handler.
package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ctxKey is the request-scope key for the authenticated user.
type ctxKey struct{}

var userKey = ctxKey{}

// UserFromContext returns the authenticated user, or a zero-value
// User with ok=false when the request is anonymous.
func UserFromContext(ctx context.Context) (model.User, bool) {
	v, ok := ctx.Value(userKey).(model.User)
	return v, ok
}

// UserIDFromContext is a convenience for handlers that only need
// the user ID.
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
// api.UseMiddleware. humagin runs gin middleware first, so by the
// time this fires the gin context has the user (or doesn't, for
// anonymous requests).
func ContextMiddleware(ctx huma.Context, next func(huma.Context)) {
	if g := humagin.Unwrap(ctx); g != nil {
		if u, ok := middleware.CurrentUser(g); ok {
			next(huma.WithContext(ctx, context.WithValue(ctx.Context(), userKey, u)))
			return
		}
	}
	next(ctx)
}

// Permission is the huma-side role check. It must be chained after
// ContextMiddleware and after JWTAuth has populated the gin
// context. It mirrors the gin `middleware.Auth.Permission` for
// huma-registered routes: super admin is implicitly allowed; the
// first matching role is enough.
func Permission(requiredRoles ...string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		role := UserRoleFromContext(ctx.Context())
		if !hasRole(role, requiredRoles) {
			ctx.SetStatus(http.StatusForbidden)
			_, _ = ctx.BodyWriter().Write([]byte(`{"error":"Insufficient permissions"}`))
			return
		}
		next(ctx)
	}
}

// hasRole reports whether the given role is one of the required
// roles, or is the super admin role (which is implicitly allowed
// everywhere).
func hasRole(role string, required []string) bool {
	if role == model.RoleSuperAdmin {
		return true
	}
	for _, r := range required {
		if r == role {
			return true
		}
	}
	return false
}
